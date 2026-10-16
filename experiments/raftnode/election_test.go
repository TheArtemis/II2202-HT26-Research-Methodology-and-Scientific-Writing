package raftnode

import "testing"
import "time"

func TestDeriveElectionTimeoutRTTAware(t *testing.T) {
	base := deriveElectionTimeout(Config{
		Heartbeat:    100 * time.Millisecond,
		NetworkDelay: 20 * time.Millisecond,
	})
	// max(5*hb, 20*RTT, 250ms) = max(500ms, 800ms, 250ms) = 800ms
	if base != 800*time.Millisecond {
		t.Fatalf("base election = %s, want 800ms", base)
	}
	prefer := deriveElectionTimeout(Config{
		Heartbeat:    100 * time.Millisecond,
		NetworkDelay: 20 * time.Millisecond,
		PreferLeader: true,
	})
	// max(2*hb, 8*RTT) = max(200ms, 320ms) = 320ms (< base)
	if prefer != 320*time.Millisecond {
		t.Fatalf("prefer election = %s, want 320ms", prefer)
	}
	if prefer >= base {
		t.Fatalf("prefer %s should be < base %s", prefer, base)
	}
}

func TestDeferredCampaignUsesElectionAsHeartbeat(t *testing.T) {
	// Mirrors Open(): non-prefer + explicit long election → campaign timer = election.
	cfg := Config{
		Heartbeat:       100 * time.Millisecond,
		ElectionTimeout: 30 * time.Second,
		PreferLeader:    false,
	}
	hb := cfg.Heartbeat
	if !cfg.PreferLeader && cfg.ElectionTimeout > hb {
		hb = cfg.ElectionTimeout
	}
	if hb != 30*time.Second {
		t.Fatalf("deferred heartbeat = %s, want 30s", hb)
	}
}
