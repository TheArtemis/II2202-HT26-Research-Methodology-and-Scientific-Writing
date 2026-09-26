package netinfra

// Package netinfra is the Go control plane for Mininet-backed experiment networks.
//
// A thin Python daemon (mininetd) owns the Mininet 2.3.0 object. This package
// loads T0–T4 YAML specs, speaks JSON-RPC to the daemon, and exposes Controller
// for the experiment harness (Start → inject failures → Close).
