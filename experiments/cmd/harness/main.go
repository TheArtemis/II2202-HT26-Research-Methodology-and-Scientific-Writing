package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/collect"
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
	case "summarize":
		err = cmdSummarize(args)
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
  harness run [flags] <config.yaml>       # execute trials (requires mininetd)
  harness summarize <results/dir>         # join runs → dataset.jsonl + dataset.csv

Flags for run (before or after the config path):
  --from N     skip first N trials (0-based index)
  --limit N    run at most N trials (0 = all)
  --addr ADDR  mininetd unix socket or host:port

Flags for summarize:
  --host-load-max F   mark host_overloaded when load1 exceeds F (0=off)

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
  ./bin/harness summarize results/smoke
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
	// Go's flag package stops at the first non-flag arg, so
	// `run config.yaml --from 113` previously ignored --from/--limit (all
	// workers then ran the full matrix from index 0). Reorder so flags work
	// before or after the config path.
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: harness run [--from N] [--limit N] <config.yaml>")
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

func cmdSummarize(args []string) error {
	fs := flag.NewFlagSet("summarize", flag.ContinueOnError)
	hostMax := fs.Float64("host-load-max", 0, "overload threshold for load1 (0=off)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: harness summarize <results/dir>")
	}
	dir := fs.Arg(0)
	n, err := collect.JoinDataset(dir, *hostMax)
	if err != nil {
		return err
	}
	fmt.Printf("summarize: %d runs → %s/dataset.jsonl + dataset.csv\n", n, dir)
	return nil
}

func parseConfigArgs(args []string) (path string, rest []string, err error) {
	if len(args) < 1 {
		return "", nil, fmt.Errorf("config yaml path required")
	}
	return args[0], args[1:], nil
}

// reorderFlags moves dash-flags (and their values) before positional args so
// Go's flag.FlagSet can parse them when callers pass `config.yaml --from N`.
func reorderFlags(args []string) []string {
	valueFlags := map[string]bool{
		"-from": true, "--from": true,
		"-limit": true, "--limit": true,
		"-addr": true, "--addr": true,
	}
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			name, _, hasEq := strings.Cut(a, "=")
			if hasEq {
				continue
			}
			if valueFlags[name] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		pos = append(pos, a)
	}
	return append(flags, pos...)
}
