"""Build Mininet host–host TCLink networks from a topology dict.

Addressing model (matches the Go netinfra plan):
  - Identity IP on lo: 10.0.0.(i+1) for nodes[i] (from identity_cidr base)
  - One /30 per physical link under 10.100.(link_index+1).0/30
  - Underlay: full mesh of TCLinks; failed links are brought down at inject
  - Direct mode: ip_forward=0; routes only to on-link neighbors' identity IPs
  - Forwarding mode: ip_forward=1 + shortest-path overlay detours around down
    links (simplified NIFTY-style reroute through intermediate nodes)
"""

from __future__ import annotations

import ipaddress
import logging
from typing import Any, Dict, List, Optional, Tuple

log = logging.getLogger("mininetd.topology")


def _link_key(a: str, b: str) -> str:
    return "--".join(sorted((a, b)))


def _parse_identity_base(cidr: str) -> ipaddress.IPv4Address:
    net = ipaddress.ip_network(cidr, strict=False)
    return net.network_address


def identity_map(topo: Dict[str, Any]) -> Dict[str, str]:
    base = int(_parse_identity_base(topo["identity_cidr"]))
    out = {}
    for i, name in enumerate(topo["nodes"]):
        out[name] = str(ipaddress.IPv4Address(base + i + 1))
    return out


def link_subnet(index: int) -> ipaddress.IPv4Network:
    # 10.100.(index+1).0/30 — enough for dozens of mesh edges
    return ipaddress.ip_network(f"10.100.{index + 1}.0/30")


class TopologyRuntime:
    """Holds a live Mininet network and addressing metadata."""

    def __init__(self) -> None:
        self.net = None
        self.topo: Dict[str, Any] = {}
        self.mode: str = "direct"
        self.delay_ms: float = 0.0
        self.idents: Dict[str, str] = {}
        # link_key -> {index, a, b, ip_a, ip_b, intf_a, intf_b}
        self.links: Dict[str, Dict[str, Any]] = {}
        self.down: Dict[str, bool] = {}

    def start(self, topo: Dict[str, Any], mode: str, delay_ms: float) -> None:
        if self.net is not None:
            raise RuntimeError("network already running; call Stop first")

        # Import here so the module can be inspected without Mininet installed.
        from mininet.link import TCLink
        from mininet.log import setLogLevel
        from mininet.net import Mininet

        setLogLevel("warning")
        self.topo = topo
        self.mode = mode
        self.delay_ms = float(delay_ms)
        self.idents = identity_map(topo)
        self.links = {}
        self.down = {}

        delay_str = _delay_str(self.delay_ms)
        net = Mininet(topo=None, build=False, controller=None, link=TCLink)
        hosts = {}
        for name in topo["nodes"]:
            # Addresses are applied after start (identity on lo + per-link /30s).
            hosts[name] = net.addHost(name)

        for i, link in enumerate(topo.get("links", [])):
            a, b = link["endpoints"][0], link["endpoints"][1]
            key = _link_key(a, b)
            subnet = link_subnet(i)
            hosts_list = list(subnet.hosts())
            ip_a, ip_b = str(hosts_list[0]), str(hosts_list[1])

            lnk = net.addLink(
                hosts[a],
                hosts[b],
                cls=TCLink,
                delay=delay_str,
                # Modest bw avoids sch_htb "quantum … is big" warnings from bw=1000.
                # Experiments vary delay, not capacity; 100 Mbit/s >> Raft RPC load.
                bw=100,
            )
            # intf1 is attached to first host arg, intf2 to second.
            intf_a, intf_b = lnk.intf1, lnk.intf2
            self.links[key] = {
                "index": i,
                "a": a,
                "b": b,
                "ip_a": ip_a,
                "ip_b": ip_b,
                "intf_a": intf_a,
                "intf_b": intf_b,
                "link": lnk,
            }

        net.build()
        net.start()
        self.net = net

        # Drop Mininet's default 10.0.0.0/8 assignments so they cannot collide
        # with identity addresses on lo.
        for name in topo["nodes"]:
            h = net.get(name)
            for intf in h.intfList():
                if intf.name == "lo":
                    continue
                h.cmd(f"ip addr flush dev {intf.name}")
                h.cmd(f"ip link set {intf.name} up")

        # Identity addresses on lo.
        for name, ip in self.idents.items():
            h = net.get(name)
            h.cmd("ip addr flush dev lo")
            h.cmd("ip addr add 127.0.0.1/8 dev lo")
            h.cmd(f"ip addr add {ip}/32 dev lo")
            h.cmd("ip link set lo up")

        # Per-link /30s on veth endpoints.
        for key, meta in self.links.items():
            meta["intf_a"].setIP(meta["ip_a"], prefixLen=30)
            meta["intf_b"].setIP(meta["ip_b"], prefixLen=30)

        # Start with ALL links up (planned failures injected later by Go).
        for key in self.links:
            self.down[key] = False

        self.apply_mode(mode)
        log.info(
            "started topology %s mode=%s delay_ms=%s nodes=%s links=%d",
            topo.get("name"),
            mode,
            delay_ms,
            list(topo.get("nodes", [])),
            len(self.links),
        )

    def stop(self) -> None:
        if self.net is not None:
            try:
                self.net.stop()
            finally:
                self.net = None
        self._cleanup_mn()
        self.links = {}
        self.down = {}
        self.topo = {}

    @staticmethod
    def _cleanup_mn() -> None:
        """Best-effort leftover namespace cleanup between runs."""
        import subprocess

        try:
            subprocess.run(
                ["mn", "-c"],
                check=False,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                timeout=60,
            )
        except (FileNotFoundError, subprocess.TimeoutExpired) as exc:
            log.warning("mn -c cleanup skipped: %s", exc)

    def set_link(self, a: str, b: str, up: bool) -> None:
        self._require_net()
        key = _link_key(a, b)
        if key not in self.links:
            raise KeyError(f"unknown link {a}--{b}")
        status = "up" if up else "down"
        self.net.configLinkStatus(a, b, status)
        self.down[key] = not up
        # Refresh routes so multi-hop entries do not point through a dead next hop.
        self.apply_mode(self.mode)

    def set_delay(self, delay_ms: float) -> None:
        self._require_net()
        self.delay_ms = float(delay_ms)
        delay_str = _delay_str(self.delay_ms)
        for meta in self.links.values():
            # Symmetric delay on both ends so RTT ≈ 2 × per-link delay.
            meta["intf_a"].config(delay=delay_str)
            meta["intf_b"].config(delay=delay_str)

    def apply_mode(self, mode: str) -> None:
        self._require_net()
        mode = mode.lower().strip()
        if mode not in ("direct", "forwarding"):
            raise ValueError(f"unknown mode {mode!r}")
        self.mode = mode
        forward = "1" if mode == "forwarding" else "0"

        # Clear host routes to identity /32s, then reinstall.
        for name in self.topo["nodes"]:
            h = self.net.get(name)
            h.cmd(f"sysctl -w net.ipv4.ip_forward={forward}")
            # Identity IPs live on lo; multi-hop replies and Raft need loose rp_filter
            # and accept_local so packets to lo addresses may ingress on veths.
            h.cmd("sysctl -w net.ipv4.conf.all.rp_filter=0")
            h.cmd("sysctl -w net.ipv4.conf.default.rp_filter=0")
            h.cmd("sysctl -w net.ipv4.conf.all.accept_local=1")
            for intf in h.intfList():
                if intf.name == "lo":
                    continue
                # sysctl replaces '.' in iface names with '/'.
                sys_name = intf.name.replace(".", "/")
                h.cmd(f"sysctl -w net.ipv4.conf.{sys_name}.rp_filter=0")
            # Drop previous experiment identity routes (keep link-local /30 connected).
            for other, ip in self.idents.items():
                if other == name:
                    continue
                h.cmd(f"ip route del {ip}/32 2>/dev/null")

        # Direct on-link identity routes for every UP neighbor.
        # Pin src=identity so Raft TCP (which does not bind -I) uses a returnable
        # address; otherwise multi-hop peers only learn /32 identity routes and
        # cannot reply to a /30 link-local source (T3 spoke↔spoke timeouts).
        for key, meta in self.links.items():
            if self.down.get(key):
                continue
            a, b = meta["a"], meta["b"]
            ha, hb = self.net.get(a), self.net.get(b)
            ha.cmd(
                f"ip route replace {self.idents[b]}/32 via {meta['ip_b']} "
                f"dev {meta['intf_a'].name} src {self.idents[a]}"
            )
            hb.cmd(
                f"ip route replace {self.idents[a]}/32 via {meta['ip_a']} "
                f"dev {meta['intf_b'].name} src {self.idents[b]}"
            )

        if mode != "forwarding":
            return

        # Simplified NIFTY overlay: for every non-adjacent reachable pair, install
        # a /32 via the first hop of a shortest path on the surviving mesh.
        routes = self._compute_overlay_routes()
        self.topo["forwarding_routes"] = routes
        for route in routes:
            node, dest, via = route["node"], route["dest"], route["via"]
            leg = _link_key(node, via)
            if self.down.get(leg):
                continue
            meta = self.links[leg]
            if meta["a"] == node:
                via_ip = meta["ip_b"]
                out_dev = meta["intf_a"].name
            else:
                via_ip = meta["ip_a"]
                out_dev = meta["intf_b"].name
            h = self.net.get(node)
            dest_ip = self.idents[dest]
            src_ip = self.idents[node]
            h.cmd(f"ip route replace {dest_ip}/32 via {via_ip} dev {out_dev} src {src_ip}")

    def _compute_overlay_routes(self) -> List[Dict[str, str]]:
        """BFS next-hop table for multi-hop identity routes (surviving links only)."""
        nodes = list(self.topo.get("nodes") or [])
        adj: Dict[str, List[str]] = {n: [] for n in nodes}
        for key, meta in self.links.items():
            if self.down.get(key):
                continue
            a, b = meta["a"], meta["b"]
            adj[a].append(b)
            adj[b].append(a)

        routes: List[Dict[str, str]] = []
        for src in nodes:
            # dest → first hop from src (neighbour hop == dest means on-link).
            next_hop: Dict[str, str] = {src: ""}
            queue: List[Tuple[str, str]] = [(src, "")]
            qi = 0
            while qi < len(queue):
                cur, hop = queue[qi]
                qi += 1
                for nb in adj.get(cur, []):
                    if nb in next_hop:
                        continue
                    first = hop if hop else nb
                    next_hop[nb] = first
                    queue.append((nb, first))
            for dst in nodes:
                if src == dst:
                    continue
                via = next_hop.get(dst)
                if not via or via == dst:
                    continue
                routes.append({"node": src, "dest": dst, "via": via})
        routes.sort(key=lambda r: (r["node"], r["dest"], r["via"]))
        return routes

    def exec(self, node: str, cmd: str) -> Tuple[str, int]:
        self._require_net()
        if node not in self.idents:
            raise KeyError(f"unknown node {node}")
        h = self.net.get(node)
        # host.cmd returns combined stdout/stderr as a string; exit via echo.
        wrapped = f"{cmd}; printf '\\n__EXIT__%s' $?"
        out = h.cmd(wrapped)
        code = 0
        marker = "__EXIT__"
        if marker in out:
            body, _, tail = out.rpartition(marker)
            out = body
            try:
                code = int(tail.strip().split()[0])
            except (ValueError, IndexError):
                code = 1
        return out, code

    def validate_ping_matrix(self) -> Dict[str, Any]:
        """Server-side optional helper used by Go Validate (best-effort)."""
        self._require_net()
        results = []
        for src in self.topo["nodes"]:
            for dst, ip in self.idents.items():
                if src == dst:
                    continue
                # Bind source to identity IP so multi-hop return path uses identity routes
                # (default src would be the /30 link address, which remote hosts cannot route).
                src_ip = self.idents[src]
                out, code = self.exec(src, f"ping -c 1 -W 1 -I {src_ip} {ip}")
                results.append(
                    {"src": src, "dst": dst, "ip": ip, "ok": code == 0, "code": code}
                )
        return {"mode": self.mode, "results": results}

    def _require_net(self) -> None:
        if self.net is None:
            raise RuntimeError("no active network")


def _delay_str(delay_ms: float) -> str:
    if delay_ms <= 0:
        return "0ms"
    # Keep sub-ms precision for 0.5ms experiments.
    if abs(delay_ms - round(delay_ms)) < 1e-9:
        return f"{int(round(delay_ms))}ms"
    return f"{delay_ms}ms"
