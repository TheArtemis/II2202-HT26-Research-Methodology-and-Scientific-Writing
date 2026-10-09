package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Experiment is the top-level YAML configuration for a batch of trials.
type Experiment struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	OutputDir   string `yaml:"output_dir"`

	Binaries BinariesConfig `yaml:"binaries"`
	Mininetd MininetdConfig `yaml:"mininetd"`
	Matrix   MatrixConfig   `yaml:"matrix"`
	Timing   TimingConfig   `yaml:"timing"`
	Workload WorkloadConfig `yaml:"workload"`
	Raft     RaftConfig     `yaml:"raft"`
	QA       QAConfig       `yaml:"qa"`
	Seeds    SeedsConfig    `yaml:"seeds"`
}

// BinariesConfig locates experiment binaries (absolute or relative to CWD).
type BinariesConfig struct {
	Raftd      string `yaml:"raftd"`
	Raftclient string `yaml:"raftclient"`
}

// MininetdConfig selects the JSON-RPC daemon address.
type MininetdConfig struct {
	Addr string `yaml:"addr"` // empty → netinfra defaults
}

// MatrixConfig is the independent-variable Cartesian product (research plan).
type MatrixConfig struct {
	Topologies  []string `yaml:"topologies"`
	Modes       []string `yaml:"modes"`
	Delays      []string `yaml:"delays"`
	Heartbeats  []string `yaml:"heartbeats"`
	Repetitions int      `yaml:"repetitions"`
}

// TimingConfig controls the per-trial schedule.
type TimingConfig struct {
	ClusterReadyTimeout Duration `yaml:"cluster_ready_timeout"`
	Settle              Duration `yaml:"settle"`
	Warmup              Duration `yaml:"warmup"`
	Observe             Duration `yaml:"observe"`
	TeardownPause       Duration `yaml:"teardown_pause"`
}

// WorkloadConfig is the open-loop client profile.
type WorkloadConfig struct {
	Rate          float64  `yaml:"rate"`
	PayloadSize   int      `yaml:"payload_size"`
	ApplyTimeout  Duration `yaml:"apply_timeout"`
	ClientNode    string   `yaml:"client_node"` // empty → topology initial_leader
}

// RaftConfig tunes replica startup.
type RaftConfig struct {
	RaftPort int  `yaml:"raft_port"`
	APIPort  int  `yaml:"api_port"`
	SeedT3   bool `yaml:"seed_t3"`
}

// QAConfig gates trial acceptance.
type QAConfig struct {
	ValidateAfterInject  bool    `yaml:"validate_after_inject"`
	RequireWarmupCommits bool    `yaml:"require_warmup_commits"`
	MinWarmupOK          int     `yaml:"min_warmup_ok"`
	ContinueOnError      bool    `yaml:"continue_on_error"`
	HostLoadMax1m        float64 `yaml:"host_load_max_1m"` // 0 = disabled; marks host_overloaded in metrics
	HostLoadInterval     Duration `yaml:"host_load_interval"`
}

// SeedsConfig records reproducible seeds (seed = base + trial index).
type SeedsConfig struct {
	Base int64 `yaml:"base"`
}

// Duration wraps time.Duration with YAML string support ("5ms", "2s").
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		var ms float64
		if err2 := n.Decode(&ms); err2 != nil {
			return fmt.Errorf("duration: %w", err)
		}
		d.Duration = time.Duration(ms * float64(time.Millisecond))
		return nil
	}
	s = strings.TrimSpace(s)
	if s == "" {
		d.Duration = 0
		return nil
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

// LoadExperimentFile reads and validates an experiment YAML file.
func LoadExperimentFile(path string) (Experiment, error) {
	f, err := os.Open(path)
	if err != nil {
		return Experiment{}, err
	}
	defer f.Close()
	var exp Experiment
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&exp); err != nil {
		return Experiment{}, fmt.Errorf("decode %s: %w", path, err)
	}
	exp.applyDefaults()
	if err := exp.Validate(); err != nil {
		return Experiment{}, err
	}
	if err := exp.resolveBinaries(); err != nil {
		return Experiment{}, err
	}
	return exp, nil
}

func (e *Experiment) applyDefaults() {
	if e.OutputDir == "" {
		e.OutputDir = filepath.Join("results", e.Name)
	}
	if e.Binaries.Raftd == "" {
		e.Binaries.Raftd = "./bin/raftd"
	}
	if e.Binaries.Raftclient == "" {
		e.Binaries.Raftclient = "./bin/raftclient"
	}
	if e.Matrix.Repetitions <= 0 {
		e.Matrix.Repetitions = 1
	}
	if e.Timing.ClusterReadyTimeout.Duration == 0 {
		e.Timing.ClusterReadyTimeout.Duration = 20 * time.Second
	}
	if e.Timing.Settle.Duration == 0 {
		e.Timing.Settle.Duration = 1 * time.Second
	}
	// Warmup may be 0s to skip pre-inject client load (T3 keeps divergent seeds).
	if e.Timing.Observe.Duration == 0 {
		e.Timing.Observe.Duration = 20 * time.Second
	}
	if e.Timing.TeardownPause.Duration == 0 {
		e.Timing.TeardownPause.Duration = 500 * time.Millisecond
	}
	if e.Workload.Rate <= 0 {
		e.Workload.Rate = 100
	}
	if e.Workload.PayloadSize < 0 {
		e.Workload.PayloadSize = 64
	}
	if e.Workload.PayloadSize == 0 {
		e.Workload.PayloadSize = 64
	}
	if e.Workload.ApplyTimeout.Duration == 0 {
		e.Workload.ApplyTimeout.Duration = 2 * time.Second
	}
	if e.Raft.RaftPort == 0 {
		e.Raft.RaftPort = 7000
	}
	if e.Raft.APIPort == 0 {
		e.Raft.APIPort = 7001
	}
	if e.QA.MinWarmupOK == 0 && e.QA.RequireWarmupCommits {
		e.QA.MinWarmupOK = 5
	}
	if e.QA.MinWarmupOK == 0 {
		e.QA.MinWarmupOK = 5
	}
	if e.QA.HostLoadInterval.Duration == 0 {
		e.QA.HostLoadInterval.Duration = time.Second
	}
}

func (e *Experiment) resolveBinaries() error {
	for _, pair := range []struct {
		name string
		path *string
	}{
		{"raftd", &e.Binaries.Raftd},
		{"raftclient", &e.Binaries.Raftclient},
	} {
		abs, err := filepath.Abs(*pair.path)
		if err != nil {
			return fmt.Errorf("%s path: %w", pair.name, err)
		}
		*pair.path = abs
	}
	return nil
}

// Validate checks structural consistency.
func (e Experiment) Validate() error {
	if strings.TrimSpace(e.Name) == "" {
		return fmt.Errorf("experiment name is required")
	}
	if len(e.Matrix.Topologies) == 0 {
		return fmt.Errorf("matrix.topologies must be non-empty")
	}
	if len(e.Matrix.Modes) == 0 {
		return fmt.Errorf("matrix.modes must be non-empty")
	}
	if len(e.Matrix.Delays) == 0 {
		return fmt.Errorf("matrix.delays must be non-empty")
	}
	if len(e.Matrix.Heartbeats) == 0 {
		return fmt.Errorf("matrix.heartbeats must be non-empty")
	}
	if e.Matrix.Repetitions < 1 {
		return fmt.Errorf("matrix.repetitions must be >= 1")
	}
	for _, m := range e.Matrix.Modes {
		if _, err := parseMode(m); err != nil {
			return err
		}
	}
	for _, d := range e.Matrix.Delays {
		if _, err := time.ParseDuration(d); err != nil {
			return fmt.Errorf("matrix.delays %q: %w", d, err)
		}
	}
	for _, h := range e.Matrix.Heartbeats {
		if _, err := time.ParseDuration(h); err != nil {
			return fmt.Errorf("matrix.heartbeats %q: %w", h, err)
		}
	}
	return nil
}

func parseMode(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "direct", "direct-only", "direct_only", "p2p":
		return "direct", nil
	case "forwarding", "forward", "overlay", "multi-hop", "multihop":
		return "forwarding", nil
	default:
		return "", fmt.Errorf("unknown mode %q", s)
	}
}
