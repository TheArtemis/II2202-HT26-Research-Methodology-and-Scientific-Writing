package raftclient_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/raftclient"
	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/raftnode"
)

func TestOpenLoopAgainstSingleNode(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	raftPort := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	apiPort := ln2.Addr().(*net.TCPAddr).Port
	_ = ln2.Close()

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

	out := filepath.Join(t.TempDir(), "client.jsonl")
	base := fmt.Sprintf("http://127.0.0.1:%d", apiPort)
	c, err := raftclient.New(raftclient.Config{
		Peers:       []string{base},
		PeerIDs:     []string{"A"},
		Rate:        50,
		Duration:    200 * time.Millisecond,
		PayloadSize: 8,
		Timeout:     time.Second,
		OutPath:     out,
		LeaderID:    "A",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if err := c.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	sent, ok, failed := c.Stats()
	if sent == 0 || ok == 0 {
		t.Fatalf("sent=%d ok=%d failed=%d", sent, ok, failed)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	line := raw
	for i, b := range raw {
		if b == '\n' {
			line = raw[:i]
			break
		}
	}
	var rec raftclient.Record
	if err := json.Unmarshal(line, &rec); err != nil {
		t.Fatal(err)
	}
	if !rec.OK || rec.LatencyNS <= 0 {
		t.Fatalf("%+v", rec)
	}

	resp, err := http.Get(base + "/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}
