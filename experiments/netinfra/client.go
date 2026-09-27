package netinfra

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// DefaultUnixSocket is the preferred localhost transport on the experiment VM.
	DefaultUnixSocket = "/tmp/mininetd.sock"
	// DefaultTCPAddr is used when the Unix socket is unavailable (or forced).
	DefaultTCPAddr = "127.0.0.1:17300"
)

// ClientOptions configures the JSON-RPC client.
type ClientOptions struct {
	// Addr is a unix path (containing '/') or host:port TCP address.
	// Empty uses MININETD_ADDR, then DefaultUnixSocket.
	Addr string
	// DialTimeout bounds connection establishment.
	DialTimeout time.Duration
}

// Client is a JSON-RPC Controller backed by mininetd.
type Client struct {
	opts ClientOptions

	mu     sync.Mutex
	conn   net.Conn
	reader *bufio.Reader
	nextID atomic.Uint64

	topo   TopologySpec
	mode   Mode
	delay  time.Duration
	active bool
	// down tracks currently failed links by linkKey (a--b sorted). Empty after Start.
	down map[string]bool
}

// NewClient returns a Controller that talks to mininetd.
func NewClient(opts ClientOptions) *Client {
	if opts.DialTimeout == 0 {
		opts.DialTimeout = 5 * time.Second
	}
	if opts.Addr == "" {
		opts.Addr = os.Getenv("MININETD_ADDR")
	}
	if opts.Addr == "" {
		opts.Addr = DefaultUnixSocket
	}
	return &Client{opts: opts}
}

func (c *Client) dial() error {
	if c.conn != nil {
		return nil
	}
	addr := c.opts.Addr
	network := "unix"
	if !isUnixAddr(addr) {
		network = "tcp"
	}
	conn, err := net.DialTimeout(network, addr, c.opts.DialTimeout)
	if err != nil {
		// Fall back to TCP if the default unix socket is missing.
		if network == "unix" && addr == DefaultUnixSocket {
			conn, err = net.DialTimeout("tcp", DefaultTCPAddr, c.opts.DialTimeout)
			if err != nil {
				return fmt.Errorf("dial mininetd (%s or %s): %w", addr, DefaultTCPAddr, err)
			}
			c.opts.Addr = DefaultTCPAddr
		} else {
			return fmt.Errorf("dial mininetd %s://%s: %w", network, addr, err)
		}
	}
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	return nil
}

func isUnixAddr(addr string) bool {
	// Unix paths contain '/' (or '\'); TCP is host:port with neither.
	for i := 0; i < len(addr); i++ {
		if addr[i] == '/' || addr[i] == '\\' {
			return true
		}
	}
	return len(addr) > 0 && addr[0] == '@' // abstract Linux namespace
}

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      uint64      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *Client) call(ctx context.Context, method string, params interface{}, result interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.dial(); err != nil {
		return err
	}

	id := c.nextID.Add(1)
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')

	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetDeadline(deadline)
		defer c.conn.SetDeadline(time.Time{})
	}

	if _, err := c.conn.Write(payload); err != nil {
		c.resetConn()
		return fmt.Errorf("rpc write %s: %w", method, err)
	}

	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		c.resetConn()
		return fmt.Errorf("rpc read %s: %w", method, err)
	}
	var resp rpcResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return fmt.Errorf("rpc decode %s: %w", method, err)
	}
	if resp.Error != nil {
		return fmt.Errorf("mininetd %s: %s", method, resp.Error.Message)
	}
	if result != nil && len(resp.Result) > 0 && string(resp.Result) != "null" {
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return fmt.Errorf("rpc result %s: %w", method, err)
		}
	}
	return nil
}

func (c *Client) resetConn() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = nil
	c.reader = nil
}

// Start implements Controller.
func (c *Client) Start(ctx context.Context, topo TopologySpec, mode Mode, delay time.Duration) error {
	if err := topo.Validate(); err != nil {
		return err
	}
	params := map[string]interface{}{
		"topo":     topo,
		"mode":     mode.String(),
		"delay_ms": delay.Seconds() * 1000,
	}
	if err := c.call(ctx, "Start", params, nil); err != nil {
		return err
	}
	c.topo = topo
	c.mode = mode
	c.delay = delay
	c.active = true
	c.down = map[string]bool{}
	_ = c.saveSession()
	return nil
}

// SetLink implements Controller.
func (c *Client) SetLink(a, b string, up bool) error {
	params := map[string]interface{}{"a": a, "b": b, "up": up}
	if err := c.call(context.Background(), "SetLink", params, nil); err != nil {
		return err
	}
	if c.down == nil {
		c.down = map[string]bool{}
	}
	key := linkKey(a, b)
	if up {
		delete(c.down, key)
	} else {
		c.down[key] = true
	}
	if c.active {
		_ = c.saveSession()
	}
	return nil
}

// SetDelay implements Controller.
func (c *Client) SetDelay(delay time.Duration) error {
	params := map[string]interface{}{"delay_ms": delay.Seconds() * 1000}
	if err := c.call(context.Background(), "SetDelay", params, nil); err != nil {
		return err
	}
	c.delay = delay
	if c.active {
		_ = c.saveSession()
	}
	return nil
}

// SetMode implements Controller.
func (c *Client) SetMode(mode Mode) error {
	params := map[string]interface{}{"mode": mode.String()}
	if err := c.call(context.Background(), "SetMode", params, nil); err != nil {
		return err
	}
	c.mode = mode
	if c.active {
		_ = c.saveSession()
	}
	return nil
}

// ExecOn implements Controller.
func (c *Client) ExecOn(node string, cmd string) (string, error) {
	params := map[string]interface{}{"node": node, "cmd": cmd}
	var out struct {
		Stdout string `json:"stdout"`
		Code   int    `json:"code"`
	}
	if err := c.call(context.Background(), "Exec", params, &out); err != nil {
		return out.Stdout, err
	}
	if out.Code != 0 {
		return out.Stdout, fmt.Errorf("exec on %s exited %d: %s", node, out.Code, out.Stdout)
	}
	return out.Stdout, nil
}

// Close implements Controller.
func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	err := c.call(ctx, "Stop", map[string]interface{}{}, nil)
	c.active = false
	c.clearSession()
	c.mu.Lock()
	c.resetConn()
	c.mu.Unlock()
	return err
}

// InjectPlannedFailures implements Controller.
func (c *Client) InjectPlannedFailures() error {
	for _, pair := range c.topo.PlannedFailures() {
		if err := c.SetLink(pair[0], pair[1], false); err != nil {
			return fmt.Errorf("inject %s--%s: %w", pair[0], pair[1], err)
		}
	}
	return nil
}

// InjectAroundLeader implements Controller.
func (c *Client) InjectAroundLeader(actualLeader string) error {
	remapped := c.topo.RemapLeader(actualLeader)
	routes := make([]map[string]string, 0, len(remapped.ForwardingRoutes))
	for _, r := range remapped.ForwardingRoutes {
		routes = append(routes, map[string]string{
			"node": r.Node,
			"dest": r.Dest,
			"via":  r.Via,
		})
	}
	if err := c.call(context.Background(), "SetForwardingRoutes", map[string]interface{}{
		"routes": routes,
	}, nil); err != nil {
		return fmt.Errorf("set forwarding routes: %w", err)
	}
	c.topo.ForwardingRoutes = remapped.ForwardingRoutes
	for _, pair := range remapped.PlannedFailures() {
		if err := c.SetLink(pair[0], pair[1], false); err != nil {
			return fmt.Errorf("inject %s--%s (around %s): %w", pair[0], pair[1], actualLeader, err)
		}
	}
	return nil
}

// ActiveTopo returns the last started topology (empty if none).
func (c *Client) ActiveTopo() TopologySpec { return c.topo }

// ActiveMode returns the last applied mode.
func (c *Client) ActiveMode() Mode { return c.mode }

// ActiveDelay returns the last configured per-link delay.
func (c *Client) ActiveDelay() time.Duration { return c.delay }

// IsActive reports whether Start succeeded without Close.
func (c *Client) IsActive() bool { return c.active }

// AttachSession restores client-side topology/mode/delay for Validate/Inject
// when talking to an already-running daemon (e.g. a fresh netctl process).
func (c *Client) AttachSession(topo TopologySpec, mode Mode, delay time.Duration) {
	c.topo = topo
	c.mode = mode
	c.delay = delay
	c.active = true
	if c.down == nil {
		c.down = map[string]bool{}
	}
}
