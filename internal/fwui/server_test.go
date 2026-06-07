package fwui

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"
)

// testAPI builds a humatest API with the routes registered against server s.
func testAPI(t *testing.T, s *Server) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	addRoutes(api, s)
	return api
}

func TestServerSnapshot(t *testing.T) {
	d := newFakeDaemon(t, cannedHandler)
	p := NewPoller(New(d.path), time.Hour, 10, 10)
	p.PollOnce()
	api := testAPI(t, &Server{p: p})
	resp := api.Get("/api/snapshot?since=0")
	if resp.Code != http.StatusOK {
		t.Fatalf("status %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"connected":true`) || !strings.Contains(body, `"allow":3`) {
		t.Fatalf("snapshot body: %s", body)
	}
}

func TestServerRulesSetReloadReset(t *testing.T) {
	d := newFakeDaemon(t, cannedHandler)
	c := New(d.path)
	api := testAPI(t, &Server{c: c})

	if r := api.Get("/api/rules"); r.Code != http.StatusOK ||
		!strings.Contains(r.Body.String(), `"format":"json"`) {
		t.Fatalf("rules: %d %s", r.Code, r.Body.String())
	}
	if r := api.Put("/api/acl", map[string]any{"json": `{"default_action":"allow"}`}); r.Code !=
		http.StatusOK || !strings.Contains(r.Body.String(), `"rules":2`) {
		t.Fatalf("set_acl: %d %s", r.Code, r.Body.String())
	}
	if r := api.Post("/api/reload", struct{}{}); r.Code != http.StatusOK {
		t.Fatalf("reload: %d %s", r.Code, r.Body.String())
	}
	if r := api.Post("/api/reset", struct{}{}); r.Code != http.StatusOK ||
		!strings.Contains(r.Body.String(), `"ok":true`) {
		t.Fatalf("reset: %d %s", r.Code, r.Body.String())
	}
}

func TestServerGatewayErrors(t *testing.T) {
	c := New("/tmp/fwui-none-xyz.sock") // never connects
	api := testAPI(t, &Server{c: c})
	for _, tc := range []struct {
		name string
		do   func() *httptest.ResponseRecorder
	}{
		{"rules", func() *httptest.ResponseRecorder { return api.Get("/api/rules") }},
		{"acl", func() *httptest.ResponseRecorder { return api.Put("/api/acl", map[string]any{"json": "{}"}) }},
		{"reload", func() *httptest.ResponseRecorder { return api.Post("/api/reload", struct{}{}) }},
		{"reset", func() *httptest.ResponseRecorder { return api.Post("/api/reset", struct{}{}) }},
	} {
		if r := tc.do(); r.Code != http.StatusBadGateway {
			t.Fatalf("%s: want 502, got %d (%s)", tc.name, r.Code, r.Body.String())
		}
	}
}

func TestStreamLoop(t *testing.T) {
	d := newFakeDaemon(t, cannedHandler)
	p := NewPoller(New(d.path), time.Hour, 10, 10)
	p.PollOnce()

	// (1) initial send fails -> returns immediately.
	calls := 0
	streamLoop(context.Background(), p, time.Hour, func(any) error {
		calls++
		return context.Canceled // any error
	})
	if calls != 1 {
		t.Fatalf("initial-error: want 1 send, got %d", calls)
	}

	// (2) initial send ok, then context cancelled -> ctx.Done branch.
	ctx, cancel := context.WithCancel(context.Background())
	calls = 0
	go func() { time.Sleep(10 * time.Millisecond); cancel() }()
	streamLoop(ctx, p, time.Hour, func(any) error { calls++; return nil })
	if calls != 1 {
		t.Fatalf("cancel: want 1 send, got %d", calls)
	}

	// (3) initial ok, a tick fires, the tick send fails -> ticker-error branch.
	calls = 0
	streamLoop(context.Background(), p, time.Millisecond, func(any) error {
		calls++
		if calls >= 2 {
			return context.Canceled
		}
		return nil
	})
	if calls != 2 {
		t.Fatalf("tick-error: want 2 sends, got %d", calls)
	}
}

func TestServerStaticAndSSE(t *testing.T) {
	d := newFakeDaemon(t, cannedHandler)
	p := NewPoller(New(d.path), time.Hour, 10, 10)
	p.PollOnce()
	static := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<h1>fw-ui</h1>")}}
	srv := NewServer(p, New(d.path), static)
	srv.streamInterval = 5 * time.Millisecond
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// static index
	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 64)
	n, _ := res.Body.Read(b)
	res.Body.Close()
	if !strings.Contains(string(b[:n]), "fw-ui") {
		t.Fatalf("static index: %q", string(b[:n]))
	}

	// SSE stream: read until we see a snapshot event, then cancel.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/stream", nil)
	sr, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer sr.Body.Close()
	sc := bufio.NewScanner(sr.Body)
	got := false
	for sc.Scan() {
		if strings.Contains(sc.Text(), "connected") {
			got = true
			break
		}
	}
	cancel()
	if !got {
		t.Fatal("did not receive a snapshot over SSE")
	}
}
