package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/raftclient"
	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/raftnode"
)

func main() {
	peers := flag.String("peers", "", "peer map A=10.0.0.1,B=10.0.0.2 (identity IPs)")
	apiPort := flag.Int("api-port", raftnode.DefaultAPIPort, "HTTP API port on each peer")
	urls := flag.String("urls", "", "optional explicit HTTP bases (overrides -peers), comma-separated")
	rate := flag.Float64("rate", 100, "open-loop request rate (req/s)")
	duration := flag.Duration("duration", 30*time.Second, "how long to keep submitting (0 = until signal)")
	size := flag.Int("size", 64, "payload bytes per request")
	timeout := flag.Duration("timeout", 2*time.Second, "per-request HTTP timeout")
	out := flag.String("out", "", "JSONL metrics path (default: stdout)")
	leader := flag.String("leader", "", "optional initial leader ID for sticky routing")
	flag.Parse()

	var peerURLs []string
	var peerIDs []string
	switch {
	case *urls != "":
		for _, u := range strings.Split(*urls, ",") {
			u = strings.TrimSpace(u)
			if u != "" {
				peerURLs = append(peerURLs, strings.TrimRight(u, "/"))
			}
		}
	case *peers != "":
		m, err := raftnode.ParsePeers(*peers)
		if err != nil {
			fatal(err)
		}
		// Stable order by FormatPeers then re-parse order via sorted FormatPeers IDs.
		formatted := raftnode.FormatPeers(m)
		for _, part := range strings.Split(formatted, ",") {
			kv := strings.SplitN(part, "=", 2)
			id, ip := kv[0], kv[1]
			peerIDs = append(peerIDs, id)
			peerURLs = append(peerURLs, fmt.Sprintf("http://%s:%d", ip, *apiPort))
		}
	default:
		fmt.Fprintf(os.Stderr, "usage: raftclient -peers A=10.0.0.1,B=... [flags]\n")
		flag.PrintDefaults()
		os.Exit(2)
	}

	c, err := raftclient.New(raftclient.Config{
		Peers:       peerURLs,
		PeerIDs:     peerIDs,
		Rate:        *rate,
		Duration:    *duration,
		PayloadSize: *size,
		Timeout:     *timeout,
		OutPath:     *out,
		LeaderID:    *leader,
	})
	if err != nil {
		fatal(err)
	}
	defer c.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Fprintf(os.Stderr, "raftclient rate=%.1f/s duration=%s peers=%d out=%q\n",
		*rate, *duration, len(peerURLs), *out)

	if err := c.Run(ctx); err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		fatal(err)
	}
	sent, ok, failed := c.Stats()
	fmt.Fprintf(os.Stderr, "raftclient done sent=%d ok=%d failed=%d\n", sent, ok, failed)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "raftclient: %v\n", err)
	os.Exit(1)
}
