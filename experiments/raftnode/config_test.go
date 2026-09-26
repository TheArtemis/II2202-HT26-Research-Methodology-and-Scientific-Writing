package raftnode

import (
	"testing"
)

func TestParsePeers(t *testing.T) {
	m, err := ParsePeers("C=10.0.0.3,A=10.0.0.1,B=10.0.0.2")
	if err != nil {
		t.Fatal(err)
	}
	if m["A"] != "10.0.0.1" || m["C"] != "10.0.0.3" {
		t.Fatalf("unexpected map: %#v", m)
	}
	s := FormatPeers(m)
	if s != "A=10.0.0.1,B=10.0.0.2,C=10.0.0.3" {
		t.Fatalf("FormatPeers: %q", s)
	}
}

func TestParseSeedLog(t *testing.T) {
	terms, err := ParseSeedLog("1,1,2,2")
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 4 || terms[2] != 2 {
		t.Fatalf("%v", terms)
	}
	empty, err := ParseSeedLog("")
	if err != nil || empty != nil {
		t.Fatalf("empty: %v %v", empty, err)
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := Config{
		ID:     "C",
		BindIP: "10.0.0.3",
		Peers:  map[string]string{"C": "10.0.0.3", "A": "10.0.0.1"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.ID = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error")
	}
}
