package raftnode_test

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/raftnode"
	"github.com/hashicorp/raft"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func TestSingleNodeApply(t *testing.T) {
	raftPort := freePort(t)
	apiPort := freePort(t)
	n, err := raftnode.Open(raftnode.Config{
		ID:           "A",
		BindIP:       "127.0.0.1",
		RaftPort:     raftPort,
		APIPort:      apiPort,
		Peers:        map[string]string{"A": "127.0.0.1"},
		Heartbeat:    50 * time.Millisecond,
		Bootstrap:    true,
		PreferLeader: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	if err := n.WaitForLeader(5 * time.Second); err != nil {
		t.Fatal(err)
	}
	if n.Raft.State() != raft.Leader {
		t.Fatalf("state %s", n.Raft.State())
	}

	res, err := n.Apply(raftnode.Request{ID: "r1", Payload: []byte("hi")}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "r1" || res.Index == 0 {
		t.Fatalf("%+v", res)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/stats", apiPort)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("stats %d", resp.StatusCode)
	}
}

func TestPreVoteWithoutQuorum(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("disabled=%v", disabled), func(t *testing.T) {
			n, err := raftnode.Open(raftnode.Config{
				ID:              "A",
				BindIP:          "127.0.0.1",
				RaftPort:        freePort(t),
				APIPort:         freePort(t),
				Peers:           map[string]string{"A": "127.0.0.1", "B": "[::1]"},
				Heartbeat:       20 * time.Millisecond,
				Bootstrap:       true,
				PreVoteDisabled: disabled,
				EventsPath:      filepath.Join(t.TempDir(), "events.jsonl"),
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { n.Close() })

			deadline := time.Now().Add(3 * time.Second)
			for n.Raft.State() != raft.Candidate {
				if time.Now().After(deadline) {
					t.Fatal("isolated node did not start an election campaign")
				}
				time.Sleep(5 * time.Millisecond)
			}
			// An isolated replica cannot win pre-vote. Its term must stay at the
			// bootstrap term (1), unless pre-vote is disabled and it votes directly.
			time.Sleep(600 * time.Millisecond)
			term := n.Raft.Stats()["term"]
			if disabled && term == "1" {
				t.Fatal("term did not increase with pre-vote disabled")
			}
			if !disabled && term != "1" {
				t.Fatalf("term increased to %s with pre-vote enabled and no quorum", term)
			}
		})
	}
}
