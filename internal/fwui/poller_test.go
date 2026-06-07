package fwui

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestPollerBasic(t *testing.T) {
	d := newFakeDaemon(t, cannedHandler)
	p := NewPoller(New(d.path), time.Hour, 10, 10)
	p.PollOnce()
	s := p.Snapshot(0)
	if !s.Connected || s.Error != "" {
		t.Fatalf("expected connected, got %+v", s)
	}
	if s.Latest == nil || s.Latest.ACL.Egress.Allow != 3 {
		t.Fatalf("latest: %+v", s.Latest)
	}
	if len(s.Samples) != 1 || len(s.Events) != 1 || s.Events[0].Seq != 0 {
		t.Fatalf("samples=%d events=%d", len(s.Samples), len(s.Events))
	}
	if s.NextSeq != 2 {
		t.Fatalf("next seq: %d", s.NextSeq)
	}
}

func TestPollerDisconnected(t *testing.T) {
	p := NewPoller(New("/tmp/fwui-none.sock"), time.Hour, 10, 10)
	p.PollOnce()
	s := p.Snapshot(0)
	if s.Connected || s.Error == "" || len(s.Samples) != 0 {
		t.Fatalf("expected disconnected, got %+v", s)
	}
}

func TestPollerClampAndTrim(t *testing.T) {
	d := newFakeDaemon(t, cannedHandler)
	p := NewPoller(New(d.path), time.Hour, 0, 0) // clamped to 1/1
	p.PollOnce()
	p.PollOnce()
	p.PollOnce()
	s := p.Snapshot(0)
	if len(s.Samples) != 1 || len(s.Events) != 1 {
		t.Fatalf("ring not trimmed: samples=%d events=%d", len(s.Samples), len(s.Events))
	}
}

func TestPollerEventsError(t *testing.T) {
	// get_stats succeeds, get_events returns garbage: a sample is still recorded
	// but no events, and the connection counts as up.
	d := newFakeDaemon(t, func(req map[string]any, _ int) (string, bool) {
		if req["cmd"] == "get_events" {
			return "garbage not json", false
		}
		return cannedHandler(req, 0)
	})
	p := NewPoller(New(d.path), time.Hour, 10, 10)
	p.PollOnce()
	s := p.Snapshot(0)
	if !s.Connected || len(s.Samples) != 1 || len(s.Events) != 0 || s.NextSeq != 0 {
		t.Fatalf("snapshot: %+v", s)
	}
}

// seqDaemon serves get_events with a fresh, increasing seq on every poll.
type seqState struct {
	mu sync.Mutex
	n  uint64
}

func TestPollerSnapshotSince(t *testing.T) {
	st := &seqState{}
	d := newFakeDaemon(t, func(req map[string]any, _ int) (string, bool) {
		if req["cmd"] == "get_events" {
			st.mu.Lock()
			n := st.n
			st.n++
			st.mu.Unlock()
			return fmt.Sprintf(`{"ok":true,"next":%d,"events":[{"seq":%d,"ts":1,"dir":"egress",`+
				`"verdict":"deny","rule":-1,"len":40,"family":4,"proto":6}]}`, n+1, n), false
		}
		return cannedHandler(req, 0)
	})
	p := NewPoller(New(d.path), time.Hour, 10, 10)
	p.PollOnce() // seq 0
	p.PollOnce() // seq 1
	p.PollOnce() // seq 2
	if all := p.Snapshot(0); len(all.Events) != 3 {
		t.Fatalf("want 3 events, got %d", len(all.Events))
	}
	since := p.Snapshot(2)
	if len(since.Events) != 1 || since.Events[0].Seq != 2 {
		t.Fatalf("since=2 should give [seq2], got %+v", since.Events)
	}
}

func TestPollerRun(t *testing.T) {
	d := newFakeDaemon(t, cannedHandler)
	p := NewPoller(New(d.path), time.Millisecond, 100, 100)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.Run(ctx); close(done) }()
	// let a few ticks happen, then stop.
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancel")
	}
	if !p.Snapshot(0).Connected {
		t.Fatal("expected at least one successful poll")
	}
}
