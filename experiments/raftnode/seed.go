package raftnode

import (
	"fmt"

	"github.com/hashicorp/raft"
)

// seedStores writes an initial configuration plus command log entries whose
// terms match seed (research-plan T3). Index 1 is always the cluster
// configuration with term = seed[0]; remaining seed terms become empty commands.
func seedStores(logs raft.LogStore, stable raft.StableStore, peers map[string]string, raftPort int, seed []uint64) error {
	if len(seed) == 0 {
		return fmt.Errorf("empty seed log")
	}
	maxTerm := seed[0]
	for _, t := range seed {
		if t > maxTerm {
			maxTerm = t
		}
	}
	if err := stable.SetUint64([]byte("CurrentTerm"), maxTerm); err != nil {
		return fmt.Errorf("set term: %w", err)
	}

	configuration := clusterConfiguration(peers, raftPort)
	cfgData := raft.EncodeConfiguration(configuration)

	entries := make([]*raft.Log, 0, len(seed))
	// Index 1: configuration using first seed term.
	entries = append(entries, &raft.Log{
		Index: 1,
		Term:  seed[0],
		Type:  raft.LogConfiguration,
		Data:  cfgData,
	})
	for i := 1; i < len(seed); i++ {
		entries = append(entries, &raft.Log{
			Index: uint64(i + 1),
			Term:  seed[i],
			Type:  raft.LogCommand,
			Data:  []byte(fmt.Sprintf(`{"id":"seed-%d"}`, i+1)),
		})
	}
	if err := logs.StoreLogs(entries); err != nil {
		return fmt.Errorf("store seed logs: %w", err)
	}
	return nil
}

func clusterConfiguration(peers map[string]string, raftPort int) raft.Configuration {
	if raftPort == 0 {
		raftPort = DefaultRaftPort
	}
	servers := make([]raft.Server, 0, len(peers))
	for id, ip := range peers {
		servers = append(servers, raft.Server{
			Suffrage: raft.Voter,
			ID:       raft.ServerID(id),
			Address:  raft.ServerAddress(fmt.Sprintf("%s:%d", ip, raftPort)),
		})
	}
	return raft.Configuration{Servers: servers}
}
