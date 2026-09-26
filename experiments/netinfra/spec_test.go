package netinfra_test

import (
	"strings"
	"testing"
	"time"

	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/netinfra"
)

func TestLoadEmbeddedSpecs(t *testing.T) {
	names, err := netinfra.ListEmbeddedSpecs()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"t0", "t1", "t2", "t3", "t4"}
	if len(names) != len(want) {
		t.Fatalf("got %v want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("got %v want %v", names, want)
		}
	}
	for _, n := range want {
		spec, err := netinfra.LoadNamedSpec(n)
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		if !strings.EqualFold(spec.Name, n) {
			t.Fatalf("%s name=%q", n, spec.Name)
		}
	}
}

func TestT4IdentityAndFailures(t *testing.T) {
	spec, err := netinfra.LoadNamedSpec("t4")
	if err != nil {
		t.Fatal(err)
	}
	ipA, _ := spec.IdentityIP("A")
	ipB, _ := spec.IdentityIP("B")
	ipC, _ := spec.IdentityIP("C")
	if ipA != "10.0.0.1" || ipB != "10.0.0.2" || ipC != "10.0.0.3" {
		t.Fatalf("idents A=%s B=%s C=%s", ipA, ipB, ipC)
	}
	fails := spec.PlannedFailures()
	if len(fails) != 1 || fails[0][0] != "B" || fails[0][1] != "C" {
		t.Fatalf("planned failures: %v", fails)
	}
}

func TestExpectedReachableT4(t *testing.T) {
	spec, err := netinfra.LoadNamedSpec("t4")
	if err != nil {
		t.Fatal(err)
	}
	down := map[string]bool{"B--C": true}
	if netinfra.ExpectedReachable(spec, netinfra.Direct, down, "B", "C") {
		t.Fatal("direct B→C should be unreachable")
	}
	if !netinfra.ExpectedReachable(spec, netinfra.Forwarding, down, "B", "C") {
		t.Fatal("forwarding B→C should be reachable via A")
	}
	if !netinfra.ExpectedReachable(spec, netinfra.Direct, down, "B", "A") {
		t.Fatal("direct B→A should be reachable")
	}
}

func TestExpectedReachableT2(t *testing.T) {
	spec, err := netinfra.LoadNamedSpec("t2")
	if err != nil {
		t.Fatal(err)
	}
	down := map[string]bool{}
	for _, link := range spec.Links {
		if link.Failed {
			a, b := link.Endpoints[0], link.Endpoints[1]
			if a > b {
				a, b = b, a
			}
			down[a+"--"+b] = true
		}
	}
	if netinfra.ExpectedReachable(spec, netinfra.Direct, down, "C", "B") {
		t.Fatal("direct C→B should fail under T2")
	}
	if !netinfra.ExpectedReachable(spec, netinfra.Forwarding, down, "C", "B") {
		t.Fatal("forwarding C→B via A should succeed under T2")
	}
}

func TestParseMode(t *testing.T) {
	m, err := netinfra.ParseMode("forwarding")
	if err != nil || m != netinfra.Forwarding {
		t.Fatalf("got %v %v", m, err)
	}
	if _, err := netinfra.ParseMode("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseDelayDuration(t *testing.T) {
	d, err := time.ParseDuration("0.5ms")
	if err != nil {
		t.Fatal(err)
	}
	if d != 500*time.Microsecond {
		t.Fatalf("got %v", d)
	}
}
