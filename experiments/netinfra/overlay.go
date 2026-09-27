package netinfra

import "sort"

// ComputeOverlayRoutes returns next-hop routes for every non-adjacent pair that
// remains reachable on the surviving undirected graph (simplified NIFTY-style
// detour: reroute around down links through intermediate nodes).
//
// Direct neighbors are omitted — apply_mode already installs on-link routes.
// If YAML forwarding_routes is non-empty it is ignored by the Mininet daemon in
// favour of this computation (kept on the spec only for documentation/tests).
func ComputeOverlayRoutes(topo TopologySpec, down map[string]bool) []ForwardingRoute {
	adj := adjacency(topo, down)
	var out []ForwardingRoute
	for _, src := range topo.Nodes {
		next := bfsNextHop(adj, src)
		for _, dst := range topo.Nodes {
			if src == dst {
				continue
			}
			via, ok := next[dst]
			if !ok || via == "" {
				continue // unreachable
			}
			if via == dst {
				continue // on-link neighbour
			}
			out = append(out, ForwardingRoute{Node: src, Dest: dst, Via: via})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Node != b.Node {
			return a.Node < b.Node
		}
		if a.Dest != b.Dest {
			return a.Dest < b.Dest
		}
		return a.Via < b.Via
	})
	return out
}

// bfsNextHop maps dest → first hop from src on a shortest path (empty hop means
// dest is src). Missing dest keys are unreachable.
func bfsNextHop(adj map[string][]string, src string) map[string]string {
	next := map[string]string{src: ""}
	type item struct {
		node string
		hop  string // first hop from src toward node
	}
	q := []item{{node: src, hop: ""}}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		for _, n := range adj[cur.node] {
			if _, seen := next[n]; seen {
				continue
			}
			hop := cur.hop
			if hop == "" {
				hop = n // neighbour of src
			}
			next[n] = hop
			q = append(q, item{node: n, hop: hop})
		}
	}
	return next
}

// IsFullMesh reports whether every unordered pair of nodes has a link entry.
func IsFullMesh(topo TopologySpec) bool {
	n := len(topo.Nodes)
	if n < 2 {
		return true
	}
	want := n * (n - 1) / 2
	seen := make(map[string]struct{}, len(topo.Links))
	for _, link := range topo.Links {
		if len(link.Endpoints) != 2 {
			return false
		}
		seen[linkKey(link.Endpoints[0], link.Endpoints[1])] = struct{}{}
	}
	return len(seen) == want
}

// DownFromFailedMarks builds a down-set from YAML failed: true flags.
func DownFromFailedMarks(topo TopologySpec) map[string]bool {
	down := make(map[string]bool)
	for _, link := range topo.Links {
		if link.Failed {
			down[linkKey(link.Endpoints[0], link.Endpoints[1])] = true
		}
	}
	return down
}
