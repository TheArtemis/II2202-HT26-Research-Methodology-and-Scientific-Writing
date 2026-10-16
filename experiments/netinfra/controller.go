package netinfra

import (
	"context"
	"time"
)

// Controller is the Go-facing control plane for a Mininet-backed experiment network.
// The harness depends only on this interface; it never imports Mininet.
type Controller interface {
	// Start builds the topology with all links up at the given per-link delay,
	// then applies mode (identity IPs, routes, ip_forward). Planned failures in
	// the YAML are not applied automatically — call InjectPlannedFailures or
	// SetLink after warm-up.
	Start(ctx context.Context, topo TopologySpec, mode Mode, delay time.Duration) error

	// SetLink fails or restores the bidirectional link between a and b.
	SetLink(a, b string, up bool) error

	// SetDelay reconfigures TCLink netem delay on both ends of every link.
	SetDelay(delay time.Duration) error

	// SetMode switches between Direct and Forwarding without rebuilding the net.
	SetMode(mode Mode) error

	// ExecOn runs a shell command inside the named Mininet host namespace.
	ExecOn(node string, cmd string) (stdout string, err error)

	// Validate checks reachability (and delay when possible) for the active
	// topology/mode against the research-plan QA gates.
	Validate(ctx context.Context) error

	// Close stops the Mininet network and best-effort cleans leftover state.
	Close() error

	// InjectPlannedFailures brings down every link marked failed in the YAML.
	InjectPlannedFailures() error

	// InjectAroundLeader remaps YAML failure marks so the cut pattern is
	// centered on actualLeader, brings those links down, and refreshes overlay
	// detours on the surviving full mesh.
	InjectAroundLeader(actualLeader string) error
}
