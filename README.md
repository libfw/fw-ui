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
connects to it; it polls the counters to derive rates, tails the event ring, and
serves a lightweight browser UI (server-sent events for the live stream, plain
`fetch` POSTs for edits — no WebSocket dependency).

## Layout

- `internal/fwui` — the control-socket client (`Client`) and, soon, the poller
  and HTTP/SSE server.
- `cmd/fw-ui` — the `fw-ui` CLI (cobra): point it at a `--control-socket` and a
  `--listen` address.

## Build & test

```sh
go test ./...          # internal/ is held at 100% statement coverage
go build ./cmd/fw-ui
```

CGO is not used (`CGO_ENABLED=0`); the result is a single static binary.

## License

BSD-3-Clause. See [LICENSE](LICENSE).
