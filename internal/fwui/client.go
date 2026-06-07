// Package fwui talks to a socket_vmnet firewall daemon over its --control-socket
// JSON control plane and serves a real-time web UI on top of it.
package fwui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// DirStats are the per-direction ACL counters.
type DirStats struct {
	Allow      uint64 `json:"allow"`
	Deny       uint64 `json:"deny"`
	BytesAllow uint64 `json:"bytes_allow"`
	BytesDeny  uint64 `json:"bytes_deny"`
	Nonip      uint64 `json:"nonip"`
}

// RuleStat is the cumulative hit count of one ACL rule.
type RuleStat struct {
	Index int    `json:"index"`
	Hits  uint64 `json:"hits"`
}

// ConntrackStats is a snapshot of the connection tracker.
type ConntrackStats struct {
	Capacity uint64 `json:"capacity"`
	Live     uint64 `json:"live"`
	Lookups  uint64 `json:"lookups"`
	Hits     uint64 `json:"hits"`
	Inserts  uint64 `json:"inserts"`
}

// Stats is the get_stats response.
type Stats struct {
	OK  bool `json:"ok"`
	ACL struct {
		Egress  DirStats `json:"egress"`
		Ingress DirStats `json:"ingress"`
	} `json:"acl"`
	Rules     []RuleStat     `json:"rules"`
	Conntrack ConntrackStats `json:"conntrack"`
}

// Event is one allow/deny decision recorded by the data path.
type Event struct {
	Seq     uint64 `json:"seq"`
	TS      uint64 `json:"ts"`
	Dir     string `json:"dir"`
	Verdict string `json:"verdict"`
	Rule    int    `json:"rule"`
	Len     uint64 `json:"len"`
	Family  int    `json:"family"`
	Proto   int    `json:"proto"`
	Src     string `json:"src,omitempty"`
	Dst     string `json:"dst,omitempty"`
	Sport   int    `json:"sport,omitempty"`
	Dport   int    `json:"dport,omitempty"`
}

// Events is the get_events response.
type Events struct {
	OK     bool    `json:"ok"`
	Next   uint64  `json:"next"`
	Events []Event `json:"events"`
}

// Rules is the get_rules response (Source is nil when no ACL is loaded).
type Rules struct {
	OK     bool    `json:"ok"`
	Format string  `json:"format"`
	Source *string `json:"source"`
}

// Ack is the response to a mutating command (set_acl / reload / reset_stats).
type Ack struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
	Rules int    `json:"rules"`
}

// Client is a connection to the daemon's control socket. It keeps a single
// connection (the daemon serves one at a time) and transparently reconnects.
// Methods are safe for concurrent use.
type Client struct {
	addr    string
	timeout time.Duration

	mu sync.Mutex
	c  net.Conn
	r  *bufio.Reader
}

// New returns a client for the control socket at `addr` (a UNIX socket path).
func New(addr string) *Client { return &Client{addr: addr, timeout: 5 * time.Second} }

// Close drops the current connection (a later call transparently reconnects).
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dropLocked()
}

func (c *Client) dropLocked() error {
	if c.c == nil {
		return nil
	}
	err := c.c.Close()
	c.c = nil
	c.r = nil
	return err
}

func (c *Client) dialLocked() error {
	conn, err := net.DialTimeout("unix", c.addr, c.timeout)
	if err != nil {
		return err
	}
	c.c = conn
	c.r = bufio.NewReaderSize(conn, 64*1024)
	return nil
}

// roundtrip marshals req, sends it as one line and reads one response line. On a
// connection error it reconnects once and retries, so a daemon restart between
// calls is transparent.
func (c *Client) roundtrip(req any) ([]byte, error) {
	line, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	line = append(line, '\n')

	c.mu.Lock()
	defer c.mu.Unlock()
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if c.c == nil {
			if err := c.dialLocked(); err != nil {
				return nil, err
			}
		}
		if c.timeout > 0 {
			_ = c.c.SetDeadline(time.Now().Add(c.timeout))
		}
		if _, err := c.c.Write(line); err != nil {
			lastErr = err
			c.dropLocked()
			continue
		}
		resp, err := c.r.ReadBytes('\n')
		if err != nil {
			lastErr = err
			c.dropLocked()
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("control: %w", lastErr)
}

func call[T any](c *Client, req any) (*T, error) {
	resp, err := c.roundtrip(req)
	if err != nil {
		return nil, err
	}
	var out T
	if err := json.Unmarshal(resp, &out); err != nil {
		return nil, fmt.Errorf("control: bad response: %w", err)
	}
	return &out, nil
}

// GetStats fetches the ACL + conntrack counters.
func (c *Client) GetStats() (*Stats, error) {
	return call[Stats](c, map[string]any{"cmd": "get_stats"})
}

// GetEvents fetches recorded decisions with sequence >= since.
func (c *Client) GetEvents(since uint64) (*Events, error) {
	return call[Events](c, map[string]any{"cmd": "get_events", "since": since})
}

// GetRules fetches the current ruleset source text + format.
func (c *Client) GetRules() (*Rules, error) {
	return call[Rules](c, map[string]any{"cmd": "get_rules"})
}

// SetACL compiles and hot-swaps a JSON ruleset.
func (c *Client) SetACL(rulesetJSON string) (*Ack, error) {
	return call[Ack](c, map[string]any{"cmd": "set_acl", "json": rulesetJSON})
}

// Reload re-reads the daemon's --acl file.
func (c *Client) Reload() (*Ack, error) {
	return call[Ack](c, map[string]any{"cmd": "reload"})
}

// ResetStats zeroes the counters.
func (c *Client) ResetStats() (*Ack, error) {
	return call[Ack](c, map[string]any{"cmd": "reset_stats"})
}
