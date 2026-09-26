package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/raftnode"
)

func main() {
	id := flag.String("id", "", "Raft server ID (node name, e.g. C)")
	bind := flag.String("bind", "", "identity IP to bind (e.g. 10.0.0.3)")
	peers := flag.String("peers", "", "peer map A=10.0.0.1,B=10.0.0.2,...")
	raftPort := flag.Int("raft-port", raftnode.DefaultRaftPort, "Raft RPC port")
	apiPort := flag.Int("api-port", raftnode.DefaultAPIPort, "HTTP Apply API port")
	heartbeat := flag.Duration("heartbeat", raftnode.DefaultHeartbeat, "Raft heartbeat / lease interval")
	election := flag.Duration("election", 0, "election timeout (default 5× heartbeat, min 50ms)")
	bootstrap := flag.Bool("bootstrap", true, "bootstrap cluster config when stores are empty")
	prefer := flag.Bool("prefer-leader", false, "shorten election timeout to win warm-up leadership")
	dataDir := flag.String("data-dir", "", "optional data dir (default: in-memory stores)")
	events := flag.String("events", "", "JSONL event log path (default: stderr)")
	seedLog := flag.String("seed-log", "", "optional T3 seed terms, e.g. 1,1,2,2")
	flag.Parse()

	if *id == "" || *bind == "" || *peers == "" {
		fmt.Fprintf(os.Stderr, "usage: raftd -id C -bind 10.0.0.3 -peers A=10.0.0.1,B=...,C=10.0.0.3 [flags]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	peerMap, err := raftnode.ParsePeers(*peers)
	if err != nil {
		fatal(err)
	}
	seed, err := raftnode.ParseSeedLog(*seedLog)
	if err != nil {
		fatal(err)
	}

	cfg := raftnode.Config{
		ID:              *id,
		BindIP:          *bind,
		RaftPort:        *raftPort,
		APIPort:         *apiPort,
		Peers:           peerMap,
		Heartbeat:       *heartbeat,
		ElectionTimeout: *election,
		PreferLeader:    *prefer,
		Bootstrap:       *bootstrap,
		DataDir:         *dataDir,
		EventsPath:      *events,
		SeedLogTerms:    seed,
	}

	n, err := raftnode.Open(cfg)
	if err != nil {
		fatal(err)
	}
	defer n.Close()

	fmt.Fprintf(os.Stderr, "raftd id=%s raft=%s api=%s heartbeat=%s prefer_leader=%v\n",
		cfg.ID, cfg.RaftAddr(), cfg.APIAddr(), cfg.Heartbeat, cfg.PreferLeader)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Fprintf(os.Stderr, "raftd %s shutting down\n", cfg.ID)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "raftd: %v\n", err)
	os.Exit(1)
}