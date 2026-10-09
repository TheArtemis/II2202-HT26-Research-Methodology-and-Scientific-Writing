package raftnode

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/raft"
)

// EventType classifies Raft observation records for RQ1 analysis.
type EventType string

const (
	EventLeader   EventType = "leader"
	EventTerm     EventType = "term"
	EventCommit   EventType = "commit"
	EventElection EventType = "election"
)

// Event is one Raft observation written as JSONL.
type Event struct {
	TS     time.Time `json:"ts"`
	TSNS   int64     `json:"ts_ns"`
	Type   EventType `json:"type"`
	Node   string    `json:"node"`
	Leader string    `json:"leader,omitempty"`
	Term   uint64    `json:"term,omitempty"`
	Index  uint64    `json:"index,omitempty"`
}

// EventLogger appends JSONL Raft events.
type EventLogger struct {
	node string
	w    io.Writer
	mu   sync.Mutex
	file *os.File

	lastLeader string
	lastTerm   uint64
	stop       chan struct{}
}

// NewEventLogger writes to path, or stderr when path is empty.
func NewEventLogger(node, path string) (*EventLogger, error) {
	el := &EventLogger{node: node, w: os.Stderr, stop: make(chan struct{})}
	if path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("events file: %w", err)
		}
		el.file = f
		el.w = f
	}
	return el, nil
}

// Close stops background pollers and closes an underlying file if any.
func (el *EventLogger) Close() error {
	select {
	case <-el.stop:
	default:
		close(el.stop)
	}
	if el.file != nil {
		return el.file.Close()
	}
	return nil
}

// Log writes one event.
func (el *EventLogger) Log(ev Event) {
	el.mu.Lock()
	defer el.mu.Unlock()
	now := time.Now().UTC()
	if ev.TS.IsZero() {
		ev.TS = now
	}
	if ev.TSNS == 0 {
		ev.TSNS = now.UnixNano()
	}
	ev.Node = el.node
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	_, _ = el.w.Write(append(b, '\n'))
}

// Observe registers a Raft observer that records leader, term, and commit events.
func (el *EventLogger) Observe(r *raft.Raft) {
	ch := make(chan raft.Observation, 64)
	obs := raft.NewObserver(ch, false, func(o *raft.Observation) bool {
		switch o.Data.(type) {
		case raft.LeaderObservation, raft.RaftState:
			return true
		default:
			return false
		}
	})
	r.RegisterObserver(obs)

	go func() {
		for {
			select {
			case <-el.stop:
				return
			case o, ok := <-ch:
				if !ok {
					return
				}
				el.handleObservation(r, o)
			}
		}
	}()

	go el.pollCommits(r)
}

func (el *EventLogger) handleObservation(r *raft.Raft, o raft.Observation) {
	switch d := o.Data.(type) {
	case raft.LeaderObservation:
		leader := string(d.LeaderID)
		term := r.CurrentTerm()
		el.mu.Lock()
		leaderChanged := leader != el.lastLeader
		termChanged := term != el.lastTerm && term > 0
		prevTerm := el.lastTerm
		if leaderChanged {
			el.lastLeader = leader
		}
		if termChanged {
			el.lastTerm = term
		}
		el.mu.Unlock()
		if leaderChanged {
			el.Log(Event{Type: EventLeader, Leader: leader, Term: term})
		}
		if termChanged {
			el.Log(Event{Type: EventTerm, Leader: leader, Term: term})
			if term > prevTerm {
				el.Log(Event{Type: EventElection, Leader: leader, Term: term})
			}
		}
	case raft.RaftState:
		term := r.CurrentTerm()
		el.mu.Lock()
		termChanged := term != el.lastTerm && term > 0
		prevTerm := el.lastTerm
		if termChanged {
			el.lastTerm = term
		}
		el.mu.Unlock()
		if termChanged {
			el.Log(Event{Type: EventTerm, Term: term, Leader: leaderIDString(r)})
			if term > prevTerm {
				el.Log(Event{Type: EventElection, Term: term})
			}
		}
	}
}

func leaderIDString(r *raft.Raft) string {
	_, id := r.LeaderWithID()
	return string(id)
}

func (el *EventLogger) pollCommits(r *raft.Raft) {
	var last uint64
	t := time.NewTicker(5 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-el.stop:
			return
		case <-t.C:
			idx := r.AppliedIndex()
			if idx > last {
				_, leaderID := r.LeaderWithID()
				el.Log(Event{
					Type:   EventCommit,
					Index:  idx,
					Term:   r.CurrentTerm(),
					Leader: string(leaderID),
				})
				last = idx
			}
		}
	}
}
