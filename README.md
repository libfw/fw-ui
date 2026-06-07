# fw-ui

A real-time web UI for the [libfw](https://github.com/libfw) firewall as exposed
by [`socket_vmnet`](https://github.com/tannevaled/socket_vmnet)'s control plane:
watch the ACL behave live (per-rule hit counts, allow/deny rates, a streaming
event log, connection-tracker state) and edit the ruleset on the fly.

> **Status: in progress.** The daemon-side control plane (a JSON UNIX socket,
> `socket_vmnet --control-socket`) and this repository's Go client of it are in
> place; the HTTP/SSE server and the browser frontend are being built next.

## Architecture

```
  socket_vmnet daemon                fw-ui (this repo)              browser
  ┌───────────────────┐  UNIX     ┌──────────────────────┐  HTTP  ┌─────────┐
  │ c-fw ACL+conntrack │  socket   │ control-socket client │  +SSE  │ graphs, │
  │  counters + event  │◀────────▶│  poller + aggregator  │◀──────▶│ events, │
  │  ring (control.c)  │   JSON    │  HTTP / SSE server    │        │ rules   │
  └───────────────────┘           └──────────────────────┘        └─────────┘
```

The daemon never speaks HTTP itself: it exposes a small line-delimited JSON
request/response protocol on a local UNIX socket (`get_stats`, `get_events`,
`get_rules`, `set_acl`, `reload`, `reset_stats`). fw-ui is the only process that
connects to it; it polls the counters to derive rates and tails the event ring.

The backend is a [Huma](https://huma.rocks) REST API (typed operations, an
auto-generated OpenAPI spec at `/openapi.json`, and a `/api/stream` server-sent
event feed) over the standard library router. The frontend is **Svelte +
DaisyUI** (Tailwind), built to static assets and embedded into the Go binary via
`go:embed` — so a single `fw-ui` binary serves everything. Live updates ride
SSE; rule edits are plain `PUT`/`POST` calls (no WebSocket).

## API

| method | path | purpose |
|--------|------|---------|
| GET | `/api/snapshot?since=` | latest stats + rate samples + recent events |
| GET | `/api/stream` | SSE stream of snapshots |
| GET | `/api/rules` | current ruleset source + format |
| PUT | `/api/acl` | compile + hot-swap a JSON ruleset |
| POST | `/api/reload` | re-read the daemon's `--acl` file |
| POST | `/api/reset` | zero the counters |

## Layout

- `internal/fwui` — the control-socket client (`Client`), the poller/aggregator
  (`Poller`) and the Huma API server (`Server`).
- `cmd/fw-ui` — the `fw-ui` CLI (cobra) and the embedded frontend (`static/`).
- `web/` — the Svelte + DaisyUI source; `npm run build` emits into
  `cmd/fw-ui/static/`.

## Build & test

```sh
go test ./...          # internal/ is held at 100% statement coverage
(cd web && npm install && npm run build)   # builds the frontend into cmd/fw-ui/static
CGO_ENABLED=0 go build ./cmd/fw-ui
./fw-ui --control-socket /var/run/socket_vmnet.control --listen 127.0.0.1:8849
```

CGO is not used; the result is a single static binary that serves both the API
and the frontend.

## License

BSD-3-Clause. See [LICENSE](LICENSE).
