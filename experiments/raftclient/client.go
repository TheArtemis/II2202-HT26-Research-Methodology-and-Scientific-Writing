package raftclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Record is one open-loop client measurement (research-plan RQ2).
type Record struct {
	ID        string        `json:"id"`
	SubmitNS  int64         `json:"submit_ns"`
	CommitNS  int64         `json:"commit_ns,omitempty"`
	LatencyNS int64         `json:"latency_ns,omitempty"`
	OK        bool          `json:"ok"`
	Index     uint64        `json:"index,omitempty"`
	Term      uint64        `json:"term,omitempty"`
	LeaderID  string        `json:"leader_id,omitempty"`
	Error     string        `json:"error,omitempty"`
}

// Config configures the open-loop workload.
type Config struct {
	// Peers are HTTP base URLs, e.g. http://10.0.0.1:7001
	Peers []string
	// Rate is offered requests per second (open-loop; does not wait for replies).
	Rate float64
	// Duration bounds the send window. Zero runs until ctx cancel.
	Duration time.Duration
	// PayloadSize is the number of payload bytes per request.
	PayloadSize int
	// Timeout is the per-request Apply HTTP timeout.
	Timeout time.Duration
	// OutPath is the JSONL metrics file. Empty → stdout.
	OutPath string
	// LeaderID optionally starts requests at a known leader's peer index.
	LeaderID string
	// PeerIDs aligns with Peers (same order) for leader sticky routing.
	PeerIDs []string
}

// Client continuously submits Apply requests without waiting for prior completion.
type Client struct {
	cfg    Config
	http   *http.Client
	seq    atomic.Uint64
	leader atomic.Int32 // index into Peers

	outMu sync.Mutex
	out   io.Writer
	file  *os.File

	sent     atomic.Uint64
	ok       atomic.Uint64
	failed   atomic.Uint64
	inflight atomic.Int64
}

// New constructs an open-loop client.
func New(cfg Config) (*Client, error) {
	if len(cfg.Peers) == 0 {
		return nil, fmt.Errorf("raftclient: at least one peer URL required")
	}
	if cfg.Rate <= 0 {
		cfg.Rate = 100
	}
	if cfg.PayloadSize < 0 {
		return nil, fmt.Errorf("raftclient: PayloadSize must be >= 0")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Second
	}
	c := &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 64,
				MaxConnsPerHost:     128,
			},
		},
		out: os.Stdout,
	}
	if cfg.OutPath != "" {
		f, err := os.OpenFile(cfg.OutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, err
		}
		c.file = f
		c.out = f
	}
	if cfg.LeaderID != "" && len(cfg.PeerIDs) == len(cfg.Peers) {
		for i, id := range cfg.PeerIDs {
			if id == cfg.LeaderID {
				c.leader.Store(int32(i))
				break
			}
		}
	}
	return c, nil
}

// Close flushes the metrics file.
func (c *Client) Close() error {
	if c.file != nil {
		return c.file.Close()
	}
	return nil
}

// Stats returns sent/ok/failed counts.
func (c *Client) Stats() (sent, ok, failed uint64) {
	return c.sent.Load(), c.ok.Load(), c.failed.Load()
}

// Run sends at cfg.Rate until duration elapses or ctx is cancelled, then waits
// for in-flight requests to finish (or ctx cancel).
func (c *Client) Run(ctx context.Context) error {
	interval := time.Duration(float64(time.Second) / c.cfg.Rate)
	if interval < time.Microsecond {
		interval = time.Microsecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var sendCtx context.Context
	var cancel context.CancelFunc
	if c.cfg.Duration > 0 {
		sendCtx, cancel = context.WithTimeout(ctx, c.cfg.Duration)
	} else {
		sendCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	var wg sync.WaitGroup
	payload := make([]byte, c.cfg.PayloadSize)
	for i := range payload {
		payload[i] = 'x'
	}

loop:
	for {
		select {
		case <-sendCtx.Done():
			break loop
		case <-ticker.C:
			id := fmt.Sprintf("%d", c.seq.Add(1))
			wg.Add(1)
			c.inflight.Add(1)
			c.sent.Add(1)
			go func(reqID string, body []byte) {
				defer wg.Done()
				defer c.inflight.Add(-1)
				c.doOne(ctx, reqID, body)
			}(id, payload)
		}
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) doOne(ctx context.Context, id string, payload []byte) {
	submit := time.Now()
	rec := Record{
		ID:       id,
		SubmitNS: submit.UnixNano(),
	}
	idx := int(c.leader.Load())
	if idx < 0 || idx >= len(c.cfg.Peers) {
		idx = 0
	}

	var lastErr error
	for attempt := 0; attempt < len(c.cfg.Peers); attempt++ {
		peerIdx := (idx + attempt) % len(c.cfg.Peers)
		base := c.cfg.Peers[peerIdx]
		ok, index, term, leaderID, err := c.applyOne(ctx, base, id, payload)
		if err == nil && ok {
			commit := time.Now()
			rec.OK = true
			rec.CommitNS = commit.UnixNano()
			rec.LatencyNS = commit.Sub(submit).Nanoseconds()
			rec.Index = index
			rec.Term = term
			if leaderID != "" {
				rec.LeaderID = leaderID
			} else if peerIdx < len(c.cfg.PeerIDs) {
				rec.LeaderID = c.cfg.PeerIDs[peerIdx]
			}
			c.leader.Store(int32(peerIdx))
			c.ok.Add(1)
			c.write(rec)
			return
		}
		lastErr = err
		if leaderID != "" {
			if i := c.indexOfPeerID(leaderID); i >= 0 {
				idx = i
				c.leader.Store(int32(i))
			}
		}
	}
	rec.OK = false
	if lastErr != nil {
		rec.Error = lastErr.Error()
	} else {
		rec.Error = "apply failed"
	}
	c.failed.Add(1)
	c.write(rec)
}

func (c *Client) indexOfPeerID(id string) int {
	for i, p := range c.cfg.PeerIDs {
		if p == id {
			return i
		}
	}
	return -1
}

type applyResp struct {
	OK       bool   `json:"ok"`
	ID       string `json:"id"`
	Index    uint64 `json:"index"`
	Term     uint64 `json:"term"`
	Leader   string `json:"leader"`
	LeaderID string `json:"leader_id"`
	Error    string `json:"error"`
}

func (c *Client) applyOne(ctx context.Context, base, id string, payload []byte) (ok bool, index, term uint64, leaderID string, err error) {
	body, _ := json.Marshal(map[string]interface{}{
		"id":      id,
		"payload": payload,
	})
	url := base + "/apply"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return false, 0, 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return false, 0, 0, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var ar applyResp
	if err := json.Unmarshal(raw, &ar); err != nil {
		return false, 0, 0, "", fmt.Errorf("decode %s: %w (%s)", url, err, string(raw))
	}
	if ar.LeaderID == "" {
		ar.LeaderID = resp.Header.Get("X-Raft-Leader-ID")
	}
	if resp.StatusCode == http.StatusOK && ar.OK {
		return true, ar.Index, ar.Term, ar.LeaderID, nil
	}
	msg := ar.Error
	if msg == "" {
		msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return false, 0, 0, ar.LeaderID, fmt.Errorf("%s", msg)
}

func (c *Client) write(rec Record) {
	c.outMu.Lock()
	defer c.outMu.Unlock()
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	_, _ = c.out.Write(append(b, '\n'))
}
