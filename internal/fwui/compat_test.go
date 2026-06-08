//go:build compat

// Cross-implementation test: drive the real socket_vmnet control plane (the C
// control.c, built as test/control_harness) with this package's Go client, to
// guarantee the wire protocol agrees end to end. Set FWUI_HARNESS to the built
// harness binary; the test skips if it is absent.
//
//	make -C ../socket_vmnet control-harness
//	FWUI_HARNESS=../socket_vmnet/test/control_harness go test -tags=compat ./internal/fwui
package fwui

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCompatRealControlPlane(t *testing.T) {
	harness := os.Getenv("FWUI_HARNESS")
	if harness == "" {
		t.Skip("FWUI_HARNESS not set")
	}
	if _, err := os.Stat(harness); err != nil {
		t.Skipf("harness %q: %v", harness, err)
	}

	dir, err := os.MkdirTemp("/tmp", "fwuicompat")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	sock := filepath.Join(dir, "s")
	aclf := filepath.Join(dir, "acl.json")

	cmd := exec.Command(harness, sock, aclf)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_ = cmd.Wait()
	}()

	ready := make(chan bool, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if strings.Contains(sc.Text(), "ready") {
				ready <- true
				return
			}
		}
		ready <- false
	}()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("harness exited before signalling ready")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for harness ready")
	}

	c := New(sock)
	defer c.Close()

	// stats (retry until the socket is accepting)
	var s *Stats
	for i := 0; i < 50; i++ {
		if s, err = c.GetStats(); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if !s.OK || len(s.Rules) != 1 || s.Rules[0].Hits == 0 {
		t.Fatalf("stats: %+v", s)
	}
	if s.ACL.Egress.Allow == 0 || s.ACL.Egress.Deny == 0 {
		t.Fatalf("egress counters: %+v", s.ACL.Egress)
	}

	// events: 3 allow + 2 deny, IPv4 dst 8.8.8.1
	ev, err := c.GetEvents(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.Events) != 5 {
		t.Fatalf("want 5 events, got %d", len(ev.Events))
	}
	allow, deny := 0, 0
	for _, e := range ev.Events {
		if e.Verdict == "allow" {
			allow++
		} else {
			deny++
		}
		if e.Dst != "8.8.8.1" {
			t.Fatalf("unexpected dst %q", e.Dst)
		}
	}
	if allow != 3 || deny != 2 {
		t.Fatalf("verdicts allow=%d deny=%d", allow, deny)
	}

	// rules round-trip: set_acl swaps, reload restores the file ruleset
	r, err := c.GetRules()
	if err != nil || r.Source == nil || !strings.Contains(*r.Source, "dst_port") {
		t.Fatalf("get_rules: %+v (err %v)", r, err)
	}
	ack, err := c.SetACL(`{"default_action":"allow"}`)
	if err != nil || !ack.OK || ack.Rules != 0 {
		t.Fatalf("set_acl: %+v (err %v)", ack, err)
	}
	if r2, _ := c.GetRules(); r2.Source == nil || strings.Contains(*r2.Source, "dst_port") {
		t.Fatalf("rules not swapped: %+v", r2)
	}
	rl, err := c.Reload()
	if err != nil || !rl.OK {
		t.Fatalf("reload: %+v (err %v)", rl, err)
	}
	if r3, _ := c.GetRules(); r3.Source == nil || !strings.Contains(*r3.Source, "dst_port") {
		t.Fatalf("reload did not restore file ruleset: %+v", r3)
	}
}
