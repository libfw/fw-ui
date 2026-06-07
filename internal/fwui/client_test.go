package fwui

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// fakeDaemon is a stand-in control socket. For each request line it calls
// handle(req, connNum) and either writes the returned response (+'\n') or, when
// drop is true, closes the connection without replying.
type fakeDaemon struct {
	path   string
	ln     net.Listener
	mu     sync.Mutex
	conns  int
	handle func(req map[string]any, connNum int) (resp string, drop bool)
}

func newFakeDaemon(t *testing.T, handle func(map[string]any, int) (string, bool)) *fakeDaemon {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "fwui")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "s")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	d := &fakeDaemon{path: path, ln: ln, handle: handle}
	go d.serve()
	t.Cleanup(func() { ln.Close(); os.RemoveAll(dir) })
	return d
}

func (d *fakeDaemon) serve() {
	for {
		conn, err := d.ln.Accept()
		if err != nil {
			return
		}
		d.mu.Lock()
		d.conns++
		n := d.conns
		d.mu.Unlock()
		go func(c net.Conn, connNum int) {
			defer c.Close()
			r := bufio.NewReader(c)
			for {
				line, err := r.ReadBytes('\n')
				if err != nil {
					return
				}
				var req map[string]any
				_ = json.Unmarshal(line, &req)
				resp, drop := d.handle(req, connNum)
				if drop {
					return
				}
				if _, err := c.Write([]byte(resp + "\n")); err != nil {
					return
				}
			}
		}(conn, n)
	}
}

// a handler that always returns canned, well-formed responses by command.
func cannedHandler(req map[string]any, _ int) (string, bool) {
	switch req["cmd"] {
	case "get_stats":
		return `{"ok":true,"acl":{"egress":{"allow":3,"deny":1,"bytes_allow":300,"bytes_deny":40,` +
			`"nonip":2},"ingress":{"allow":5,"deny":0,"bytes_allow":500,"bytes_deny":0,"nonip":0}},` +
			`"rules":[{"index":0,"hits":3}],"conntrack":{"capacity":4096,"live":2,"lookups":7,` +
			`"hits":4,"inserts":2}}`, false
	case "get_events":
		return `{"ok":true,"next":2,"events":[{"seq":0,"ts":1000,"dir":"egress","verdict":"allow",` +
			`"rule":0,"len":60,"family":4,"proto":6,"src":"10.0.0.1","dst":"8.8.8.8","sport":1234,` +
			`"dport":80}]}`, false
	case "get_rules":
		return `{"ok":true,"format":"json","source":"{\"default_action\":\"deny\"}"}`, false
	case "set_acl":
		if _, ok := req["json"].(string); !ok || req["json"] == "" {
			return `{"ok":false,"error":"set_acl requires a \"json\" string"}`, false
		}
		return `{"ok":true,"rules":2}`, false
	case "reload":
		return `{"ok":false,"error":"reload failed"}`, false
	case "reset_stats":
		return `{"ok":true}`, false
	}
	return `{"ok":false,"error":"unknown command"}`, false
}

func TestClientGetStats(t *testing.T) {
	d := newFakeDaemon(t, cannedHandler)
	c := New(d.path)
	defer c.Close()
	s, err := c.GetStats()
	if err != nil {
		t.Fatal(err)
	}
	if !s.OK || s.ACL.Egress.Allow != 3 || s.ACL.Egress.Deny != 1 || s.ACL.Ingress.Allow != 5 {
		t.Fatalf("acl stats: %+v", s.ACL)
	}
	if len(s.Rules) != 1 || s.Rules[0].Hits != 3 {
		t.Fatalf("rules: %+v", s.Rules)
	}
	if s.Conntrack.Capacity != 4096 || s.Conntrack.Live != 2 || s.Conntrack.Inserts != 2 {
		t.Fatalf("conntrack: %+v", s.Conntrack)
	}
}

func TestClientGetEvents(t *testing.T) {
	d := newFakeDaemon(t, cannedHandler)
	c := New(d.path)
	defer c.Close()
	ev, err := c.GetEvents(0)
	if err != nil {
		t.Fatal(err)
	}
	if !ev.OK || ev.Next != 2 || len(ev.Events) != 1 {
		t.Fatalf("events: %+v", ev)
	}
	e := ev.Events[0]
	if e.Verdict != "allow" || e.Dst != "8.8.8.8" || e.Dport != 80 || e.Rule != 0 {
		t.Fatalf("event: %+v", e)
	}
}

func TestClientGetRules(t *testing.T) {
	d := newFakeDaemon(t, cannedHandler)
	c := New(d.path)
	defer c.Close()
	r, err := c.GetRules()
	if err != nil {
		t.Fatal(err)
	}
	if !r.OK || r.Format != "json" || r.Source == nil || *r.Source == "" {
		t.Fatalf("rules: %+v", r)
	}
}

func TestClientSetACL(t *testing.T) {
	d := newFakeDaemon(t, cannedHandler)
	c := New(d.path)
	defer c.Close()
	ack, err := c.SetACL(`{"default_action":"allow"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !ack.OK || ack.Rules != 2 {
		t.Fatalf("ack: %+v", ack)
	}
	// an empty ruleset is rejected by the daemon (ok:false, no transport error)
	bad, err := c.SetACL("")
	if err != nil {
		t.Fatal(err)
	}
	if bad.OK || bad.Error == "" {
		t.Fatalf("expected ok:false with error, got %+v", bad)
	}
}

func TestClientReloadAndReset(t *testing.T) {
	d := newFakeDaemon(t, cannedHandler)
	c := New(d.path)
	defer c.Close()
	rl, err := c.Reload()
	if err != nil {
		t.Fatal(err)
	}
	if rl.OK || rl.Error == "" {
		t.Fatalf("reload: %+v", rl)
	}
	rs, err := c.ResetStats()
	if err != nil {
		t.Fatal(err)
	}
	if !rs.OK {
		t.Fatalf("reset: %+v", rs)
	}
}

func TestClientDialError(t *testing.T) {
	c := New("/tmp/fwui-nonexistent-xyz.sock")
	if _, err := c.GetStats(); err == nil {
		t.Fatal("expected dial error")
	}
}

func TestClientBadResponse(t *testing.T) {
	d := newFakeDaemon(t, func(map[string]any, int) (string, bool) { return "not json at all", false })
	c := New(d.path)
	defer c.Close()
	if _, err := c.GetStats(); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestClientReconnectAfterDrop(t *testing.T) {
	// The first connection is dropped without a reply (read error); the client
	// must reconnect and the second connection serves the response.
	d := newFakeDaemon(t, func(req map[string]any, connNum int) (string, bool) {
		if connNum == 1 {
			return "", true // drop
		}
		return cannedHandler(req, connNum)
	})
	c := New(d.path)
	defer c.Close()
	s, err := c.GetStats()
	if err != nil {
		t.Fatalf("reconnect should succeed: %v", err)
	}
	if !s.OK {
		t.Fatalf("stats: %+v", s)
	}
}

func TestClientPersistentDrop(t *testing.T) {
	// Every connection drops: after one reconnect attempt the call fails.
	d := newFakeDaemon(t, func(map[string]any, int) (string, bool) { return "", true })
	c := New(d.path)
	defer c.Close()
	if _, err := c.GetStats(); err == nil {
		t.Fatal("expected error when every connection drops")
	}
}

func TestRoundtripMarshalError(t *testing.T) {
	c := New("/tmp/whatever.sock")
	if _, err := c.roundtrip(make(chan int)); err == nil { // channels are not JSON-marshalable
		t.Fatal("expected marshal error")
	}
}

func TestRoundtripWriteErrorReconnect(t *testing.T) {
	// Establish a connection, then close the underlying socket while keeping the
	// reference: the next write fails, exercising the write-error reconnect path.
	d := newFakeDaemon(t, cannedHandler)
	c := New(d.path)
	defer c.Close()
	if _, err := c.GetStats(); err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	_ = c.c.Close()
	c.mu.Unlock()
	if _, err := c.GetStats(); err != nil {
		t.Fatalf("write-error reconnect should succeed: %v", err)
	}
}

func TestClientClose(t *testing.T) {
	c := New("/tmp/whatever.sock")
	if err := c.Close(); err != nil { // no connection yet
		t.Fatalf("close with no conn: %v", err)
	}
	d := newFakeDaemon(t, cannedHandler)
	c2 := New(d.path)
	if _, err := c2.GetStats(); err != nil {
		t.Fatal(err)
	}
	if err := c2.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}
