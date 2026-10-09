package collect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestAnalyzeSyntheticRun(t *testing.T) {
	dir := t.TempDir()
	inject := time.Date(2026, 9, 26, 12, 0, 10, 0, time.UTC)
	start := inject.Add(-5 * time.Second)
	end := inject.Add(10 * time.Second)

	st := Status{
		RunID:      "T4_forwarding_d5ms_hb10ms_r01",
		OK:         true,
		StartedAt:  start,
		FinishedAt: end,
		InjectAt:   inject,
		Leader:     "B",
		Seed:       1,
		Topology:   "T4",
		Mode:       "forwarding",
		Delay:      "5ms",
		Heartbeat:  "10ms",
		Repetition: 1,
	}
	writeJSON(t, filepath.Join(dir, "status.json"), st)

	tl := Timeline{
		RunID:           st.RunID,
		ClientStartedNS: start.UnixNano(),
		WarmupEndNS:     inject.UnixNano(),
		InjectAtNS:      inject.UnixNano(),
		InjectAt:        inject.Format(time.RFC3339Nano),
		ObserveEndNS:    end.UnixNano(),
		Clock:           "CLOCK_REALTIME",
	}
	writeJSON(t, filepath.Join(dir, "timeline.json"), tl)

	var clients []ClientRecord
	for i := 0; i < 5; i++ {
		sub := start.Add(time.Duration(i) * 200 * time.Millisecond).UnixNano()
		clients = append(clients, ClientRecord{
			ID: strconv.Itoa(i + 1), SubmitNS: sub, CommitNS: sub + 1e6, LatencyNS: 1e6, OK: true, Index: uint64(i + 1),
		})
	}
	recSub := inject.Add(50 * time.Millisecond).UnixNano()
	clients = append(clients, ClientRecord{
		ID: "6", SubmitNS: recSub, CommitNS: recSub + 2e6, LatencyNS: 2e6, OK: true, Index: 6,
	})
	for i := 0; i < 10; i++ {
		sub := inject.Add(time.Duration(i+1) * time.Second).UnixNano()
		clients = append(clients, ClientRecord{
			ID: strconv.Itoa(i + 7), SubmitNS: sub, CommitNS: sub + 3e6, LatencyNS: 3e6, OK: true, Index: uint64(i + 7),
		})
	}
	writeJSONL(t, filepath.Join(dir, "client.jsonl"), clients)

	events := []RaftEvent{
		{TSNS: start.UnixNano(), Type: "election", Node: "B", Term: 1},
		{TSNS: start.UnixNano() + 1, Type: "term", Node: "B", Term: 1},
		{TSNS: inject.Add(10 * time.Millisecond).UnixNano(), Type: "election", Node: "B", Term: 2},
		{TSNS: inject.Add(10 * time.Millisecond).UnixNano() + 1, Type: "term", Node: "B", Term: 2},
		{TSNS: inject.Add(100 * time.Millisecond).UnixNano(), Type: "commit", Node: "B", Index: 10, Term: 2, Leader: "B"},
	}
	writeJSONL(t, filepath.Join(dir, "events-B.jsonl"), events)

	loads := []HostLoadSample{
		{TSNS: inject.UnixNano(), Load1: 1.5},
		{TSNS: inject.UnixNano() + 1e9, Load1: 2.0},
	}
	writeJSONL(t, filepath.Join(dir, "hostload.jsonl"), loads)

	m, err := AnalyzeRunDir(dir, 4.0)
	if err != nil {
		t.Fatal(err)
	}
	if m.ClientOKWarmup != 5 {
		t.Fatalf("warmup ok=%d", m.ClientOKWarmup)
	}
	if m.ClientOKObserve != 11 {
		t.Fatalf("observe ok=%d", m.ClientOKObserve)
	}
	if m.RecoveryTimeMS == nil || *m.RecoveryTimeMS < 49 || *m.RecoveryTimeMS > 55 {
		t.Fatalf("recovery=%v", m.RecoveryTimeMS)
	}
	if m.ElectionsObserve != 1 || m.TermChangesObserve != 1 {
		t.Fatalf("elections=%d terms=%d", m.ElectionsObserve, m.TermChangesObserve)
	}
	if m.LatencyCount == 0 || m.LatencyMedianMS == nil {
		t.Fatal("latency missing")
	}
	if m.CommitThroughputObserve <= 0 {
		t.Fatal("throughput")
	}
	if !m.StableProgress {
		t.Fatal("expected stable progress")
	}
	if m.HostLoadMax1 == nil || *m.HostLoadMax1 != 2.0 {
		t.Fatalf("load max=%v", m.HostLoadMax1)
	}
	if err := WriteMetricsJSON(dir, m); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "client_enriched.jsonl")); err != nil {
		t.Fatal(err)
	}
}

func TestJoinDataset(t *testing.T) {
	root := t.TempDir()
	run := filepath.Join(root, "T0_direct_d5ms_hb10ms_r01")
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	inject := time.Now().UTC()
	writeJSON(t, filepath.Join(run, "status.json"), Status{
		RunID: "T0_direct_d5ms_hb10ms_r01", OK: true,
		StartedAt: inject.Add(-2 * time.Second), FinishedAt: inject.Add(2 * time.Second),
		InjectAt: inject, Topology: "T0", Mode: "direct", Delay: "5ms", Heartbeat: "10ms", Repetition: 1,
	})
	writeJSON(t, filepath.Join(run, "timeline.json"), Timeline{
		RunID:           "T0_direct_d5ms_hb10ms_r01",
		ClientStartedNS: inject.Add(-2 * time.Second).UnixNano(),
		WarmupEndNS:     inject.UnixNano(),
		InjectAtNS:      inject.UnixNano(),
		ObserveEndNS:    inject.Add(2 * time.Second).UnixNano(),
		Clock:           "CLOCK_REALTIME",
	})
	sub := inject.Add(100 * time.Millisecond).UnixNano()
	writeJSONL(t, filepath.Join(run, "client.jsonl"), []ClientRecord{{
		ID: "1", SubmitNS: sub, CommitNS: sub + 1e6, LatencyNS: 1e6, OK: true,
	}})
	writeJSONL(t, filepath.Join(run, "events-A.jsonl"), []RaftEvent{})

	n, err := JoinDataset(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("joined %d", n)
	}
	for _, name := range []string{"dataset.jsonl", "dataset.csv"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func writeJSON(t *testing.T, path string, v interface{}) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeJSONL(t *testing.T, path string, rows interface{}) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	switch xs := rows.(type) {
	case []ClientRecord:
		for _, r := range xs {
			_ = enc.Encode(r)
		}
	case []RaftEvent:
		for _, r := range xs {
			_ = enc.Encode(r)
		}
	case []HostLoadSample:
		for _, r := range xs {
			_ = enc.Encode(r)
		}
	default:
		t.Fatalf("unsupported %T", rows)
	}
}
