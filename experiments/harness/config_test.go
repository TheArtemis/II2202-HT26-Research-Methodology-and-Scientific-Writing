package harness

import (
	"path/filepath"
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
	// 5×2×3×1×20 = 600
	if len(trials) != 600 {
		t.Fatalf("got %d, want 600", len(trials))
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
