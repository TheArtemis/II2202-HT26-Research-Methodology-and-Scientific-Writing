package main

import "testing"

func TestReorderFlagsAfterConfig(t *testing.T) {
	got := reorderFlags([]string{"configs/full.yaml", "--from", "113", "--limit", "113"})
	want := []string{"--from", "113", "--limit", "113", "configs/full.yaml"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestReorderFlagsBeforeConfig(t *testing.T) {
	in := []string{"--from", "113", "--limit", "113", "configs/full.yaml"}
	got := reorderFlags(in)
	for i := range in {
		if got[i] != in[i] {
			t.Fatalf("got %v want %v", got, in)
		}
	}
}
