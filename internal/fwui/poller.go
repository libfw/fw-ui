package fwui

import (
	"context"
	"sync"
	"time"
)

// Sample is a counter snapshot taken at a point in time. The daemon only knows
// running totals, so the poller records a series of these to drive rate graphs.
type Sample struct {
	TS    int64 `json:"ts"` // unix milliseconds
	Stats Stats `json:"stats"`
}

// Snapshot is the aggregated view served to the UI.
type Snapshot struct {
	Connected bool     `json:"connected"`
	Error     string   `json:"error,omitempty"`
	Latest    *Stats   `json:"latest,omitempty"`
	Samples   []Sample `json:"samples"`
	Events    []Event  `json:"events"`
	NextSeq   uint64   `json:"next_seq"`
}

// Poller periodically reads the daemon counters + event ring and keeps a
// bounded history for the API. Safe for concurrent readers.
type Poller struct {
	c        *Client
	interval time.Duration
	maxSamp  int
	maxEvent int

	mu        sync.Mutex
	samples   []Sample
	events    []Event
	latest    *Stats
	lastSeq   uint64
	connected bool
	lastErr   string
	now       func() time.Time // injectable clock for tests
}

// NewPoller polls `c` every `interval`, retaining `maxSamp` samples and
// `maxEvent` recent events.
func NewPoller(c *Client, interval time.Duration, maxSamp, maxEvent int) *Poller {
	if maxSamp < 1 {
		maxSamp = 1
	}
	if maxEvent < 1 {
		maxEvent = 1
	}
	return &Poller{c: c, interval: interval, maxSamp: maxSamp, maxEvent: maxEvent,
		now: time.Now}
}

// PollOnce performs a single stats+events read and folds it into the history.
func (p *Poller) PollOnce() {
	stats, err := p.c.GetStats()
	if err != nil {
		p.mu.Lock()
		p.connected = false
		p.lastErr = err.Error()
		p.mu.Unlock()
		return
	}
	p.mu.Lock()
	since := p.lastSeq
	p.mu.Unlock()

	ev, err := p.c.GetEvents(since)

	p.mu.Lock()
	defer p.mu.Unlock()
	p.connected = true
	p.lastErr = ""
	p.latest = stats
	p.samples = append(p.samples, Sample{TS: p.now().UnixMilli(), Stats: *stats})
	if len(p.samples) > p.maxSamp {
		p.samples = p.samples[len(p.samples)-p.maxSamp:]
	}
	if err == nil && ev != nil {
		p.lastSeq = ev.Next
		if len(ev.Events) > 0 {
			p.events = append(p.events, ev.Events...)
			if len(p.events) > p.maxEvent {
				p.events = p.events[len(p.events)-p.maxEvent:]
			}
		}
	}
}

// Run polls until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	t := time.NewTicker(p.interval)
	defer t.Stop()
	p.PollOnce()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.PollOnce()
		}
	}
}

// Snapshot returns a copy of the current aggregated state. `sinceSeq` trims the
// returned events to those with seq >= sinceSeq (0 = all retained).
func (p *Poller) Snapshot(sinceSeq uint64) Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := Snapshot{
		Connected: p.connected,
		Error:     p.lastErr,
		Latest:    p.latest,
		NextSeq:   p.lastSeq,
		Samples:   append([]Sample(nil), p.samples...),
	}
	for _, e := range p.events {
		if e.Seq >= sinceSeq {
			s.Events = append(s.Events, e)
		}
	}
	return s
}
