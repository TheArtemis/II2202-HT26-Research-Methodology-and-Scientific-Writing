package harness

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadPilotYAML(t *testing.T) {
	path := filepath.Join("..", "configs", "pilot.yaml")
	exp, err := LoadExperimentFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if exp.Name != "pilot" {
		t.Fatalf("name %q", exp.Name)
	}
	if exp.Matrix.Repetitions != 5 {
		t.Fatalf("reps %d", exp.Matrix.Repetitions)
	}
	if exp.Timing.Observe.Duration != 20*time.Second {
		t.Fatalf("observe %s", exp.Timing.Observe.Duration)
	}
	if exp.Workload.Rate != 100 {
		t.Fatalf("rate %v", exp.Workload.Rate)
	}
}

func TestExpandPilotCount(t *testing.T) {
	path := filepath.Join("..", "configs", "pilot.yaml")
	exp, err := LoadExperimentFile(path)
	if err != nil {
		t.Fatal(err)
	}
	trials, err := Expand(exp)
	if err != nil {
		t.Fatal(err)
	}
	// 5 topologies × 2 modes × 3 delays × 1 heartbeat × 5 reps = 150
	want := 5 * 2 * 3 * 1 * 5
	if len(trials) != want {
		t.Fatalf("got %d trials, want %d", len(trials), want)
	}
	if trials[0].RunID == "" || trials[0].Leader == "" {
		t.Fatalf("empty fields: %+v", trials[0])
	}
	// T4 initial leader is B
	var t4 *Trial
	for i := range trials {
		if trials[i].Topology == "T4" {
			t4 = &trials[i]
			break
		}
	}
	if t4 == nil || t4.Leader != "B" {
		t.Fatalf("T4 leader: %+v", t4)
	}
}

func TestExpandFullCount(t *testing.T) {
	path := filepath.Join("..", "configs", "full.yaml")
	exp, err := LoadExperimentFile(path)
	if err != nil {
		t.Fatal(err)
	}
	trials, err := Expand(exp)
	if err != nil {
		t.Fatal(err)
	}
	// 5×2×3×1×30 = 900
	if len(trials) != 900 {
		t.Fatalf("got %d, want 900", len(trials))
	}
}

// Contiguous equal shards must each see every topology — otherwise worker identity
// is confounded with topology when running --from/--limit across EC2 instances.
func TestExpandShardBalancesTopologies(t *testing.T) {
	path := filepath.Join("..", "configs", "full.yaml")
	exp, err := LoadExperimentFile(path)
	if err != nil {
		t.Fatal(err)
	}
	trials, err := Expand(exp)
	if err != nil {
		t.Fatal(err)
	}
	const workers = 5
	chunk := (len(trials) + workers - 1) / workers
	wantTopos := map[string]struct{}{}
	for _, name := range exp.Matrix.Topologies {
		wantTopos[strings.ToUpper(name)] = struct{}{}
		wantTopos[name] = struct{}{}
	}
	for w := 0; w < workers; w++ {
		from := w * chunk
		to := from + chunk
		if to > len(trials) {
			to = len(trials)
		}
		seen := map[string]int{}
		for _, tr := range trials[from:to] {
			seen[tr.Topology]++
		}
		for _, name := range exp.Matrix.Topologies {
			// Spec.Name is typically uppercase (T0…T4).
			n := seen[name] + seen[strings.ToUpper(name)]
			if n == 0 {
				t.Fatalf("worker %d shard [%d,%d) missing topology %s (seen=%v)", w, from, to, name, seen)
			}
		}
	}
	// First trial of the campaign should be rep 1 (outermost loop).
	if trials[0].Repetition != 1 {
		t.Fatalf("first trial rep=%d, want 1", trials[0].Repetition)
	}
	// With 30 conditions per rep, index 30 starts rep 2.
	if trials[30].Repetition != 2 {
		t.Fatalf("trial 30 rep=%d, want 2", trials[30].Repetition)
	}
}

func TestExpandSmoke(t *testing.T) {
	path := filepath.Join("..", "configs", "smoke.yaml")
	exp, err := LoadExperimentFile(path)
	if err != nil {
		t.Fatal(err)
	}
	trials, err := Expand(exp)
	if err != nil {
		t.Fatal(err)
	}
	if len(trials) != 2 {
		t.Fatalf("smoke trials %d", len(trials))
	}
}

func TestParseHalfMsDelay(t *testing.T) {
	d, err := time.ParseDuration("0.5ms")
	if err != nil {
		t.Fatal(err)
	}
	if d != 500*time.Microsecond {
		t.Fatalf("%s", d)
	}
}
