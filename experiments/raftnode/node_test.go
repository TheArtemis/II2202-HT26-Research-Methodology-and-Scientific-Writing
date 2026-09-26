package raftnode_test

import (
	"fmt"
	"net"
	"net/http"
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
