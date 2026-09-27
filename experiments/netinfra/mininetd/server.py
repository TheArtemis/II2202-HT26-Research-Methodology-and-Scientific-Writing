#!/usr/bin/env python3
"""Thin Mininet 2.3.0 JSON-RPC daemon for Go netinfra.

No Raft, no experiment loops — only Start/SetLink/SetDelay/SetMode/Exec/Validate/Stop.

Listen (prefer Unix socket):
  sudo python3 server.py --socket /tmp/mininetd.sock
  sudo python3 server.py --tcp 127.0.0.1:17300

Requires root + system Mininet 2.3.0 on Linux.
"""

from __future__ import annotations

import argparse
import json
import logging
import os
import signal
import socket
import sys
import traceback
from typing import Any, Dict, Optional

from topology_builder import TopologyRuntime

log = logging.getLogger("mininetd")


class MininetDaemon:
    def __init__(self) -> None:
        self.rt = TopologyRuntime()

    def handle(self, req: Dict[str, Any]) -> Dict[str, Any]:
        req_id = req.get("id")
        method = req.get("method")
        params = req.get("params") or {}
        if not isinstance(params, dict):
            params = _coerce_params(method, params)
        try:
            result = self.dispatch(method, params)
            return {"jsonrpc": "2.0", "id": req_id, "result": result}
        except Exception as exc:  # noqa: BLE001 — surface to Go client
            log.exception("method %s failed", method)
            return {
                "jsonrpc": "2.0",
                "id": req_id,
                "error": {"code": -32000, "message": str(exc)},
            }

    def dispatch(self, method: Optional[str], params: Dict[str, Any]) -> Any:
        if method == "Start":
            topo = params["topo"]
            mode = params.get("mode", "direct")
            delay_ms = float(params.get("delay_ms", 0))
            self.rt.start(topo, mode, delay_ms)
            return {"ok": True, "name": topo.get("name"), "mode": mode, "delay_ms": delay_ms}
        if method == "SetLink":
            self.rt.set_link(params["a"], params["b"], bool(params["up"]))
            return {"ok": True}
        if method == "SetDelay":
            self.rt.set_delay(float(params["delay_ms"]))
            return {"ok": True}
        if method == "SetMode":
            self.rt.apply_mode(params["mode"])
            return {"ok": True, "mode": self.rt.mode}
        if method == "SetForwardingRoutes":
            # Overlay routes are derived from the surviving mesh; this RPC only
            # forces a refresh of apply_mode (params.routes is ignored).
            self.rt.apply_mode(self.rt.mode)
            n = len(self.rt.topo.get("forwarding_routes") or [])
            return {"ok": True, "count": n}
        if method == "Exec":
            out, code = self.rt.exec(params["node"], params["cmd"])
            return {"stdout": out, "code": code}
        if method == "Validate":
            return self.rt.validate_ping_matrix()
        if method == "Stop":
            self.rt.stop()
            return {"ok": True}
        if method == "Ping":
            return {"pong": True}
        raise ValueError(f"unknown method {method!r}")


def _coerce_params(method: Optional[str], params: Any) -> Dict[str, Any]:
    if isinstance(params, dict):
        return params
    if not isinstance(params, list):
        return {}
    if method == "SetLink" and len(params) >= 3:
        return {"a": params[0], "b": params[1], "up": params[2]}
    if method == "Exec" and len(params) >= 2:
        return {"node": params[0], "cmd": params[1]}
    if method == "SetDelay" and len(params) >= 1:
        return {"delay_ms": params[0]}
    if method == "SetMode" and len(params) >= 1:
        return {"mode": params[0]}
    if method == "Start" and len(params) >= 3:
        return {"topo": params[0], "mode": params[1], "delay_ms": params[2]}
    return {}


def _serve_connection(daemon: MininetDaemon, conn: socket.socket) -> None:
    buf = b""
    with conn:
        while True:
            chunk = conn.recv(65536)
            if not chunk:
                break
            buf += chunk
            while b"\n" in buf:
                line, buf = buf.split(b"\n", 1)
                line = line.strip()
                if not line:
                    continue
                try:
                    req = json.loads(line.decode("utf-8"))
                except json.JSONDecodeError as exc:
                    resp = {
                        "jsonrpc": "2.0",
                        "id": None,
                        "error": {"code": -32700, "message": f"parse error: {exc}"},
                    }
                else:
                    resp = daemon.handle(req)
                conn.sendall((json.dumps(resp) + "\n").encode("utf-8"))


def serve_unix(path: str, daemon: MininetDaemon) -> None:
    if os.path.exists(path):
        os.unlink(path)
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    sock.bind(path)
    os.chmod(path, 0o666)
    sock.listen(8)
    log.info("listening on unix:%s", path)
    _accept_loop(sock, daemon)


def serve_tcp(host: str, port: int, daemon: MininetDaemon) -> None:
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind((host, port))
    sock.listen(8)
    log.info("listening on tcp:%s:%d", host, port)
    _accept_loop(sock, daemon)


def _accept_loop(sock: socket.socket, daemon: MininetDaemon) -> None:
    def _shutdown(signum, frame):  # noqa: ANN001
        log.info("signal %s — stopping network and exiting", signum)
        try:
            daemon.rt.stop()
        finally:
            sock.close()
            sys.exit(0)

    signal.signal(signal.SIGINT, _shutdown)
    signal.signal(signal.SIGTERM, _shutdown)
    while True:
        conn, _addr = sock.accept()
        try:
            _serve_connection(daemon, conn)
        except Exception:  # noqa: BLE001
            log.error("connection error:\n%s", traceback.format_exc())


def main(argv: Optional[list] = None) -> int:
    parser = argparse.ArgumentParser(description="Thin Mininet JSON-RPC daemon")
    parser.add_argument(
        "--socket",
        default=os.environ.get("MININETD_SOCKET", "/tmp/mininetd.sock"),
        help="Unix socket path (default /tmp/mininetd.sock)",
    )
    parser.add_argument(
        "--tcp",
        default=os.environ.get("MININETD_TCP", ""),
        help="If set, listen on host:port instead of Unix socket (e.g. 127.0.0.1:17300)",
    )
    parser.add_argument("-v", "--verbose", action="store_true")
    args = parser.parse_args(argv)

    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )

    if os.geteuid() != 0:
        log.warning("not running as root — Mininet will likely fail to create namespaces")

    daemon = MininetDaemon()
    if args.tcp:
        host, _, port_s = args.tcp.partition(":")
        serve_tcp(host or "127.0.0.1", int(port_s or "17300"), daemon)
    else:
        serve_unix(args.socket, daemon)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
