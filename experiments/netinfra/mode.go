package netinfra

import (
	"fmt"
	"strings"
)

// Mode selects how identity addresses are reached between Mininet hosts.
type Mode int

const (
	// Direct allows only on-link neighbors (ip_forward=0, no multi-hop routes).
	Direct Mode = iota
	// Forwarding enables ip_forward and installs shortest-path overlay detours
	// around down links on the full-mesh underlay (simplified NIFTY-style).
	Forwarding
)

func (m Mode) String() string {
	switch m {
	case Direct:
		return "direct"
	case Forwarding:
		return "forwarding"
	default:
		return fmt.Sprintf("Mode(%d)", int(m))
	}
}

// ParseMode accepts "direct" / "forwarding" (case-insensitive).
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "direct", "direct-only", "direct_only":
		return Direct, nil
	case "forwarding", "forward", "overlay":
		return Forwarding, nil
	default:
		return 0, fmt.Errorf("unknown mode %q (want direct or forwarding)", s)
	}
}
