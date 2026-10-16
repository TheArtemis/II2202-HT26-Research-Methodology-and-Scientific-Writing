package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

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
	case "up":
		err = cmdUp(args)
	case "down":
		err = cmdDown(args)
	case "link":
		err = cmdLink(args)
	case "delay":
		err = cmdDelay(args)
	case "mode":
		err = cmdMode(args)
	case "exec":
		err = cmdExec(args)
	case "validate":
		err = cmdValidate(args)
	case "inject":
		err = cmdInject(args)
	case "list":
		err = cmdList(args)
	case "peers":
		err = cmdPeers(args)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "netctl %s: %v\n", cmd, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `netctl — Go control CLI for Mininet netinfra

Usage:
  netctl up <t0|t1|t2|t3|t4|path.yaml> [--mode direct|forwarding] [--delay 5ms] [--inject] [--addr ADDR]
  netctl inject                          # bring down YAML planned failures
  netctl link up|down <A> <B>
  netctl delay <5ms>
  netctl mode direct|forwarding
  netctl exec <node> -- <command...>
  netctl validate
  netctl down                            # Stop + mn -c cleanup
  netctl list                            # embedded topology names
  netctl peers <t0|…|t4>                 # print Raft -peers A=10.0.0.1,... string

Environment:
  MININETD_ADDR   unix socket path or host:port (default /tmp/mininetd.sock)

Typical harness sequence:
  sudo python3 experiments/netinfra/mininetd/server.py &
  netctl up t4 --mode forwarding --delay 5ms
  PEERS=$(netctl peers t4)
  # start raftd on each host (prefer initial_leader), then open-loop raftclient
  netctl inject
  netctl validate
  netctl down
`)
}

func newClient(addr string) *netinfra.Client {
	return netinfra.NewClient(netinfra.ClientOptions{Addr: addr})
}

func cmdUp(args []string) error {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	modeStr := fs.String("mode", "direct", "direct or forwarding")
	delayStr := fs.String("delay", "5ms", "per-link TCLink delay")
	inject := fs.Bool("inject", false, "apply YAML planned failures after Start")
	addr := fs.String("addr", "", "mininetd address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: netctl up <topo> [--mode ...] [--delay ...] [--inject]")
	}
	topoName := fs.Arg(0)
	mode, err := netinfra.ParseMode(*modeStr)
	if err != nil {
		return err
	}
	delay, err := time.ParseDuration(*delayStr)
	if err != nil {
		return fmt.Errorf("delay: %w", err)
	}
	spec, err := loadTopo(topoName)
	if err != nil {
		return err
	}
	c := newClient(*addr)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := c.Start(ctx, spec, mode, delay); err != nil {
		return err
	}
	fmt.Printf("started %s mode=%s delay=%s links=%d (all up)\n",
		spec.Name, mode, delay, len(spec.Links))
	if *inject {
		if err := c.InjectPlannedFailures(); err != nil {
			return err
		}
		fmt.Printf("injected %d planned failure(s)\n", len(spec.PlannedFailures()))
	}
	return nil
}

func loadTopo(name string) (netinfra.TopologySpec, error) {
	if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") ||
		strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return netinfra.LoadSpecFile(name)
	}
	return netinfra.LoadNamedSpec(name)
}

func cmdDown(args []string) error {
	fs := flag.NewFlagSet("down", flag.ContinueOnError)
	addr := fs.String("addr", "", "mininetd address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c := newClient(*addr)
	if err := c.Close(); err != nil {
		return err
	}
	fmt.Println("stopped")
	return nil
}

func cmdLink(args []string) error {
	fs := flag.NewFlagSet("link", flag.ContinueOnError)
	addr := fs.String("addr", "", "mininetd address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 3 {
		return fmt.Errorf("usage: netctl link up|down <A> <B>")
	}
	action, a, b := rest[0], rest[1], rest[2]
	up := action == "up"
	if action != "up" && action != "down" {
		return fmt.Errorf("usage: netctl link up|down <A> <B>")
	}
	c := newClient(*addr)
	_ = c.LoadSession() // best-effort so down-set persists
	if err := c.SetLink(a, b, up); err != nil {
		return err
	}
	fmt.Printf("link %s--%s %s\n", a, b, action)
	return nil
}

func cmdDelay(args []string) error {
	fs := flag.NewFlagSet("delay", flag.ContinueOnError)
	addr := fs.String("addr", "", "mininetd address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: netctl delay <duration>")
	}
	d, err := time.ParseDuration(fs.Arg(0))
	if err != nil {
		return err
	}
	c := newClient(*addr)
	_ = c.LoadSession()
	if err := c.SetDelay(d); err != nil {
		return err
	}
	fmt.Printf("delay %s\n", d)
	return nil
}

func cmdMode(args []string) error {
	fs := flag.NewFlagSet("mode", flag.ContinueOnError)
	addr := fs.String("addr", "", "mininetd address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: netctl mode direct|forwarding")
	}
	mode, err := netinfra.ParseMode(fs.Arg(0))
	if err != nil {
		return err
	}
	c := newClient(*addr)
	_ = c.LoadSession()
	if err := c.SetMode(mode); err != nil {
		return err
	}
	fmt.Printf("mode %s\n", mode)
	return nil
}

func cmdExec(args []string) error {
	fs := flag.NewFlagSet("exec", flag.ContinueOnError)
	addr := fs.String("addr", "", "mininetd address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) < 2 {
		return fmt.Errorf("usage: netctl exec <node> -- <command...>")
	}
	node := rest[0]
	cmdParts := rest[1:]
	if cmdParts[0] == "--" {
		cmdParts = cmdParts[1:]
	}
	if len(cmdParts) == 0 {
		return fmt.Errorf("missing command")
	}
	c := newClient(*addr)
	out, err := c.ExecOn(node, strings.Join(cmdParts, " "))
	fmt.Print(out)
	return err
}

func cmdValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	addr := fs.String("addr", "", "mininetd address")
	topoName := fs.String("topo", "", "override topology (default: session from netctl up)")
	modeStr := fs.String("mode", "", "override mode")
	delayStr := fs.String("delay", "", "override delay")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c := newClient(*addr)
	if *topoName != "" {
		spec, err := loadTopo(*topoName)
		if err != nil {
			return err
		}
		mode := netinfra.Direct
		if *modeStr != "" {
			mode, err = netinfra.ParseMode(*modeStr)
			if err != nil {
				return err
			}
		} else if err := c.LoadSession(); err == nil {
			mode = c.ActiveMode()
		}
		delay := c.ActiveDelay()
		if *delayStr != "" {
			delay, err = time.ParseDuration(*delayStr)
			if err != nil {
				return err
			}
		} else if err := c.LoadSession(); err == nil {
			delay = c.ActiveDelay()
		}
		c.AttachSession(spec, mode, delay)
	} else if err := c.LoadSession(); err != nil {
		return fmt.Errorf("validate: %w (or pass --topo)", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := c.Validate(ctx); err != nil {
		return err
	}
	fmt.Println("validate: ok")
	return nil
}

func cmdInject(args []string) error {
	fs := flag.NewFlagSet("inject", flag.ContinueOnError)
	addr := fs.String("addr", "", "mininetd address")
	topoName := fs.String("topo", "", "override topology (default: session)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c := newClient(*addr)
	if *topoName != "" {
		spec, err := loadTopo(*topoName)
		if err != nil {
			return err
		}
		c.AttachSession(spec, netinfra.Direct, 0)
	} else if err := c.LoadSession(); err != nil {
		return fmt.Errorf("inject: %w (or pass --topo)", err)
	}
	if err := c.InjectPlannedFailures(); err != nil {
		return err
	}
	fmt.Printf("injected %d planned failure(s)\n", len(c.ActiveTopo().PlannedFailures()))
	return nil
}

func cmdList(args []string) error {
	names, err := netinfra.ListEmbeddedSpecs()
	if err != nil {
		return err
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return nil
}

func cmdPeers(args []string) error {
	fs := flag.NewFlagSet("peers", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: netctl peers <t0|t1|t2|t3|t4|path.yaml>")
	}
	spec, err := loadTopo(fs.Arg(0))
	if err != nil {
		return err
	}
	m, err := spec.IdentityMap()
	if err != nil {
		return err
	}
	// Stable ID=IP,... for raftd/raftclient -peers
	ids := append([]string(nil), spec.Nodes...)
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%s=%s", id, m[id]))
	}
	fmt.Println(strings.Join(parts, ","))
	if spec.InitialLeader != "" {
		fmt.Fprintf(os.Stderr, "# initial_leader=%s\n", spec.InitialLeader)
	}
	return nil
}
