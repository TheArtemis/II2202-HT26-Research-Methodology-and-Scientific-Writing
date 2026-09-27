// Package collect derives research-plan dependent variables from per-run artifacts.
package collect

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ClientRecord is one open-loop client measurement (RQ2 raw).
type ClientRecord struct {
	ID        string `json:"id"`
	SubmitNS  int64  `json:"submit_ns"`
	CommitNS  int64  `json:"commit_ns,omitempty"`
	LatencyNS int64  `json:"latency_ns,omitempty"`
	OK        bool   `json:"ok"`
	Index     uint64 `json:"index,omitempty"`
	Term      uint64 `json:"term,omitempty"`
	LeaderID  string `json:"leader_id,omitempty"`
	Error     string `json:"error,omitempty"`
	Phase     string `json:"phase,omitempty"` // warmup | observe | other (enriched)
}

// RaftEvent is one Raft observation (RQ1 raw).
type RaftEvent struct {
	TS     time.Time `json:"ts"`
	TSNS   int64     `json:"ts_ns"`
	Type   string    `json:"type"`
	Node   string    `json:"node"`
	Leader string    `json:"leader,omitempty"`
	Term   uint64    `json:"term,omitempty"`
	Index  uint64    `json:"index,omitempty"`
}

// Timeline marks trial phases (written by harness).
type Timeline struct {
	RunID           string `json:"run_id"`
	ClientStartedNS int64  `json:"client_started_ns"`
	WarmupEndNS     int64  `json:"warmup_end_ns"`
	InjectAtNS      int64  `json:"inject_at_ns"`
	InjectAt        string `json:"inject_at,omitempty"`
	ObserveEndNS    int64  `json:"observe_end_ns"`
	Clock           string `json:"clock"`
}

// Status mirrors harness TrialStatus fields used for joining.
type Status struct {
	RunID      string    `json:"run_id"`
	OK         bool      `json:"ok"`
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	InjectAt   time.Time `json:"inject_at"`
	WarmupOK   int       `json:"warmup_ok_commits"`
	Leader     string    `json:"leader"`
	Seed       int64     `json:"seed"`
	Topology   string    `json:"topology"`
	Mode       string    `json:"mode"`
	Delay      string    `json:"delay"`
	Heartbeat  string    `json:"heartbeat"`
	Repetition int       `json:"repetition"`
}

// HostLoadSample is one controller-host load observation.
type HostLoadSample struct {
	TSNS          int64   `json:"ts_ns"`
	Load1         float64 `json:"load1"`
	Load5         float64 `json:"load5"`
	Load15        float64 `json:"load15"`
	MemAvailableKB int64  `json:"mem_available_kb,omitempty"`
}

// RunMetrics holds research-plan dependent variables for one trial.
type RunMetrics struct {
	// Independent variables (joined for analysis).
	RunID      string `json:"run_id"`
	Topology   string `json:"topology"`
	Mode       string `json:"mode"`
	Delay      string `json:"delay"`
	Heartbeat  string `json:"heartbeat"`
	Repetition int    `json:"repetition"`
	Seed       int64  `json:"seed"`
	TrialOK    bool   `json:"trial_ok"`

	// RQ1
	CommitThroughputObserve float64 `json:"commit_throughput_observe_eps"` // successful client commits / observe window
	CommitThroughputWarmup  float64 `json:"commit_throughput_warmup_eps"`
	RecoveryTimeNS          *int64  `json:"recovery_time_ns"` // inject → first post-inject commit; null if none
	RecoveryTimeMS          *float64 `json:"recovery_time_ms"`
	ElectionsObserve        int     `json:"elections_observe"`
	TermChangesObserve      int     `json:"term_changes_observe"`
	ElectionsWarmup         int     `json:"elections_warmup"`
	TermChangesWarmup       int     `json:"term_changes_warmup"`
	StableProgress          bool    `json:"stable_progress"` // heuristic: recovered + commits in observe + limited churn
	RaftCommitsObserve      int     `json:"raft_commits_observe"`
	ClientOKObserve         int     `json:"client_ok_observe"`
	ClientFailObserve       int     `json:"client_fail_observe"`
	ClientOKWarmup          int     `json:"client_ok_warmup"`

	// RQ2 — observe-window successful commit latency
	LatencyCount      int      `json:"latency_count"`
	LatencyMedianNS   *int64   `json:"latency_median_ns"`
	LatencyP95NS      *int64   `json:"latency_p95_ns"`
	LatencyMeanNS     *float64 `json:"latency_mean_ns"`
	LatencyMedianMS   *float64 `json:"latency_median_ms"`
	LatencyP95MS      *float64 `json:"latency_p95_ms"`

	// Throughput over time (1s buckets in observe window)
	ThroughputSeries []ThroughputBucket `json:"throughput_series,omitempty"`

	// Host load summary
	HostLoadMax1   *float64 `json:"host_load_max_1m,omitempty"`
	HostLoadMean1  *float64 `json:"host_load_mean_1m,omitempty"`
	HostOverloaded bool     `json:"host_overloaded,omitempty"`
}

// ThroughputBucket is commits/sec in a 1-second observe slice.
type ThroughputBucket struct {
	OffsetS float64 `json:"offset_s"`
	EPS     float64 `json:"eps"`
}

// AnalyzeRunDir computes metrics from a harness run directory.
func AnalyzeRunDir(dir string, hostLoadMax1 float64) (RunMetrics, error) {
	st, err := readStatus(filepath.Join(dir, "status.json"))
	if err != nil {
		return RunMetrics{}, err
	}
	tl, _ := readTimeline(filepath.Join(dir, "timeline.json"))
	injectNS := tl.InjectAtNS
	if injectNS == 0 && !st.InjectAt.IsZero() {
		injectNS = st.InjectAt.UnixNano()
	}
	observeEnd := tl.ObserveEndNS
	if observeEnd == 0 {
		observeEnd = st.FinishedAt.UnixNano()
	}
	warmupEnd := tl.WarmupEndNS
	if warmupEnd == 0 {
		warmupEnd = injectNS
	}
	clientStart := tl.ClientStartedNS

	clients, err := readClientJSONL(filepath.Join(dir, "client.jsonl"))
	if err != nil {
		return RunMetrics{}, err
	}
	events, err := readAllEvents(dir)
	if err != nil {
		return RunMetrics{}, err
	}
	loads, _ := readHostLoad(filepath.Join(dir, "hostload.jsonl"))

	m := RunMetrics{
		RunID:      st.RunID,
		Topology:   st.Topology,
		Mode:       st.Mode,
		Delay:      st.Delay,
		Heartbeat:  st.Heartbeat,
		Repetition: st.Repetition,
		Seed:       st.Seed,
		TrialOK:    st.OK,
	}

	var warmLat, obsLat []int64
	var obsOK, obsFail, warmOK int
	for i := range clients {
		rec := &clients[i]
		phase := classifyPhase(rec.SubmitNS, clientStart, warmupEnd, injectNS, observeEnd)
		rec.Phase = phase
		if !rec.OK {
			if phase == "observe" {
				obsFail++
			}
			continue
		}
		switch phase {
		case "warmup":
			warmOK++
			if rec.LatencyNS > 0 {
				warmLat = append(warmLat, rec.LatencyNS)
			}
		case "observe":
			obsOK++
			if rec.LatencyNS > 0 {
				obsLat = append(obsLat, rec.LatencyNS)
			}
		}
	}
	m.ClientOKWarmup = warmOK
	m.ClientOKObserve = obsOK
	m.ClientFailObserve = obsFail

	warmDur := secondsBetween(clientStart, warmupEnd)
	if warmDur == 0 && warmOK > 0 && injectNS > 0 {
		// Infer warmup window from first warmup submit when timeline is missing.
		var first int64
		for _, rec := range clients {
			if rec.Phase == "warmup" && rec.OK {
				if first == 0 || rec.SubmitNS < first {
					first = rec.SubmitNS
				}
			}
		}
		if first > 0 {
			warmDur = secondsBetween(first, injectNS)
		}
	}
	obsDur := secondsBetween(injectNS, observeEnd)
	if warmDur > 0 {
		m.CommitThroughputWarmup = float64(warmOK) / warmDur
	}
	if obsDur > 0 {
		m.CommitThroughputObserve = float64(obsOK) / obsDur
	}

	// Recovery: first successful client commit at/after inject.
	if injectNS > 0 {
		var first int64
		for _, rec := range clients {
			if !rec.OK || rec.CommitNS < injectNS {
				continue
			}
			if first == 0 || rec.CommitNS < first {
				first = rec.CommitNS
			}
		}
		if first > 0 {
			rt := first - injectNS
			m.RecoveryTimeNS = &rt
			ms := float64(rt) / 1e6
			m.RecoveryTimeMS = &ms
		}
	}

	m.ElectionsWarmup, m.TermChangesWarmup = countRaftChurn(events, clientStart, injectNS)
	m.ElectionsObserve, m.TermChangesObserve = countRaftChurn(events, injectNS, observeEnd)
	m.RaftCommitsObserve = countRaftCommits(events, injectNS, observeEnd)

	// Stable progress heuristic (research-plan RQ1): recovered (or no inject gap),
	// at least one observe commit, and election count not exploding vs throughput.
	recovered := m.RecoveryTimeNS != nil || obsOK == 0 && injectNS == 0
	if injectNS > 0 {
		recovered = m.RecoveryTimeNS != nil
	}
	churnOK := m.ElectionsObserve <= max(5, obsOK/10+3)
	m.StableProgress = st.OK && recovered && obsOK > 0 && churnOK

	if len(obsLat) > 0 {
		sort.Slice(obsLat, func(i, j int) bool { return obsLat[i] < obsLat[j] })
		med := percentileSorted(obsLat, 0.50)
		p95 := percentileSorted(obsLat, 0.95)
		m.LatencyMedianNS = &med
		m.LatencyP95NS = &p95
		mean := meanInt64(obsLat)
		m.LatencyMeanNS = &mean
		medMS := float64(med) / 1e6
		p95MS := float64(p95) / 1e6
		m.LatencyMedianMS = &medMS
		m.LatencyP95MS = &p95MS
		m.LatencyCount = len(obsLat)
	}

	m.ThroughputSeries = throughputSeries(clients, injectNS, observeEnd)
	m.HostLoadMax1, m.HostLoadMean1 = summarizeLoad(loads)
	if hostLoadMax1 > 0 && m.HostLoadMax1 != nil && *m.HostLoadMax1 > hostLoadMax1 {
		m.HostOverloaded = true
	}

	// Write enriched client file for analysis joins.
	_ = writeClientEnriched(filepath.Join(dir, "client_enriched.jsonl"), clients)

	return m, nil
}

func classifyPhase(submitNS, clientStart, warmupEnd, injectNS, observeEnd int64) string {
	if injectNS > 0 && submitNS >= injectNS {
		if observeEnd == 0 || submitNS <= observeEnd {
			return "observe"
		}
		return "other"
	}
	if clientStart > 0 && submitNS >= clientStart && (warmupEnd == 0 || submitNS < warmupEnd) {
		return "warmup"
	}
	if injectNS == 0 {
		return "other"
	}
	if submitNS < injectNS {
		return "warmup"
	}
	return "other"
}

func countRaftChurn(events []RaftEvent, fromNS, toNS int64) (elections, terms int) {
	seenTerm := map[uint64]bool{}
	for _, ev := range events {
		ts := eventNS(ev)
		if fromNS > 0 && ts < fromNS {
			continue
		}
		if toNS > 0 && ts > toNS {
			continue
		}
		switch ev.Type {
		case "election":
			elections++
		case "term":
			if ev.Term > 0 && !seenTerm[ev.Term] {
				seenTerm[ev.Term] = true
				terms++
			}
		}
	}
	return elections, terms
}

func countRaftCommits(events []RaftEvent, fromNS, toNS int64) int {
	n := 0
	for _, ev := range events {
		if ev.Type != "commit" {
			continue
		}
		ts := eventNS(ev)
		if fromNS > 0 && ts < fromNS {
			continue
		}
		if toNS > 0 && ts > toNS {
			continue
		}
		n++
	}
	return n
}

func eventNS(ev RaftEvent) int64 {
	if ev.TSNS > 0 {
		return ev.TSNS
	}
	if !ev.TS.IsZero() {
		return ev.TS.UnixNano()
	}
	return 0
}

func throughputSeries(clients []ClientRecord, injectNS, observeEnd int64) []ThroughputBucket {
	if injectNS <= 0 || observeEnd <= injectNS {
		return nil
	}
	dur := observeEnd - injectNS
	buckets := int(dur/int64(time.Second)) + 1
	if buckets > 600 {
		buckets = 600
	}
	counts := make([]int, buckets)
	for _, rec := range clients {
		if !rec.OK || rec.CommitNS < injectNS || rec.CommitNS > observeEnd {
			continue
		}
		b := int((rec.CommitNS - injectNS) / int64(time.Second))
		if b >= 0 && b < len(counts) {
			counts[b]++
		}
	}
	out := make([]ThroughputBucket, 0, len(counts))
	for i, c := range counts {
		out = append(out, ThroughputBucket{OffsetS: float64(i), EPS: float64(c)})
	}
	return out
}

func summarizeLoad(loads []HostLoadSample) (max1, mean1 *float64) {
	if len(loads) == 0 {
		return nil, nil
	}
	var sum, mx float64
	for i, s := range loads {
		sum += s.Load1
		if i == 0 || s.Load1 > mx {
			mx = s.Load1
		}
	}
	mean := sum / float64(len(loads))
	return &mx, &mean
}

func secondsBetween(a, b int64) float64 {
	if a <= 0 || b <= 0 || b <= a {
		return 0
	}
	return float64(b-a) / 1e9
}

func percentileSorted(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func meanInt64(vals []int64) float64 {
	var s float64
	for _, v := range vals {
		s += float64(v)
	}
	return s / float64(len(vals))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func readStatus(path string) (Status, error) {
	var st Status
	b, err := os.ReadFile(path)
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, err
	}
	return st, nil
}

func readTimeline(path string) (Timeline, error) {
	var tl Timeline
	b, err := os.ReadFile(path)
	if err != nil {
		return tl, err
	}
	if err := json.Unmarshal(b, &tl); err != nil {
		return tl, err
	}
	return tl, nil
}

func readClientJSONL(path string) ([]ClientRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []ClientRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec ClientRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out, sc.Err()
}

func readAllEvents(dir string) ([]RaftEvent, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "events-*.jsonl"))
	if err != nil {
		return nil, err
	}
	var out []RaftEvent
	for _, path := range matches {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			var ev RaftEvent
			if err := json.Unmarshal([]byte(line), &ev); err != nil {
				continue
			}
			out = append(out, ev)
		}
		f.Close()
	}
	return out, nil
}

func readHostLoad(path string) ([]HostLoadSample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []HostLoadSample
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var s HostLoadSample
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			continue
		}
		out = append(out, s)
	}
	return out, sc.Err()
}

func writeClientEnriched(path string, recs []ClientRecord) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, rec := range recs {
		if err := enc.Encode(rec); err != nil {
			return err
		}
	}
	return nil
}

// WriteMetricsJSON writes metrics.json into the run directory.
func WriteMetricsJSON(dir string, m RunMetrics) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "metrics.json"), append(b, '\n'), 0o644)
}

// JoinDataset walks an experiment output dir and writes dataset.jsonl (+ optional CSV).
func JoinDataset(outputDir string, hostLoadMax1 float64) (n int, err error) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return 0, err
	}
	jsonlPath := filepath.Join(outputDir, "dataset.jsonl")
	csvPath := filepath.Join(outputDir, "dataset.csv")
	jf, err := os.Create(jsonlPath)
	if err != nil {
		return 0, err
	}
	defer jf.Close()
	cf, err := os.Create(csvPath)
	if err != nil {
		return 0, err
	}
	defer cf.Close()
	fmt.Fprintln(cf, strings.Join([]string{
		"run_id", "topology", "mode", "delay", "heartbeat", "repetition", "seed", "trial_ok",
		"commit_throughput_observe_eps", "recovery_time_ms", "elections_observe", "term_changes_observe",
		"stable_progress", "latency_median_ms", "latency_p95_ms", "latency_count",
		"client_ok_observe", "client_fail_observe", "host_load_max_1m", "host_overloaded",
	}, ","))

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(outputDir, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "status.json")); err != nil {
			continue
		}
		m, err := AnalyzeRunDir(dir, hostLoadMax1)
		if err != nil {
			continue
		}
		_ = WriteMetricsJSON(dir, m)
		b, _ := json.Marshal(m)
		_, _ = jf.Write(append(b, '\n'))
		fmt.Fprintf(cf, "%s,%s,%s,%s,%s,%d,%d,%t,%.6f,%s,%d,%d,%t,%s,%s,%d,%d,%d,%s,%t\n",
			m.RunID, m.Topology, m.Mode, m.Delay, m.Heartbeat, m.Repetition, m.Seed, m.TrialOK,
			m.CommitThroughputObserve, fmtOptFloat(m.RecoveryTimeMS), m.ElectionsObserve, m.TermChangesObserve,
			m.StableProgress, fmtOptFloat(m.LatencyMedianMS), fmtOptFloat(m.LatencyP95MS), m.LatencyCount,
			m.ClientOKObserve, m.ClientFailObserve, fmtOptFloat(m.HostLoadMax1), m.HostOverloaded,
		)
		n++
	}
	return n, nil
}

func fmtOptFloat(p *float64) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%.6f", *p)
}
