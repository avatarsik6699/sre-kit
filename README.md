# sre-kit

A self-hosted SRE/observability aggregator for solo developers and small teams running a handful
of VPS. One unified data model, one live dashboard, one place to configure alerts — instead of
juggling separate tools for metrics, uptime checks, and log-based events.

## Why

Running 1–10 VPS usually means stitching together 4–6 separate dashboards to answer "is everything
okay right now": a metrics tool, an uptime checker, container logs, `fail2ban` checked by hand over
SSH, a certificate-expiry script. Each has its own data model, its own UI, its own alerting setup.

sre-kit normalizes all of that into four entities — **Metric**, **Check**, **Event**, and the
**Alert**s derived from them — collected over plain SSH (no agent to install on the monitored
host) and shown on one live-updating dashboard, with a single alert-rule engine notifying you over
Telegram.

## Features

- **Unified data contract** (`Metric` / `Check` / `Event` / `Alert`) — a small, versioned,
  additive-only NDJSON schema (`internal/contract/contract.schema.json`) that any adapter, in any
  language, can emit against.
- **`host-metrics-ssh`** — CPU / RAM / disk usage sampled over SSH, no agent required on the
  target host.
- **`uptime-http`** — HTTP/TCP reachability checks with TLS certificate-expiry tracking.
- **`fail2ban-ssh`** — ban/unban activity from a remote host's `fail2ban` log, normalized to
  `Event`s.
- **Live dashboard** — Sources, Dashboard, and Source-detail views pushed over WebSocket; new data
  is visible within seconds of an adapter emitting it.
- **Alert router** — rule-based evaluation over Metric/Check/Event with a full firing → resolved
  lifecycle, delivered to Telegram.
- **Single static binary** — Go backend, SQLite storage, one Docker container (or a plain binary +
  systemd unit) as the deploy target.

## Architecture at a glance

```
adapters/<name>/        subprocess binaries speaking NDJSON over stdio (language-agnostic)
internal/adapterengine/ spawns/schedules adapters, validates NDJSON against the contract
internal/telemetry/     Metric / Check / Event ingestion and query
internal/alertrouter/   rule evaluation, firing/resolved lifecycle, notification channels
internal/sources/       configured adapter instances (what to run, with what config)
internal/platform/      shared kernel: config, db, secrets, http server, websocket hub
web/                    React + TanStack Start + Mantine UI frontend
```

Each adapter is an independent process: given resolved config on stdin, it emits NDJSON lines
(`metric` / `check` / `event`) on stdout and exits non-zero on failure to reach its target — that's
the entire contract. Writing a new source means writing a new adapter, not touching the core.

### Relationship with infraegev2

[`infraegev2`](https://github.com/avatarsik6699/infraegev2) is this project's first-party reference
deployment, not an unrelated external consumer. sre-kit owns the observability core, adapters,
source configuration, presets and observability deployment automation. infraegev2 owns its
application telemetry, VPS/network prerequisites and application Compose stack. Work crossing that
boundary is tracked by coordinated SDD changes in both repositories; the retired infraegev2
`apps/ops` dashboard is not a competing implementation.

See [`docs/SPEC.md`](docs/SPEC.md) for the full technical specification (data model, API
contract, auth model, roadmap) and [`docs/STACK.md`](docs/STACK.md) for concrete stack details,
module layout, and conventions.

## Getting started

### Prerequisites

- Go 1.26+
- Node.js + `pnpm` (frontend)
- A Linux host reachable over SSH, for `host-metrics-ssh` / `fail2ban-ssh`

### Run locally

```bash
git clone git@github.com:avatarsik6699/sre-kit.git
cd sre-kit

cp .env.example .env
# generate a secrets-store encryption key and set SRE_KIT_SECRETS_KEY in .env
openssl rand -hex 32

go mod download
go run ./cmd/server
```

The server serves both the API and the built frontend from one process
(`SRE_KIT_ADDR`, default `:8080`).

For frontend development with hot reload:

```bash
cd web
pnpm install
pnpm dev
```

### Run with Docker

```bash
docker build -t sre-kit .
docker run -p 8080:8080 \
  -e SRE_KIT_SECRETS_KEY=$(openssl rand -hex 32) \
  -v sre-kit-data:/app/data \
  sre-kit
```

### Adding a source

Once the server is running, add a source (e.g. `host-metrics-ssh`) through the UI with the
target host's SSH credentials — no manual exporter setup or agent install required on the
monitored host. See `docs/SPEC.md` §3/§6 for the config and secrets model.

## Writing a new adapter

An adapter is any executable that:

1. Reads its resolved JSON config from stdin.
2. Emits NDJSON lines on stdout, each conforming to
   [`contract.schema.json`](internal/contract/contract.schema.json) (`metric`, `check`, or
   `event`).
3. Exits non-zero only on genuine failure to reach its target, so the core marks the source
   `unreachable`.

See `adapters/host-metrics-ssh`, `adapters/uptime-http`, and `adapters/fail2ban-ssh` for
reference implementations, and each adapter's `manifest.json` for the config-schema shape
(including how secret fields are marked and resolved).

## Project status

v1 (MVP) is feature-complete: unified contract, three SSH/HTTP-based adapters, live dashboard,
and Telegram alerting. See [`docs/SPEC.md`](docs/SPEC.md) §9 for the roadmap and
[`docs/changes/archive/`](docs/changes/archive/) for the history of how each piece was built.

## Development workflow

This project is built through a spec-driven development (SDD) process: `docs/SPEC.md` is the
product contract, and each unit of work is tracked as a `docs/changes/NN-slug.md` file carrying
its own backlog, files-touched list, and review notes, archived once shipped. The rules for that
process live in [`AGENTS.md`](AGENTS.md); see [`CONTRIBUTING.md`](CONTRIBUTING.md) for how to
propose and land a change.

## License

[MIT](LICENSE)
