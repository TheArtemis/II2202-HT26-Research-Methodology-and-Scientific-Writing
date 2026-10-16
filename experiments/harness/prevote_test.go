package harness

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/netinfra"
	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/raftnode"
	"gopkg.in/yaml.v3"
)

func TestExperimentPreVote(t *testing.T) {
	for _, tc := range []struct {
		name     string
		setting  string
		disabled bool
	}{
		{name: "omitted"},
		{name: "enabled", setting: "raft:\n  pre_vote_disabled: false\n"},
		{name: "disabled", setting: "raft:\n  pre_vote_disabled: true\n", disabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := `name: prevote
matrix:
  topologies: [t3]
  modes: [direct]
  delays: [5ms]
  heartbeats: [10ms]
` + tc.setting
			path := filepath.Join(t.TempDir(), "experiment.yaml")
			if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
				t.Fatal(err)
			}
			exp, err := LoadExperimentFile(path)
			if err != nil {
				t.Fatal(err)
			}
			trials, err := Expand(exp)
			if err != nil {
				t.Fatal(err)
			}
			trial := trials[0]
			client, commands := captureReplicaCommands(t, 2*len(trial.Spec.Nodes))
			runner := Runner{Exp: exp, Client: client}
			wantFlag := fmt.Sprintf(" -pre-vote-disabled=%t ", tc.disabled)
			for _, opts := range []replicaStartOpts{{}, {SeedLogs: raftnode.T3SeedLogs}} {
				if err := runner.startReplicas(trial, opts); err != nil {
					t.Fatal(err)
				}
				for range trial.Spec.Nodes {
					if cmd := <-commands; !strings.Contains(cmd, wantFlag) {
						t.Fatalf("startup command missing %s: %s", wantFlag, cmd)
					}
				}
			}

			data, err := yaml.Marshal(trialMeta(trial, exp))
			if err != nil {
				t.Fatal(err)
			}
			wantSetting := fmt.Sprintf("pre_vote_disabled: %t", tc.disabled)
			if !strings.Contains(string(data), wantSetting) {
				t.Fatalf("trial metadata missing %s:\n%s", wantSetting, data)
			}
		})
	}
}

func captureReplicaCommands(t *testing.T, count int) (*netinfra.Client, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	commands := make(chan string, count)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		decoder := json.NewDecoder(conn)
		for {
			var req struct {
				ID     uint64 `json:"id"`
				Params struct {
					Cmd string `json:"cmd"`
				} `json:"params"`
			}
			if err := decoder.Decode(&req); err != nil {
				return
			}
			if req.Params.Cmd != "" {
				commands <- req.Params.Cmd
			}
			if _, err := fmt.Fprintf(conn, `{"jsonrpc":"2.0","id":%d,"result":{"stdout":"","code":0}}`+"\n", req.ID); err != nil {
				return
			}
		}
	}()
	client := netinfra.NewClient(netinfra.ClientOptions{Addr: listener.Addr().String()})
	t.Setenv("NETINFRA_SESSION", filepath.Join(t.TempDir(), "session.json"))
	t.Cleanup(func() { client.Close() })
	return client, commands
}
