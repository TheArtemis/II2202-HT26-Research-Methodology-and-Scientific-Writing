package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/collect"
	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/netinfra"
	"github.com/TheArtemis/II2202-HT26-Research-Methodology-and-Scientific-Writing/experiments/raftnode"
	"gopkg.in/yaml.v3"
)

// Runner executes trials against a live mininetd + shared-filesystem binaries.
type Runner struct {
	Exp    Experiment
	Client *netinfra.Client
	Logf   func(format string, args ...interface{})
}

// TrialStatus is written to each run directory after completion.
type TrialStatus struct {
	RunID      string    `json:"run_id"`
	OK         bool      `json:"ok"`
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	InjectAt   time.Time `json:"inject_at,omitempty"`
	InjectAtNS int64     `json:"inject_at_ns,omitempty"`
	WarmupOK   int       `json:"warmup_ok_commits,omitempty"`
	Leader     string    `json:"leader,omitempty"`
	Seed       int64     `json:"seed"`
	Topology   string    `json:"topology"`
	Mode       string    `json:"mode"`
	Delay      string    `json:"delay"`
	Heartbeat  string    `json:"heartbeat"`
	Repetition int       `json:"repetition"`
}

func (r *Runner) logf(format string, args ...interface{}) {
	if r.Logf != nil {
		r.Logf(format, args...)
	} else {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

// RunAll executes every trial sequentially (research-plan sequential trials).
func (r *Runner) RunAll(ctx context.Context, trials []Trial) error {
	if err := os.MkdirAll(r.Exp.OutputDir, 0o755); err != nil {
		return err
	}
	manifestPath := filepath.Join(r.Exp.OutputDir, "manifest.jsonl")
	mf, err := os.OpenFile(manifestPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer mf.Close()

	metaPath := filepath.Join(r.Exp.OutputDir, "experiment.yaml")
	if err := writeYAML(metaPath, r.Exp); err != nil {
		return err
	}

	var firstErr error
	for i, trial := range trials {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		r.logf("[%d/%d] starting %s", i+1, len(trials), trial.RunID)
		st, err := r.RunTrial(ctx, trial)
		b, _ := json.Marshal(st)
		_, _ = mf.Write(append(b, '\n'))
		if err != nil {
			r.logf("[%d/%d] FAILED %s: %v", i+1, len(trials), trial.RunID, err)
			if firstErr == nil {
				firstErr = err
			}
			if !r.Exp.QA.ContinueOnError {
				return err
			}
			continue
		}
		r.logf("[%d/%d] ok %s", i+1, len(trials), trial.RunID)
	}
	return firstErr
}

// RunTrial executes the research-plan procedure for one condition.
func (r *Runner) RunTrial(ctx context.Context, trial Trial) (TrialStatus, error) {
	st := TrialStatus{
		RunID:      trial.RunID,
		StartedAt:  time.Now().UTC(),
		Seed:       trial.Seed,
		Topology:   trial.Topology,
		Mode:       trial.Mode.String(),
		Delay:      trial.Delay.String(),
		Heartbeat:  trial.Heartbeat.String(),
		Repetition: trial.Repetition,
		Leader:     trial.Leader,
	}
	if err := os.MkdirAll(trial.Dir, 0o755); err != nil {
		st.Error = err.Error()
		st.FinishedAt = time.Now().UTC()
		return st, err
	}
	if err := writeYAML(filepath.Join(trial.Dir, "meta.yaml"), trialMeta(trial, r.Exp)); err != nil {
		st.Error = err.Error()
		st.FinishedAt = time.Now().UTC()
		return st, err
	}

	err := r.runTrialInner(ctx, trial, &st)
	st.FinishedAt = time.Now().UTC()
	st.OK = err == nil
	if err != nil {
		st.Error = err.Error()
	}
	_ = writeJSON(filepath.Join(trial.Dir, "status.json"), st)

	// Always attempt teardown.
	_ = r.cleanupHosts(trial)
	_ = r.Client.Close()
	time.Sleep(r.Exp.Timing.TeardownPause.Duration)
	return st, err
}

func (r *Runner) runTrialInner(ctx context.Context, trial Trial, st *TrialStatus) error {
	c := r.Client
	if err := c.Start(ctx, trial.Spec, trial.Mode, trial.Delay); err != nil {
		return fmt.Errorf("start topology: %w", err)
	}

	stopLoad := r.startHostLoadMonitor(trial)
	defer stopLoad()

	tl := collect.Timeline{
		RunID: st.RunID,
		Clock: "CLOCK_REALTIME",
	}

	if err := r.startReplicas(trial); err != nil {
		return err
	}
	time.Sleep(r.Exp.Timing.Settle.Duration)

	if err := r.waitReplicasHealthy(ctx, trial); err != nil {
		_ = r.collectArtifacts(trial)
		return err
	}
	if err := r.waitClusterReady(ctx, trial); err != nil {
		_ = r.collectArtifacts(trial)
		return err
	}
	if err := r.restoreTimeouts(trial); err != nil {
		_ = r.collectArtifacts(trial)
		return err
	}

	clientDur := r.Exp.Timing.Warmup.Duration + r.Exp.Timing.Observe.Duration + 10*time.Second
	if err := r.startClient(trial, clientDur); err != nil {
		return err
	}
	tl.ClientStartedNS = time.Now().UnixNano()

	time.Sleep(r.Exp.Timing.Warmup.Duration)
	tl.WarmupEndNS = time.Now().UnixNano()

	warmupOK, err := r.countRecentCommits(trial)
	if err != nil {
		return fmt.Errorf("warmup check: %w", err)
	}
	st.WarmupOK = warmupOK
	if r.Exp.QA.RequireWarmupCommits && warmupOK < r.Exp.QA.MinWarmupOK {
		return fmt.Errorf("warmup: only %d commits (need >= %d)", warmupOK, r.Exp.QA.MinWarmupOK)
	}

	st.InjectAt = time.Now().UTC()
	st.InjectAtNS = st.InjectAt.UnixNano()
	tl.InjectAt = st.InjectAt.Format(time.RFC3339Nano)
	tl.InjectAtNS = st.InjectAtNS
	if err := c.InjectPlannedFailures(); err != nil {
		return fmt.Errorf("inject: %w", err)
	}
	if r.Exp.QA.ValidateAfterInject {
		if err := c.Validate(ctx); err != nil {
			return fmt.Errorf("validate after inject: %w", err)
		}
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(r.Exp.Timing.Observe.Duration):
	}
	tl.ObserveEndNS = time.Now().UnixNano()

	if err := r.stopClient(trial); err != nil {
		r.logf("stop client: %v", err)
	}
	if err := r.stopReplicas(trial); err != nil {
		r.logf("stop replicas: %v", err)
	}
	if err := r.collectArtifacts(trial); err != nil {
		return fmt.Errorf("collect: %w", err)
	}
	if err := writeJSON(filepath.Join(trial.Dir, "timeline.json"), tl); err != nil {
		return fmt.Errorf("timeline: %w", err)
	}
	m, err := collect.AnalyzeRunDir(trial.Dir, r.Exp.QA.HostLoadMax1m)
	if err != nil {
		r.logf("metrics: %v", err)
	} else if err := collect.WriteMetricsJSON(trial.Dir, m); err != nil {
		r.logf("metrics write: %v", err)
	}
	return nil
}

func (r *Runner) startHostLoadMonitor(trial Trial) func() {
	interval := r.Exp.QA.HostLoadInterval.Duration
	if interval <= 0 {
		interval = time.Second
	}
	path := filepath.Join(trial.Dir, "hostload.jsonl")
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			if s, ok := collect.SampleHostLoad(); ok {
				_ = collect.AppendHostLoadJSONL(path, s)
			}
			select {
			case <-done:
				return
			case <-t.C:
			}
		}
	}()
	return func() { close(done) }
}

func (r *Runner) startReplicas(trial Trial) error {
	peers := raftnode.FormatPeers(trial.Peers)
	// Start non-preferred first with a long election timeout so they do not
	// campaign during warm-up; preferred starts last with -prefer-leader.
	// (T3 seeds let B/D win majority without C if they time out first.)
	followerElection := 30 * time.Second
	if d := r.Exp.Timing.ClusterReadyTimeout.Duration; d > followerElection {
		followerElection = d
	}
	order := make([]string, 0, len(trial.Spec.Nodes))
	for _, n := range trial.Spec.Nodes {
		if n != trial.Leader {
			order = append(order, n)
		}
	}
	order = append(order, trial.Leader)
	for _, id := range order {
		ip := trial.Peers[id]
		events := fmt.Sprintf("/tmp/%s-events.jsonl", trial.RunID+"-"+id)
		logPath := fmt.Sprintf("/tmp/%s-raftd-%s.log", trial.RunID, id)
		pidPath := fmt.Sprintf("/tmp/%s-raftd-%s.pid", trial.RunID, id)
		args := []string{
			shellQuote(r.Exp.Binaries.Raftd),
			"-id", id,
			"-bind", ip,
			"-peers", shellQuote(peers),
			"-raft-port", strconv.Itoa(r.Exp.Raft.RaftPort),
			"-api-port", strconv.Itoa(r.Exp.Raft.APIPort),
			"-heartbeat", trial.Heartbeat.String(),
			"-network-delay", trial.Delay.String(),
			"-events", events,
		}
		if id == trial.Leader {
			args = append(args, "-prefer-leader")
		} else {
			args = append(args, "-election", followerElection.String())
		}
		if r.Exp.Raft.SeedT3 && strings.EqualFold(trial.Topology, "T3") {
			if terms, ok := raftnode.T3SeedLogs[id]; ok {
				args = append(args, "-seed-log", raftnode.FormatSeedLog(terms))
			}
		}
		cmd := fmt.Sprintf("rm -f %s %s; nohup %s > %s 2>&1 & echo $! > %s",
			events, pidPath, strings.Join(args, " "), logPath, pidPath)
		out, err := r.Client.ExecOn(id, cmd)
		if err != nil {
			return fmt.Errorf("start raftd on %s: %w (%s)", id, err, out)
		}
	}
	return nil
}

func (r *Runner) startClient(trial Trial, duration time.Duration) error {
	peers := raftnode.FormatPeers(trial.Peers)
	outPath := fmt.Sprintf("/tmp/%s-client.jsonl", trial.RunID)
	pidPath := fmt.Sprintf("/tmp/%s-client.pid", trial.RunID)
	logPath := fmt.Sprintf("/tmp/%s-client.log", trial.RunID)
	cmd := fmt.Sprintf(
		"rm -f %s %s; nohup %s -peers %s -api-port %d -leader %s -rate %g -duration %s -size %d -timeout %s -out %s > %s 2>&1 & echo $! > %s",
		outPath, pidPath,
		shellQuote(r.Exp.Binaries.Raftclient),
		shellQuote(peers),
		r.Exp.Raft.APIPort,
		trial.Leader,
		r.Exp.Workload.Rate,
		duration.String(),
		r.Exp.Workload.PayloadSize,
		r.Exp.Workload.ApplyTimeout.Duration.String(),
		outPath,
		logPath,
		pidPath,
	)
	out, err := r.Client.ExecOn(trial.ClientNode, cmd)
	if err != nil {
		return fmt.Errorf("start raftclient on %s: %w (%s)", trial.ClientNode, err, out)
	}
	return nil
}

func (r *Runner) stopClient(trial Trial) error {
	pidPath := fmt.Sprintf("/tmp/%s-client.pid", trial.RunID)
	_, err := r.Client.ExecOn(trial.ClientNode, killPIDFile(pidPath))
	return err
}

func (r *Runner) stopReplicas(trial Trial) error {
	var first error
	for _, id := range trial.Spec.Nodes {
		pidPath := fmt.Sprintf("/tmp/%s-raftd-%s.pid", trial.RunID, id)
		if _, err := r.Client.ExecOn(id, killPIDFile(pidPath)); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (r *Runner) cleanupHosts(trial Trial) error {
	_ = r.stopClient(trial)
	_ = r.stopReplicas(trial)
	// Best-effort pkill leftover raftd/raftclient for this run.
	for _, id := range trial.Spec.Nodes {
		_, _ = r.Client.ExecOn(id, fmt.Sprintf("pkill -f %s || true", shellQuote(trial.RunID)))
	}
	return nil
}

func (r *Runner) collectArtifacts(trial Trial) error {
	// Client metrics from client node.
	clientSrc := fmt.Sprintf("/tmp/%s-client.jsonl", trial.RunID)
	if err := r.pullFile(trial.ClientNode, clientSrc, filepath.Join(trial.Dir, "client.jsonl")); err != nil {
		r.logf("collect client.jsonl: %v", err)
	}
	clientLog := fmt.Sprintf("/tmp/%s-client.log", trial.RunID)
	_ = r.pullFile(trial.ClientNode, clientLog, filepath.Join(trial.Dir, "client.log"))

	for _, id := range trial.Spec.Nodes {
		events := fmt.Sprintf("/tmp/%s-events.jsonl", trial.RunID+"-"+id)
		_ = r.pullFile(id, events, filepath.Join(trial.Dir, fmt.Sprintf("events-%s.jsonl", id)))
		logPath := fmt.Sprintf("/tmp/%s-raftd-%s.log", trial.RunID, id)
		_ = r.pullFile(id, logPath, filepath.Join(trial.Dir, fmt.Sprintf("raftd-%s.log", id)))
	}
	return nil
}

func (r *Runner) pullFile(node, remote, local string) error {
	out, err := r.Client.ExecOn(node, fmt.Sprintf("cat %s 2>/dev/null || true", shellQuote(remote)))
	if err != nil {
		return err
	}
	return os.WriteFile(local, []byte(out), 0o644)
}

func (r *Runner) waitReplicasHealthy(ctx context.Context, trial Trial) error {
	deadline := time.Now().Add(r.Exp.Timing.ClusterReadyTimeout.Duration)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		allOK := true
		for _, id := range trial.Spec.Nodes {
			url := fmt.Sprintf("http://%s:%d/health", trial.Peers[id], r.Exp.Raft.APIPort)
			out, err := r.Client.ExecOn(id, httpGetCmd(url))
			if err != nil || !(strings.Contains(out, `"status":"ok"`) || strings.Contains(out, `"status": "ok"`)) {
				allOK = false
				break
			}
		}
		if allOK {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("raftd health: not all nodes healthy within %s", r.Exp.Timing.ClusterReadyTimeout.Duration)
}

func (r *Runner) waitClusterReady(ctx context.Context, trial Trial) error {
	deadline := time.Now().Add(r.Exp.Timing.ClusterReadyTimeout.Duration)
	var lastLeader string
	var lastXfer string
	nextXfer := time.Time{}
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		leaderID, err := r.findLeader(trial)
		if err == nil && leaderID != "" {
			lastLeader = leaderID
			if leaderID == trial.Leader {
				return nil
			}
			// Transfer must hit the *current leader* API (not a follower that
			// merely reported leader_id).
			if !time.Now().Before(nextXfer) {
				xferURL := fmt.Sprintf("http://%s:%d/transfer?id=%s",
					trial.Peers[leaderID], r.Exp.Raft.APIPort, trial.Leader)
				out, xerr := r.Client.ExecOn(leaderID, httpPostCmd(xferURL))
				lastXfer = strings.TrimSpace(out)
				if xerr != nil && lastXfer == "" {
					lastXfer = xerr.Error()
				}
				r.logf("transfer %s -> %s: %s", leaderID, trial.Leader, lastXfer)
				nextXfer = time.Now().Add(2 * time.Second)
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	if lastLeader == "" {
		return fmt.Errorf("cluster not ready: no leader elected within %s (want %s)",
			r.Exp.Timing.ClusterReadyTimeout.Duration, trial.Leader)
	}
	if lastXfer != "" {
		return fmt.Errorf("cluster not ready: leader=%s want=%s within %s (last transfer: %s)",
			lastLeader, trial.Leader, r.Exp.Timing.ClusterReadyTimeout.Duration, lastXfer)
	}
	return fmt.Errorf("cluster not ready: leader=%s want=%s within %s",
		lastLeader, trial.Leader, r.Exp.Timing.ClusterReadyTimeout.Duration)
}

func (r *Runner) restoreTimeouts(trial Trial) error {
	// Followers deferred campaigns via a long HeartbeatTimeout; restore the
	// matrix heartbeat so failure detection works after inject.
	hb := trial.Heartbeat.String()
	var first error
	for _, id := range trial.Spec.Nodes {
		url := fmt.Sprintf("http://%s:%d/timeouts?heartbeat=%s", trial.Peers[id], r.Exp.Raft.APIPort, hb)
		out, err := r.Client.ExecOn(id, httpPostCmd(url))
		if err != nil {
			if first == nil {
				first = fmt.Errorf("restore timeouts on %s: %w (%s)", id, err, strings.TrimSpace(out))
			}
			continue
		}
		if !strings.Contains(out, `"ok":true`) && !strings.Contains(out, `"ok": true`) {
			if first == nil {
				first = fmt.Errorf("restore timeouts on %s: %s", id, strings.TrimSpace(out))
			}
		}
	}
	return first
}

// findLeader returns the current leader ID, if any.
func (r *Runner) findLeader(trial Trial) (string, error) {
	// Prefer a node that reports state=Leader (authoritative) over followers'
	// cached leader_id, which can lag during transfers.
	var reported string
	for _, id := range trial.Spec.Nodes {
		url := fmt.Sprintf("http://%s:%d/stats", trial.Peers[id], r.Exp.Raft.APIPort)
		out, err := r.Client.ExecOn(id, httpGetCmd(url))
		if err != nil {
			continue
		}
		var stats map[string]interface{}
		if json.Unmarshal([]byte(strings.TrimSpace(out)), &stats) != nil {
			continue
		}
		state, _ := stats["state"].(string)
		if state == "Leader" {
			return id, nil
		}
		if lid, ok := stats["leader_id"].(string); ok && lid != "" && reported == "" {
			reported = lid
		}
	}
	if reported != "" {
		return reported, nil
	}
	return "", fmt.Errorf("no leader")
}

func (r *Runner) countRecentCommits(trial Trial) (int, error) {
	// Count ok=true lines in the live client file inside the client namespace.
	src := fmt.Sprintf("/tmp/%s-client.jsonl", trial.RunID)
	out, err := r.Client.ExecOn(trial.ClientNode,
		fmt.Sprintf("grep -c '\"ok\":true' %s 2>/dev/null || echo 0", shellQuote(src)))
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		// grep -c may print "0\n" with noise
		fields := strings.Fields(out)
		if len(fields) == 0 {
			return 0, nil
		}
		n, err = strconv.Atoi(fields[0])
		if err != nil {
			return 0, fmt.Errorf("parse commit count %q: %w", out, err)
		}
	}
	return n, nil
}

func httpGetCmd(url string) string {
	// Prefer python3 (usually present); fall back to curl/wget.
	return fmt.Sprintf(
		`python3 -c "import urllib.request; print(urllib.request.urlopen('%s', timeout=1).read().decode())" 2>/dev/null || curl -fsS --max-time 1 %s 2>/dev/null || wget -qO- %s 2>/dev/null`,
		url, shellQuote(url), shellQuote(url),
	)
}

func httpPostCmd(url string) string {
	return fmt.Sprintf(
		`python3 -c "import urllib.request; print(urllib.request.urlopen(urllib.request.Request('%s', method='POST'), timeout=3).read().decode())" 2>/dev/null || curl -fsS -X POST --max-time 3 %s 2>/dev/null`,
		url, shellQuote(url),
	)
}

func killPIDFile(pidPath string) string {
	return fmt.Sprintf(
		`if [ -f %s ]; then kill $(cat %s) 2>/dev/null || true; rm -f %s; fi`,
		shellQuote(pidPath), shellQuote(pidPath), shellQuote(pidPath),
	)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func writeYAML(path string, v interface{}) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func writeJSON(path string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

type trialMetaFile struct {
	RunID      string            `yaml:"run_id"`
	Topology   string            `yaml:"topology"`
	Mode       string            `yaml:"mode"`
	Delay      string            `yaml:"delay"`
	Heartbeat  string            `yaml:"heartbeat"`
	Repetition int               `yaml:"repetition"`
	Seed       int64             `yaml:"seed"`
	Leader     string            `yaml:"leader"`
	ClientNode string            `yaml:"client_node"`
	Peers      map[string]string `yaml:"peers"`
	Workload   WorkloadConfig    `yaml:"workload"`
	Timing     TimingConfig      `yaml:"timing"`
}

func trialMeta(t Trial, exp Experiment) trialMetaFile {
	return trialMetaFile{
		RunID:      t.RunID,
		Topology:   t.Topology,
		Mode:       t.Mode.String(),
		Delay:      t.Delay.String(),
		Heartbeat:  t.Heartbeat.String(),
		Repetition: t.Repetition,
		Seed:       t.Seed,
		Leader:     t.Leader,
		ClientNode: t.ClientNode,
		Peers:      t.Peers,
		Workload:   exp.Workload,
		Timing:     exp.Timing,
	}
}
