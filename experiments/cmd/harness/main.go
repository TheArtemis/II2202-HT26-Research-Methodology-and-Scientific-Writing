package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/harness"
	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/netinfra"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	var err error
	switch cmd {
	case "list":
		err = cmdList(args)
	case "dry-run":
		err = cmdDryRun(args)
	case "run":
		err = cmdRun(args)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "harness %s: %v\n", cmd, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `harness — YAML-driven Raft × Mininet experiment runner

Usage:
  harness list <config.yaml>              # print expanded trial run IDs
  harness dry-run <config.yaml>           # validate + expand, no Mininet
  harness run <config.yaml> [flags]       # execute trials (requires mininetd)

Flags for run:
  --from N     skip first N trials (0-based index)
  --limit N    run at most N trials (0 = all)
  --addr ADDR  mininetd unix socket or host:port

Configs:
  configs/smoke.yaml            # T4 × 2 modes × 1 rep
  configs/pilot.yaml            # Stage 3: 150 runs (5 reps)
  configs/full.yaml             # Stage 4: 600 runs (20 reps)
  configs/heartbeat-sweep.yaml  # optional heartbeat IV

Typical pilot sequence (Linux VM):
  sudo python3 netinfra/mininetd/server.py --socket /tmp/mininetd.sock &
  go build -o bin/raftd ./cmd/raftd
  go build -o bin/raftclient ./cmd/raftclient
  go build -o bin/harness ./cmd/harness
  ./bin/harness dry-run configs/smoke.yaml
  ./bin/harness run configs/smoke.yaml
  ./bin/harness run configs/pilot.yaml
`)
}

func cmdList(args []string) error {
	path, _, err := parseConfigArgs(args)
	if err != nil {
		return err
	}
	exp, err := harness.LoadExperimentFile(path)
	if err != nil {
		return err
	}
	trials, err := harness.Expand(exp)
	if err != nil {
		return err
	}
	fmt.Printf("# %s: %d trials → %s\n", exp.Name, len(trials), exp.OutputDir)
	for _, t := range trials {
		fmt.Printf("%4d  %s\n", t.Index, t.RunID)
	}
	return nil
}

func cmdDryRun(args []string) error {
	path, _, err := parseConfigArgs(args)
	if err != nil {
		return err
	}
	exp, err := harness.LoadExperimentFile(path)
	if err != nil {
		return err
	}
	trials, err := harness.Expand(exp)
	if err != nil {
		return err
	}
	fmt.Printf("experiment: %s\n", exp.Name)
	fmt.Printf("description: %s\n", exp.Description)
	fmt.Printf("output_dir: %s\n", exp.OutputDir)
	fmt.Printf("raftd: %s\n", exp.Binaries.Raftd)
	fmt.Printf("raftclient: %s\n", exp.Binaries.Raftclient)
	fmt.Printf("trials: %d\n", len(trials))
	fmt.Printf("matrix: topologies=%v modes=%v delays=%v heartbeats=%v reps=%d\n",
		exp.Matrix.Topologies, exp.Matrix.Modes, exp.Matrix.Delays, exp.Matrix.Heartbeats, exp.Matrix.Repetitions)
	fmt.Printf("timing: settle=%s warmup=%s observe=%s\n",
		exp.Timing.Settle.Duration, exp.Timing.Warmup.Duration, exp.Timing.Observe.Duration)
	fmt.Printf("workload: rate=%.1f size=%d\n", exp.Workload.Rate, exp.Workload.PayloadSize)
	if len(trials) > 0 {
		fmt.Printf("first: %s\n", trials[0].RunID)
		fmt.Printf("last:  %s\n", trials[len(trials)-1].RunID)
	}
	fmt.Println("dry-run: ok")
	return nil
}

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	from := fs.Int("from", 0, "skip first N trials")
	limit := fs.Int("limit", 0, "max trials to run (0=all)")
	addr := fs.String("addr", "", "mininetd address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: harness run <config.yaml> [--from N] [--limit N]")
	}
	path := fs.Arg(0)
	exp, err := harness.LoadExperimentFile(path)
	if err != nil {
		return err
	}
	if *addr != "" {
		exp.Mininetd.Addr = *addr
	}
	trials, err := harness.Expand(exp)
	if err != nil {
		return err
	}
	if *from < 0 || *from > len(trials) {
		return fmt.Errorf("--from %d out of range [0,%d]", *from, len(trials))
	}
	trials = trials[*from:]
	if *limit > 0 && *limit < len(trials) {
		trials = trials[:*limit]
	}
	if len(trials) == 0 {
		return fmt.Errorf("no trials to run")
	}

	if _, err := os.Stat(exp.Binaries.Raftd); err != nil {
		return fmt.Errorf("raftd binary: %w (build with: go build -o bin/raftd ./cmd/raftd)", err)
	}
	if _, err := os.Stat(exp.Binaries.Raftclient); err != nil {
		return fmt.Errorf("raftclient binary: %w (build with: go build -o bin/raftclient ./cmd/raftclient)", err)
	}

	c := netinfra.NewClient(netinfra.ClientOptions{Addr: exp.Mininetd.Addr})
	runner := &harness.Runner{
		Exp:    exp,
		Client: c,
		Logf: func(format string, a ...interface{}) {
			fmt.Fprintf(os.Stderr, format+"\n", a...)
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(os.Stderr, "harness run %s: %d trials → %s\n", exp.Name, len(trials), exp.OutputDir)
	return runner.RunAll(ctx, trials)
}

func parseConfigArgs(args []string) (path string, rest []string, err error) {
	if len(args) < 1 {
		return "", nil, fmt.Errorf("config yaml path required")
	}
	return args[0], args[1:], nil
}
