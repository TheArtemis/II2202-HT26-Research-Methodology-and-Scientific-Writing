package raftnode

import (
	"fmt"
	"strings"
	"time"
)

const (
	// DefaultRaftPort is the HashiCorp Raft TCP transport port on each identity IP.
	DefaultRaftPort = 7000
	// DefaultAPIPort is the client-facing HTTP Apply API port.
	DefaultAPIPort = 7001
	// DefaultHeartbeat is the research-plan mid-range heartbeat interval.
	DefaultHeartbeat = 10 * time.Millisecond
)

// Config configures one Raft replica.
type Config struct {
	// ID is the Raft server ID (node name: A, B, C, …).
	ID string
	// BindIP is the stable identity address (e.g. 10.0.0.3).
	BindIP string
	// RaftPort is the Raft RPC listen port (default 7000).
	RaftPort int
	// APIPort is the HTTP Apply API listen port (default 7001).
	APIPort int
	// Peers maps node ID → identity IP (must include self).
	Peers map[string]string
	// Heartbeat is HeartbeatTimeout / LeaderLeaseTimeout (research IV).
	Heartbeat time.Duration
	// ElectionTimeout overrides the derived election timeout when > 0.
	ElectionTimeout time.Duration
	// PreferLeader shortens this node's election timeout so it wins warm-up.
	PreferLeader bool
	// Bootstrap writes the initial cluster configuration if stores are empty.
	Bootstrap bool
	// DataDir holds bolt/snapshot state. Empty → in-memory stores (clean runs).
	DataDir string
	// EventsPath is a JSONL file for leader/term/commit observations. Empty → stderr.
	EventsPath string
	// SeedLogTerms optionally pre-seeds per-index terms (T3 constrained election).
	// Example: []uint64{1,1,2,2}. Nil skips seeding.
	SeedLogTerms []uint64
}

// RaftAddr returns host:port for the Raft transport.
func (c Config) RaftAddr() string {
	port := c.RaftPort
	if port == 0 {
		port = DefaultRaftPort
	}
	return fmt.Sprintf("%s:%d", c.BindIP, port)
}

// APIAddr returns host:port for the HTTP Apply API.
func (c Config) APIAddr() string {
	port := c.APIPort
	if port == 0 {
		port = DefaultAPIPort
	}
	return fmt.Sprintf("%s:%d", c.BindIP, port)
}

// Validate checks required fields.
func (c Config) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return fmt.Errorf("raftnode: ID is required")
	}
	if strings.TrimSpace(c.BindIP) == "" {
		return fmt.Errorf("raftnode: BindIP is required")
	}
	if len(c.Peers) == 0 {
		return fmt.Errorf("raftnode: Peers must be non-empty")
	}
	if _, ok := c.Peers[c.ID]; !ok {
		return fmt.Errorf("raftnode: Peers must include self %q", c.ID)
	}
	if c.Heartbeat < 0 {
		return fmt.Errorf("raftnode: Heartbeat must be >= 0")
	}
	return nil
}

// ParsePeers parses "A=10.0.0.1,B=10.0.0.2" into a map.
func ParsePeers(s string) (map[string]string, error) {
	out := make(map[string]string)
	s = strings.TrimSpace(s)
	if s == "" {
		return out, fmt.Errorf("empty peers")
	}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("peer %q: want ID=IP", part)
		}
		id, ip := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
		if id == "" || ip == "" {
			return nil, fmt.Errorf("peer %q: empty id or ip", part)
		}
		out[id] = ip
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no peers parsed")
	}
	return out, nil
}

// FormatPeers renders peers as ID=IP,... sorted by ID for stable CLI args.
func FormatPeers(peers map[string]string) string {
	ids := make([]string, 0, len(peers))
	for id := range peers {
		ids = append(ids, id)
	}
	// Insertion sort keeps this file free of sort import churn for tiny N.
	for i := 1; i < len(ids); i++ {
		j := i
		for j > 0 && ids[j-1] > ids[j] {
			ids[j-1], ids[j] = ids[j], ids[j-1]
			j--
		}
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, id+"="+peers[id])
	}
	return strings.Join(parts, ",")
}

// ParseSeedLog parses "1,1,2,2" into term slices.
func ParseSeedLog(s string) ([]uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]uint64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		var t uint64
		if _, err := fmt.Sscanf(p, "%d", &t); err != nil {
			return nil, fmt.Errorf("seed-log term %q: %w", p, err)
		}
		if t == 0 {
			return nil, fmt.Errorf("seed-log terms must be >= 1")
		}
		out = append(out, t)
	}
	return out, nil
}
