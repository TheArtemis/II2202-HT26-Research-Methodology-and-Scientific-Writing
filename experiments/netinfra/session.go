package netinfra

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const defaultSessionPath = "/tmp/netinfra-session.json"

// SessionPath returns the path used to persist Start() metadata between CLI invocations.
func SessionPath() string {
	if p := os.Getenv("NETINFRA_SESSION"); p != "" {
		return p
	}
	return defaultSessionPath
}

type sessionFile struct {
	Topo  TopologySpec   `json:"topo"`
	Mode  string         `json:"mode"`
	Delay int64          `json:"delay_ns"`
	Down  map[string]bool `json:"down"`
}

func (c *Client) saveSession() error {
	path := SessionPath()
	dir := filepath.Dir(path)
	if dir != "" && dir != "." && dir != "/" {
		_ = os.MkdirAll(dir, 0o755)
	}
	sf := sessionFile{
		Topo:  c.topo,
		Mode:  c.mode.String(),
		Delay: int64(c.delay),
		Down:  c.down,
	}
	if sf.Down == nil {
		sf.Down = map[string]bool{}
	}
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (c *Client) clearSession() {
	_ = os.Remove(SessionPath())
}

// LoadSession restores AttachSession state from disk if present.
func (c *Client) LoadSession() error {
	data, err := os.ReadFile(SessionPath())
	if err != nil {
		return err
	}
	var sf sessionFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return fmt.Errorf("session file: %w", err)
	}
	mode, err := ParseMode(sf.Mode)
	if err != nil {
		return err
	}
	if err := sf.Topo.Validate(); err != nil {
		return err
	}
	c.AttachSession(sf.Topo, mode, time.Duration(sf.Delay))
	c.down = sf.Down
	if c.down == nil {
		c.down = map[string]bool{}
	}
	return nil
}
