package raftnode

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/raft"
)

// Request is the opaque client command stored in the Raft log.
type Request struct {
	ID      string `json:"id"`
	Payload []byte `json:"payload,omitempty"`
}

// FSM is a minimal append-only state machine: it records applied request IDs
// and the latest Raft log index. Snapshots capture the applied set.
type FSM struct {
	mu      sync.RWMutex
	applied map[string]uint64 // request id → log index
	lastIdx uint64
	count   uint64
}

// NewFSM returns an empty FSM.
func NewFSM() *FSM {
	return &FSM{applied: make(map[string]uint64)}
}

// Apply implements raft.FSM.
func (f *FSM) Apply(log *raft.Log) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastIdx = log.Index
	f.count++

	var req Request
	if err := json.Unmarshal(log.Data, &req); err != nil {
		return fmt.Errorf("fsm decode: %w", err)
	}
	if req.ID != "" {
		f.applied[req.ID] = log.Index
	}
	return ApplyResult{ID: req.ID, Index: log.Index, Term: log.Term}
}

// ApplyResult is returned from FSM.Apply and exposed via ApplyFuture.Response().
type ApplyResult struct {
	ID    string `json:"id"`
	Index uint64 `json:"index"`
	Term  uint64 `json:"term"`
}

// Snapshot implements raft.FSM.
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	cp := make(map[string]uint64, len(f.applied))
	for k, v := range f.applied {
		cp[k] = v
	}
	return &fsmSnapshot{applied: cp, lastIdx: f.lastIdx, count: f.count}, nil
}

// Restore implements raft.FSM.
func (f *FSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	var snap fsmSnapshot
	if err := json.NewDecoder(rc).Decode(&snap); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if snap.applied == nil {
		snap.applied = make(map[string]uint64)
	}
	f.applied = snap.applied
	f.lastIdx = snap.lastIdx
	f.count = snap.count
	return nil
}

// Stats returns applied count and last index (for /stats).
func (f *FSM) Stats() (applied uint64, lastIdx uint64) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.count, f.lastIdx
}

// Has reports whether request id was applied.
func (f *FSM) Has(id string) (uint64, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	idx, ok := f.applied[id]
	return idx, ok
}

type fsmSnapshot struct {
	Applied map[string]uint64 `json:"applied"`
	LastIdx uint64            `json:"last_idx"`
	Count   uint64            `json:"count"`
	// lowercase fields used when building from Snapshot()
	applied map[string]uint64
	lastIdx uint64
	count   uint64
}

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		applied := s.applied
		if applied == nil {
			applied = s.Applied
		}
		last := s.lastIdx
		if last == 0 {
			last = s.LastIdx
		}
		count := s.count
		if count == 0 {
			count = s.Count
		}
		enc := json.NewEncoder(sink)
		return enc.Encode(fsmSnapshot{
			Applied: applied,
			LastIdx: last,
			Count:   count,
		})
	}()
	if err != nil {
		_ = sink.Cancel()
		return err
	}
	return sink.Close()
}

func (s *fsmSnapshot) Release() {}
