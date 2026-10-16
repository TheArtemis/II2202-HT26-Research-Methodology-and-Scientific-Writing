package raftnode

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/hashicorp/raft"
)

// APIServer is the client-facing HTTP surface for Apply and cluster stats.
type APIServer struct {
	node   *Node
	server *http.Server
	ln     net.Listener
}

// NewAPIServer starts listening on addr.
func NewAPIServer(n *Node, addr string) (*APIServer, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("api listen %s: %w", addr, err)
	}
	api := &APIServer{node: n, ln: ln}
	mux := http.NewServeMux()
	mux.HandleFunc("/apply", api.handleApply)
	mux.HandleFunc("/stats", api.handleStats)
	mux.HandleFunc("/health", api.handleHealth)
	mux.HandleFunc("/transfer", api.handleTransfer)
	mux.HandleFunc("/timeouts", api.handleTimeouts)
	api.server = &http.Server{Handler: mux}
	go func() { _ = api.server.Serve(ln) }()
	return api, nil
}

// Close shuts down the HTTP server.
func (a *APIServer) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.server.Shutdown(ctx)
}

// Addr returns the listening address.
func (a *APIServer) Addr() string {
	return a.ln.Addr().String()
}

type applyRequestBody struct {
	ID      string `json:"id"`
	Payload []byte `json:"payload,omitempty"`
}

type applyResponseBody struct {
	OK       bool   `json:"ok"`
	ID       string `json:"id,omitempty"`
	Index    uint64 `json:"index,omitempty"`
	Term     uint64 `json:"term,omitempty"`
	Leader   string `json:"leader,omitempty"`
	LeaderID string `json:"leader_id,omitempty"`
	Error    string `json:"error,omitempty"`
}

func (a *APIServer) handleApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body applyRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, applyResponseBody{OK: false, Error: err.Error()})
		return
	}
	if body.ID == "" {
		writeJSON(w, http.StatusBadRequest, applyResponseBody{OK: false, Error: "id required"})
		return
	}
	timeout := 2 * time.Second
	if t := r.URL.Query().Get("timeout"); t != "" {
		if d, err := time.ParseDuration(t); err == nil {
			timeout = d
		}
	}
	res, err := a.node.Apply(Request{ID: body.ID, Payload: body.Payload}, timeout)
	if err != nil {
		status := http.StatusServiceUnavailable
		resp := applyResponseBody{OK: false, Error: err.Error(), ID: body.ID}
		if nl, ok := err.(NotLeaderError); ok {
			resp.Leader = nl.LeaderAddr
			resp.LeaderID = nl.LeaderID
			w.Header().Set("X-Raft-Leader", nl.LeaderAddr)
			w.Header().Set("X-Raft-Leader-ID", nl.LeaderID)
		}
		writeJSON(w, status, resp)
		return
	}
	writeJSON(w, http.StatusOK, applyResponseBody{
		OK:    true,
		ID:    res.ID,
		Index: res.Index,
		Term:  res.Term,
	})
}

func (a *APIServer) handleStats(w http.ResponseWriter, r *http.Request) {
	applied, lastIdx := a.node.FSM.Stats()
	leaderAddr := string(a.node.Raft.Leader())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":             a.node.Config.ID,
		"state":          a.node.Raft.State().String(),
		"term":           a.node.Raft.CurrentTerm(),
		"leader":         leaderAddr,
		"leader_id":      leaderIDFromPeers(a.node),
		"commit_index":   a.node.Raft.CommitIndex(),
		"applied_index":  a.node.Raft.AppliedIndex(),
		"last_log_index": lastIdx,
		"applied_count":  applied,
		"api_addr":       a.node.Config.APIAddr(),
		"raft_addr":      a.node.Config.RaftAddr(),
	})
}

func (a *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"id":     a.node.Config.ID,
		"state":  a.node.Raft.State().String(),
	})
}

func (a *APIServer) handleTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "id required"})
		return
	}
	ip, ok := a.node.Config.Peers[id]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "unknown peer"})
		return
	}
	if a.node.Raft.State() != raft.Leader {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"ok":        false,
			"error":     "not leader",
			"leader_id": leaderIDFromPeers(a.node),
		})
		return
	}
	addr := raft.ServerAddress(fmt.Sprintf("%s:%d", ip, a.node.Config.RaftPort))
	if err := a.node.Raft.LeadershipTransferToServer(raft.ServerID(id), addr).Error(); err != nil {
		writeJSON(w, http.StatusConflict, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "id": id})
}

func (a *APIServer) handleTimeouts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	hbStr := r.URL.Query().Get("heartbeat")
	if hbStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "heartbeat required"})
		return
	}
	hb, err := time.ParseDuration(hbStr)
	if err != nil || hb < 5*time.Millisecond {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "invalid heartbeat"})
		return
	}
	election := deriveElectionTimeout(Config{
		Heartbeat:    hb,
		NetworkDelay: a.node.Config.NetworkDelay,
		PreferLeader: false,
	})
	if election < hb {
		election = hb
	}
	rc := a.node.Raft.ReloadableConfig()
	rc.HeartbeatTimeout = hb
	rc.ElectionTimeout = election
	if err := a.node.Raft.ReloadConfig(rc); err != nil {
		writeJSON(w, http.StatusConflict, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	a.node.Config.Heartbeat = hb
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"heartbeat": hb.String(),
		"election":  election.String(),
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
