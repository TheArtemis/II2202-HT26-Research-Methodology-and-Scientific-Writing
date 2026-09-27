package netinfra_test

import (
	"testing"

	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/netinfra"
)

func TestAllSpecsAreFullMesh(t *testing.T) {
	names, err := netinfra.ListEmbeddedSpecs()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		spec, err := netinfra.LoadNamedSpec(n)
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		if !netinfra.IsFullMesh(spec) {
			t.Fatalf("%s: not a full mesh (%d links for %d nodes)", n, len(spec.Links), len(spec.Nodes))
		}
	}
}

func TestOverlayRoutesT4(t *testing.T) {
	spec, err := netinfra.LoadNamedSpec("t4")
	if err != nil {
		t.Fatal(err)
	}
	down := netinfra.DownFromFailedMarks(spec)
	routes := netinfra.ComputeOverlayRoutes(spec, down)
	// B↔C via A only (the failed direct edge).
	found := map[string]string{}
	for _, r := range routes {
		found[r.Node+"→"+r.Dest] = r.Via
	}
	if found["B→C"] != "A" || found["C→B"] != "A" {
		t.Fatalf("routes=%v want B→C via A and C→B via A", routes)
	}
	if _, ok := found["B→A"]; ok {
		t.Fatal("on-link B→A must not appear in overlay routes")
	}
}

func TestOverlayRoutesT2SpokeViaA(t *testing.T) {
	spec, err := netinfra.LoadNamedSpec("t2")
	if err != nil {
		t.Fatal(err)
	}
	down := netinfra.DownFromFailedMarks(spec)
	routes := netinfra.ComputeOverlayRoutes(spec, down)
	found := map[string]string{}
	for _, r := range routes {
		found[r.Node+"→"+r.Dest] = r.Via
	}
	if found["C→B"] != "A" || found["B→D"] != "A" {
		t.Fatalf("spoke detours missing: %v", found)
	}
	if !netinfra.ExpectedReachable(spec, netinfra.Forwarding, down, "C", "B") {
		t.Fatal("C should reach B under forwarding")
	}
	if netinfra.ExpectedReachable(spec, netinfra.Direct, down, "C", "B") {
		t.Fatal("C should not reach B under direct")
	}
}

func TestOverlayRoutesT3IsolatesC(t *testing.T) {
	spec, err := netinfra.LoadNamedSpec("t3")
	if err != nil {
		t.Fatal(err)
	}
	down := netinfra.DownFromFailedMarks(spec)
	if netinfra.ExpectedReachable(spec, netinfra.Forwarding, down, "B", "C") {
		t.Fatal("C must stay unreachable even with overlay")
	}
	if !netinfra.ExpectedReachable(spec, netinfra.Forwarding, down, "B", "D") {
		t.Fatal("B should reach D via A under overlay")
	}
	routes := netinfra.ComputeOverlayRoutes(spec, down)
	for _, r := range routes {
		if r.Dest == "C" || r.Node == "C" {
			t.Fatalf("no overlay route should involve isolated C: %+v", r)
		}
	}
}
