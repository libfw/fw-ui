package fwui

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/danielgtaylor/huma/v2/sse"
)

// Server exposes the poller + control client over a Huma REST API (with an SSE
// live stream) and serves the embedded frontend.
type Server struct {
	p              *Poller
	c              *Client
	streamInterval time.Duration
	mux            *http.ServeMux
	api            huma.API
}

// NewServer builds the API on a fresh mux and mounts `static` (may be nil) at /.
func NewServer(p *Poller, c *Client, static fs.FS) *Server {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("fw-ui", "0.1.0"))
	s := &Server{p: p, c: c, streamInterval: time.Second, mux: mux, api: api}
	addRoutes(api, s)
	if static != nil {
		mux.Handle("/", http.FileServer(http.FS(static)))
	}
	return s
}

// Handler returns the root HTTP handler (API + static).
func (s *Server) Handler() http.Handler { return s.mux }

type snapshotOutput struct{ Body Snapshot }
type rulesOutput struct{ Body Rules }
type ackOutput struct{ Body Ack }

type sinceInput struct {
	Since uint64 `query:"since" doc:"only events with seq >= since"`
}
type setACLInput struct {
	Body struct {
		JSON string `json:"json" doc:"the full ACL ruleset as JSON"`
	}
}

func gatewayErr(err error) error {
	return huma.NewError(http.StatusBadGateway, "firewall daemon unreachable", err)
}

func addRoutes(api huma.API, s *Server) {
	huma.Register(api, huma.Operation{
		OperationID: "get-snapshot", Method: http.MethodGet, Path: "/api/snapshot",
		Summary: "Aggregated counters, rate samples and recent events",
	}, func(_ context.Context, in *sinceInput) (*snapshotOutput, error) {
		return &snapshotOutput{Body: s.p.Snapshot(in.Since)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-rules", Method: http.MethodGet, Path: "/api/rules",
		Summary: "Current ruleset source text",
	}, func(_ context.Context, _ *struct{}) (*rulesOutput, error) {
		r, err := s.c.GetRules()
		if err != nil {
			return nil, gatewayErr(err)
		}
		return &rulesOutput{Body: *r}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "set-acl", Method: http.MethodPut, Path: "/api/acl",
		Summary: "Compile and hot-swap the ruleset (JSON)",
	}, func(_ context.Context, in *setACLInput) (*ackOutput, error) {
		a, err := s.c.SetACL(in.Body.JSON)
		if err != nil {
			return nil, gatewayErr(err)
		}
		return &ackOutput{Body: *a}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "reload", Method: http.MethodPost, Path: "/api/reload",
		Summary: "Re-read the daemon's --acl file",
	}, func(_ context.Context, _ *struct{}) (*ackOutput, error) {
		a, err := s.c.Reload()
		if err != nil {
			return nil, gatewayErr(err)
		}
		return &ackOutput{Body: *a}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "reset-stats", Method: http.MethodPost, Path: "/api/reset",
		Summary: "Zero the counters",
	}, func(_ context.Context, _ *struct{}) (*ackOutput, error) {
		a, err := s.c.ResetStats()
		if err != nil {
			return nil, gatewayErr(err)
		}
		return &ackOutput{Body: *a}, nil
	})

	sse.Register(api, huma.Operation{
		OperationID: "stream", Method: http.MethodGet, Path: "/api/stream",
		Summary: "Server-sent stream of snapshots",
	}, map[string]any{"snapshot": Snapshot{}},
		func(ctx context.Context, _ *struct{}, send sse.Sender) {
			streamLoop(ctx, s.p, s.streamInterval, func(v any) error { return send.Data(v) })
		})
}

// streamLoop pushes a snapshot immediately and then on every tick, stopping when
// the context is cancelled or a send fails (the client went away). Extracted
// from the SSE handler so the disconnect branches are unit-testable.
func streamLoop(ctx context.Context, p *Poller, interval time.Duration, send func(any) error) {
	if err := send(p.Snapshot(0)); err != nil {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := send(p.Snapshot(0)); err != nil {
				return
			}
		}
	}
}
