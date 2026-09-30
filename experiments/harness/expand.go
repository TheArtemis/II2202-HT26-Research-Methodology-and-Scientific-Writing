package harness

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/netinfra"
	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/raftnode"
)

// Trial is one fully-resolved experimental condition + repetition.
type Trial struct {
	Index      int                   `json:"index" yaml:"index"`
	RunID      string                `json:"run_id" yaml:"run_id"`
	Topology   string                `json:"topology" yaml:"topology"`
	Mode       netinfra.Mode         `json:"mode" yaml:"mode"`
	Delay      time.Duration         `json:"delay" yaml:"delay"`
	Heartbeat  time.Duration         `json:"heartbeat" yaml:"heartbeat"`
	Repetition int                   `json:"repetition" yaml:"repetition"`
	Seed       int64                 `json:"seed" yaml:"seed"`
	Spec       netinfra.TopologySpec `json:"-" yaml:"-"`
	Peers      map[string]string     `json:"peers" yaml:"peers"`
	Leader     string                `json:"leader" yaml:"leader"`
	ClientNode string                `json:"client_node" yaml:"client_node"`
	Dir        string                `json:"dir" yaml:"dir"`
}

type topoReady struct {
	spec       netinfra.TopologySpec
	peers      map[string]string
	leader     string
	clientNode string
}

// Expand builds the Cartesian product of matrix factors.
//
// Repetition is the outermost loop so contiguous --from/--limit shards (e.g. 180
// trials across 5 workers for a 900-trial full campaign) each cover all
// topology×mode×delay conditions. That avoids confounding worker identity with
// topology when sharding across EC2 instances.
//
// Seed = seeds.base + global trial index remains stable for a frozen config.
func Expand(exp Experiment) ([]Trial, error) {
	tops := make([]topoReady, 0, len(exp.Matrix.Topologies))
	for _, topoName := range exp.Matrix.Topologies {
		spec, err := netinfra.LoadNamedSpec(topoName)
		if err != nil {
			// Allow path.yaml as well.
			spec, err = loadTopoFlexible(topoName)
			if err != nil {
				return nil, err
			}
		}
		peers, err := raftnode.PeersFromSpec(spec)
		if err != nil {
			return nil, err
		}
		leader := spec.InitialLeader
		if leader == "" && len(spec.Nodes) > 0 {
			leader = spec.Nodes[0]
		}
		clientNode := exp.Workload.ClientNode
		if clientNode == "" {
			clientNode = leader
		}
		if _, ok := peers[clientNode]; !ok {
			return nil, fmt.Errorf("client_node %q not in topology %s", clientNode, spec.Name)
		}
		tops = append(tops, topoReady{
			spec:       spec,
			peers:      peers,
			leader:     leader,
			clientNode: clientNode,
		})
	}

	var trials []Trial
	idx := 0
	for rep := 1; rep <= exp.Matrix.Repetitions; rep++ {
		for _, t := range tops {
			for _, modeStr := range exp.Matrix.Modes {
				modeNorm, err := parseMode(modeStr)
				if err != nil {
					return nil, err
				}
				mode, err := netinfra.ParseMode(modeNorm)
				if err != nil {
					return nil, err
				}
				for _, delayStr := range exp.Matrix.Delays {
					delay, err := time.ParseDuration(delayStr)
					if err != nil {
						return nil, err
					}
					for _, hbStr := range exp.Matrix.Heartbeats {
						hb, err := time.ParseDuration(hbStr)
						if err != nil {
							return nil, err
						}
						runID := formatRunID(t.spec.Name, modeNorm, delay, hb, rep)
						trials = append(trials, Trial{
							Index:      idx,
							RunID:      runID,
							Topology:   t.spec.Name,
							Mode:       mode,
							Delay:      delay,
							Heartbeat:  hb,
							Repetition: rep,
							Seed:       exp.Seeds.Base + int64(idx),
							Spec:       t.spec,
							Peers:      t.peers,
							Leader:     t.leader,
							ClientNode: t.clientNode,
							Dir:        filepath.Join(exp.OutputDir, runID),
						})
						idx++
					}
				}
			}
		}
	}
	return trials, nil
}

func loadTopoFlexible(name string) (netinfra.TopologySpec, error) {
	if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") ||
		strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return netinfra.LoadSpecFile(name)
	}
	return netinfra.LoadNamedSpec(name)
}

func formatRunID(topo, mode string, delay, hb time.Duration, rep int) string {
	return fmt.Sprintf("%s_%s_d%s_hb%s_r%02d",
		strings.ToUpper(topo), mode, compactDur(delay), compactDur(hb), rep)
}

func compactDur(d time.Duration) string {
	s := d.String()
	s = strings.ReplaceAll(s, "µs", "us")
	return s
}
