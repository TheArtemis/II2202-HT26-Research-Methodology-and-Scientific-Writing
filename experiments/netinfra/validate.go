package netinfra

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var pingRTT = regexp.MustCompile(`(?i)(?:rtt|round-trip)[^=]*=\s*[\d.]+/([\d.]+)/`)

// Validate implements Controller. It prefers daemon-side checks, then runs
// client-side T4 / connectivity gates via ExecOn.
func (c *Client) Validate(ctx context.Context) error {
	if !c.active {
		return fmt.Errorf("validate: no active topology (call Start first)")
	}
	// Optional daemon helper (best-effort; ignore unknown method).
	_ = c.call(ctx, "Validate", map[string]interface{}{}, nil)

	if err := c.validateConnectivity(ctx); err != nil {
		return err
	}
	if err := c.validateDelay(ctx); err != nil {
		return err
	}
	return nil
}

// ExpectedReachable reports whether src should reach dst's identity IP given
// mode and the set of currently-down links (from the YAML planned failures if
// injected, or an explicit down set).
func ExpectedReachable(topo TopologySpec, mode Mode, down map[string]bool, src, dst string) bool {
	if src == dst {
		return true
	}
	adj := adjacency(topo, down)
	if mode == Direct {
		for _, n := range adj[src] {
			if n == dst {
				return true
			}
		}
		return false
	}
	// Forwarding: BFS over surviving undirected graph.
	return pathExists(adj, src, dst)
}

func adjacency(topo TopologySpec, down map[string]bool) map[string][]string {
	adj := make(map[string][]string, len(topo.Nodes))
	for _, n := range topo.Nodes {
		adj[n] = nil
	}
	for _, link := range topo.Links {
		a, b := link.Endpoints[0], link.Endpoints[1]
		key := linkKey(a, b)
		if down[key] {
			continue
		}
		adj[a] = append(adj[a], b)
		adj[b] = append(adj[b], a)
	}
	return adj
}

func pathExists(adj map[string][]string, src, dst string) bool {
	seen := map[string]bool{src: true}
	q := []string{src}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		if cur == dst {
			return true
		}
		for _, n := range adj[cur] {
			if !seen[n] {
				seen[n] = true
				q = append(q, n)
			}
		}
	}
	return false
}

func (c *Client) actualDownSet() map[string]bool {
	out := make(map[string]bool, len(c.down))
	for k, v := range c.down {
		if v {
			out[k] = true
		}
	}
	return out
}

func (c *Client) validateConnectivity(ctx context.Context) error {
	_ = ctx
	idents, err := c.topo.IdentityMap()
	if err != nil {
		return err
	}
	down := c.actualDownSet()
	var failures []string

	// Full matrix for small topologies.
	for _, src := range c.topo.Nodes {
		for _, dst := range c.topo.Nodes {
			if src == dst {
				continue
			}
			want := ExpectedReachable(c.topo, c.mode, down, src, dst)
			ok, pingErr := c.pingIdentity(src, idents[dst])
			if want && !ok {
				failures = append(failures, fmt.Sprintf("%s→%s expected reachable (%v)", src, dst, pingErr))
			}
			if !want && ok {
				failures = append(failures, fmt.Sprintf("%s→%s unexpectedly reachable (bypass?)", src, dst))
			}
		}
	}

	// Explicit T4 pilot wording (only meaningful once B--C is down).
	if strings.EqualFold(c.topo.Name, "T4") && down[linkKey("B", "C")] {
		bToC, _ := c.pingIdentity("B", idents["C"])
		switch c.mode {
		case Direct:
			if bToC {
				failures = append(failures, "T4 pilot: B reached C in direct mode")
			}
		case Forwarding:
			if !bToC {
				failures = append(failures, "T4 pilot: B did not reach C via A in forwarding mode")
			}
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("connectivity validation failed:\n  - %s", strings.Join(failures, "\n  - "))
	}
	return nil
}

func (c *Client) pingIdentity(src, dstIP string) (bool, error) {
	// One probe, short timeout; identity /32.
	cmd := fmt.Sprintf("ping -c 1 -W 1 %s 2>&1; echo EXIT:$?", dstIP)
	out, err := c.rawExec(src, cmd)
	if err != nil && out == "" {
		return false, err
	}
	ok := strings.Contains(out, "1 received") || strings.Contains(out, "1 packets received") ||
		strings.Contains(out, "EXIT:0")
	// Prefer EXIT code if present.
	if i := strings.LastIndex(out, "EXIT:"); i >= 0 {
		code := strings.TrimSpace(out[i+5:])
		ok = code == "0"
	}
	return ok, nil
}

func (c *Client) rawExec(node, cmd string) (string, error) {
	params := map[string]interface{}{"node": node, "cmd": cmd}
	var out struct {
		Stdout string `json:"stdout"`
		Code   int    `json:"code"`
	}
	if err := c.call(context.Background(), "Exec", params, &out); err != nil {
		return out.Stdout, err
	}
	return out.Stdout, nil
}

func (c *Client) validateDelay(ctx context.Context) error {
	_ = ctx
	if c.delay <= 0 {
		return nil
	}
	idents, err := c.topo.IdentityMap()
	if err != nil {
		return err
	}
	down := c.actualDownSet()
	adj := adjacency(c.topo, down)

	// Direct-link check: first surviving edge.
	var a, b string
	for src, ns := range adj {
		if len(ns) > 0 {
			a, b = src, ns[0]
			break
		}
	}
	if a == "" {
		return nil
	}
	rtt, err := c.measureRTT(a, idents[b])
	if err != nil {
		return fmt.Errorf("delay check on %s→%s: %w", a, b, err)
	}
	// Symmetric TCLink delay on both ends ⇒ RTT ≈ 2 × per-link delay.
	expect := 2 * c.delay
	if !rttInBand(rtt, expect, 0.5) {
		return fmt.Errorf("direct delay check %s→%s: rtt=%s want ~%s (±50%%)", a, b, rtt, expect)
	}

	// Two-hop path in forwarding mode (T4: B→C via A).
	if c.mode == Forwarding && strings.EqualFold(c.topo.Name, "T4") {
		rtt2, err := c.measureRTT("B", idents["C"])
		if err != nil {
			return fmt.Errorf("T4 2-hop delay check: %w", err)
		}
		expect2 := 4 * c.delay // two links × both directions
		if !rttInBand(rtt2, expect2, 0.5) {
			return fmt.Errorf("T4 2-hop delay B→C: rtt=%s want ~%s (±50%%)", rtt2, expect2)
		}
	}
	return nil
}

func (c *Client) measureRTT(src, dstIP string) (time.Duration, error) {
	cmd := fmt.Sprintf("ping -c 3 -W 2 %s 2>&1", dstIP)
	out, err := c.rawExec(src, cmd)
	if err != nil && out == "" {
		return 0, err
	}
	m := pingRTT.FindStringSubmatch(out)
	if m == nil {
		return 0, fmt.Errorf("parse ping rtt from: %s", strings.TrimSpace(out))
	}
	ms, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(ms * float64(time.Millisecond)), nil
}

func rttInBand(got, want time.Duration, frac float64) bool {
	if want <= 0 {
		return true
	}
	lo := time.Duration(float64(want) * (1 - frac))
	hi := time.Duration(float64(want) * (1 + frac))
	// Floor for very small delays (scheduling noise).
	if lo < 200*time.Microsecond {
		lo = 0
	}
	return got >= lo && got <= hi
}
