package raftnode

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
)

// Node wraps a HashiCorp Raft replica plus client HTTP API and event logger.
type Node struct {
	Config Config
	Raft   *raft.Raft
	FSM    *FSM
	API    *APIServer
	Events *EventLogger

	transport *raft.NetworkTransport
}

// Open creates stores, optionally seeds/bootstraps, and starts Raft + HTTP API.
func Open(cfg Config) (*Node, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Heartbeat == 0 {
		cfg.Heartbeat = DefaultHeartbeat
	}
	// HashiCorp raft.ValidateConfig requires HeartbeatTimeout >= 5ms.
	if cfg.Heartbeat < 5*time.Millisecond {
		cfg.Heartbeat = 5 * time.Millisecond
	}
	if cfg.RaftPort == 0 {
		cfg.RaftPort = DefaultRaftPort
	}
	if cfg.APIPort == 0 {
		cfg.APIPort = DefaultAPIPort
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "raftd-" + cfg.ID,
		Level: hclog.Info,
	})

	logs, stable, snaps, err := openStores(cfg)
	if err != nil {
		return nil, err
	}

	hasState, err := raft.HasExistingState(logs, stable, snaps)
	if err != nil {
		return nil, fmt.Errorf("check state: %w", err)
	}

	if !hasState {
		switch {
		case len(cfg.SeedLogTerms) > 0:
			if err := seedStores(logs, stable, cfg.Peers, cfg.RaftPort, cfg.SeedLogTerms); err != nil {
				return nil, err
			}
		case cfg.Bootstrap:
			configuration := clusterConfiguration(cfg.Peers, cfg.RaftPort)
			bc := raft.DefaultConfig()
			bc.LocalID = raft.ServerID(cfg.ID)
			if err := raft.BootstrapCluster(bc, logs, stable, snaps, nil, configuration); err != nil {
				return nil, fmt.Errorf("bootstrap: %w", err)
			}
		}
	}

	addr := cfg.RaftAddr()
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("resolve raft addr: %w", err)
	}
	transport, err := raft.NewTCPTransport(addr, tcpAddr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("raft transport: %w", err)
	}

	rc := raft.DefaultConfig()
	rc.LocalID = raft.ServerID(cfg.ID)
	rc.Logger = logger
	election := deriveElectionTimeout(cfg)
	// HashiCorp starts follower elections on HeartbeatTimeout (not ElectionTimeout).
	// Non-preferred nodes with an explicit long ElectionTimeout therefore also get
	// that value as HeartbeatTimeout so they do not steal warm-up leadership.
	hb := cfg.Heartbeat
	if !cfg.PreferLeader && cfg.ElectionTimeout > hb {
		hb = cfg.ElectionTimeout
	}
	if election < hb {
		election = hb
	}
	rc.HeartbeatTimeout = hb
	rc.LeaderLeaseTimeout = cfg.Heartbeat // research IV; may be << deferred hb
	rc.ElectionTimeout = election
	rc.CommitTimeout = cfg.Heartbeat
	if err := raft.ValidateConfig(rc); err != nil {
		_ = transport.Close()
		return nil, fmt.Errorf("raft config: %w", err)
	}

	fsm := NewFSM()
	r, err := raft.NewRaft(rc, fsm, logs, stable, snaps, transport)
	if err != nil {
		_ = transport.Close()
		return nil, fmt.Errorf("new raft: %w", err)
	}

	events, err := NewEventLogger(cfg.ID, cfg.EventsPath)
	if err != nil {
		_ = r.Shutdown().Error()
		return nil, err
	}
	events.Observe(r)

	n := &Node{
		Config:    cfg,
		Raft:      r,
		FSM:       fsm,
		Events:    events,
		transport: transport,
	}
	api, err := NewAPIServer(n, cfg.APIAddr())
	if err != nil {
		_ = n.Close()
		return nil, err
	}
	n.API = api
	return n, nil
}

func openStores(cfg Config) (raft.LogStore, raft.StableStore, raft.SnapshotStore, error) {
	if cfg.DataDir == "" {
		logs := raft.NewInmemStore()
		return logs, logs, raft.NewInmemSnapshotStore(), nil
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, nil, nil, err
	}
	// File snapshot store + in-memory log/stable keeps runs simple while still
	// supporting snapshots. Durable bolt can be swapped in later if needed.
	logs := raft.NewInmemStore()
	snaps, err := raft.NewFileSnapshotStore(filepath.Join(cfg.DataDir, "snaps"), 2, os.Stderr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("snapshots: %w", err)
	}
	return logs, logs, snaps, nil
}

// WaitForLeader blocks until this node is leader or timeout.
func (n *Node) WaitForLeader(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if n.Raft.State() == raft.Leader {
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fmt.Errorf("node %s: not leader after %s (leader=%s state=%s)",
		n.Config.ID, timeout, n.Raft.Leader(), n.Raft.State())
}

// Apply submits a client request on the leader.
func (n *Node) Apply(req Request, timeout time.Duration) (ApplyResult, error) {
	if n.Raft.State() != raft.Leader {
		return ApplyResult{}, NotLeaderError{LeaderAddr: string(n.Raft.Leader()), LeaderID: leaderIDFromPeers(n)}
	}
	data, err := jsonMarshal(req)
	if err != nil {
		return ApplyResult{}, err
	}
	f := n.Raft.Apply(data, timeout)
	if err := f.Error(); err != nil {
		return ApplyResult{}, err
	}
	if resp, ok := f.Response().(ApplyResult); ok {
		return resp, nil
	}
	return ApplyResult{ID: req.ID, Index: f.Index()}, nil
}

// Close shuts down the API, Raft, and event logger.
func (n *Node) Close() error {
	var first error
	if n.API != nil {
		if err := n.API.Close(); err != nil && first == nil {
			first = err
		}
	}
	if n.Raft != nil {
		if err := n.Raft.Shutdown().Error(); err != nil && first == nil {
			first = err
		}
	}
	if n.transport != nil {
		if err := n.transport.Close(); err != nil && first == nil {
			first = err
		}
	}
	if n.Events != nil {
		if err := n.Events.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// NotLeaderError tells clients which peer may be leader.
type NotLeaderError struct {
	LeaderAddr string
	LeaderID   string
}

func (e NotLeaderError) Error() string {
	if e.LeaderID != "" {
		return fmt.Sprintf("not leader; try %s", e.LeaderID)
	}
	if e.LeaderAddr != "" {
		return fmt.Sprintf("not leader; try %s", e.LeaderAddr)
	}
	return "not leader"
}

func leaderIDFromPeers(n *Node) string {
	addr := string(n.Raft.Leader())
	if addr == "" {
		return ""
	}
	for id, ip := range n.Config.Peers {
		if fmt.Sprintf("%s:%d", ip, n.Config.RaftPort) == addr {
			return id
		}
	}
	return ""
}

// deriveElectionTimeout keeps election ≫ RTT (≈ 2×NetworkDelay) while still
// letting PreferLeader nodes campaign sooner than peers.
func deriveElectionTimeout(cfg Config) time.Duration {
	if cfg.ElectionTimeout > 0 {
		return cfg.ElectionTimeout
	}
	rtt := 2 * cfg.NetworkDelay
	base := 5 * cfg.Heartbeat
	if v := 20 * rtt; v > base {
		base = v
	}
	if base < 250*time.Millisecond {
		base = 250 * time.Millisecond
	}
	if !cfg.PreferLeader {
		return base
	}
	prefer := 2 * cfg.Heartbeat
	if v := 8 * rtt; v > prefer {
		prefer = v
	}
	if prefer < cfg.Heartbeat {
		prefer = cfg.Heartbeat
	}
	if prefer >= base {
		prefer = base / 2
		if prefer < cfg.Heartbeat {
			prefer = cfg.Heartbeat
		}
	}
	return prefer
}

func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
