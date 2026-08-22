# sre-kit

A self-hosted SRE/observability aggregator for solo developers and small teams running a handful
of VPS. One unified data model, one live dashboard, one place to configure alerts — instead of
juggling separate tools for metrics, uptime checks, and log-based events.

## Why

Running 1–10 VPS usually means stitching together 4–6 separate dashboards to answer "is everything
okay right now": a metrics tool, an uptime checker, container logs, `fail2ban` checked by hand over
SSH, a certificate-expiry script. Each has its own data model, its own UI, its own alerting setup.

sre-kit groups Sources into **Projects** and normalizes their output into four entities — **Metric**, **Check**, **Event**, and the
**Alert**s derived from them — collected by pull adapters over SSH, HTTP or TCP and shown on one
live-updating dashboard, with a single alert-rule engine notifying you over Telegram. The current
SSH adapters need no agent on the monitored host; HTTP adapters consume target-owned endpoints.

## Features

- **Unified data contract** (`Metric` / `Check` / `Event` / `Alert`) — a small, versioned,
  additive-only NDJSON schema (`internal/contract/contract.schema.json`) that any adapter, in any
  language, can emit against.
- **`host-metrics-ssh`** — CPU / RAM / disk usage sampled over SSH, no agent required on the
  target host.
- **`uptime-http`** — HTTP/TCP reachability checks with TLS certificate-expiry tracking.
- **`fail2ban-ssh`** — ban/unban activity from a remote host's `fail2ban` log, normalized to
  `Event`s.
- **Complete dashboards** — project overview and Source detail render every declared metric,
  label dimension, check, event and alert using a dark Base UI/uPlot client.
- **Pull or push** — first-party adapters and Source-token/idempotency-key push producers enter the
  same validation, storage, alert and live-update pipeline.
- **Bounded storage** — raw telemetry is kept for 30 days and hourly metric rollups for 13 months.
- **Alert router** — rule-based evaluation over Metric/Check/Event with a full firing → resolved
  lifecycle, delivered to Telegram.
- **Small self-hosted core** — Go API, SQLite storage, independently built web UI and first-party
  adapter subprocesses. Six production adapters are included in the API/adapters Docker image; a
  unified API+web release artifact remains M11.

## Architecture at a glance

```
adapters/<name>/        subprocess binaries speaking NDJSON over stdio (language-agnostic)
internal/adapterengine/ spawns/schedules adapters, validates NDJSON against the contract
internal/telemetry/     Metric / Check / Event ingestion and query
internal/alertrouter/   rule evaluation, firing/resolved lifecycle, notification channels
internal/sources/       configured adapter instances (what to run, with what config)
internal/projects/      operator-owned grouping and dashboard boundary
internal/platform/      shared kernel: config, db, secrets, http server, websocket hub
web/                    React + TanStack Start + Base UI/uPlot frontend
```

Each pull/stream adapter is an independent process: given resolved config on stdin, it emits NDJSON lines
(`metric` / `check` / `event`) on stdout and exits non-zero on failure to reach its target — that's
the entire subprocess contract. Passive push Sources instead authenticate directly to the records
endpoint and do not launch a binary.

### Relationship with infraegev2

[`infraegev2`](https://github.com/avatarsik6699/infraegev2) is the first dogfood integration, not a
repository-layout template for this product. sre-kit owns adapters, source configuration,
normalization, alerting and the monitoring UI. infraegev2 owns installation, configuration,
backup/restore and rollback of observability tools on its VPS. The repositories communicate only
through versioned Source registration and telemetry ingestion contracts.

The live infraegev2 target uses an independent `infraege-ops` Compose project. Its secret-free
template defines one Project, six pull Sources and one coarse traffic-aggregate push producer:
uptime, root/password SSH host metrics and fail2ban, WireGuard journal gateway, Beszel and Umami.
Config shapes match the current manifests. Change 20
reconciled exactly those six enabled Sources and proved repeated fresh polling, quiet success,
reversible uptime failure/recovery, and authenticated Dashboard, Sources and detail rendering.
This bounded proof starts the longer dogfood evidence window; it does not complete the planned
2–4 weeks of observation. The root/password choice is an accepted target policy; sre-kit consumes
it as Source input and does not own VPS access hardening or migration.

See [`docs/SPEC.md`](docs/SPEC.md) for the full technical specification (data model, API
contract, auth model, roadmap) and [`docs/STACK.md`](docs/STACK.md) for concrete stack details,
module layout, and conventions.

## Getting started

### Prerequisites

- Go 1.26.5 or newer within the supported 1.26 toolchain
- Node.js 24.x and pnpm 10.33.0 exactly (frontend)
- A Linux host reachable over SSH, for `host-metrics-ssh` / `fail2ban-ssh`

### Run locally

```bash
git clone git@github.com:avatarsik6699/sre-kit.git
cd sre-kit

cp .env.example .env
# Generate a secrets-store encryption key and write it to SRE_KIT_SECRETS_KEY in .env.
sre_kit_secrets_key=$(openssl rand -hex 32)
sed -i "s/^SRE_KIT_SECRETS_KEY=.*/SRE_KIT_SECRETS_KEY=$sre_kit_secrets_key/" .env
unset sre_kit_secrets_key

go mod download
set -a
. ./.env
set +a
go run ./cmd/server
```

The Go process serves the API on `SRE_KIT_ADDR` (default `:8080`). The current frontend is built
and served separately during development; a single packaged distribution is an M11 deliverable.

For frontend development with hot reload:

```bash
pnpm --dir web install --frozen-lockfile
pnpm --dir web dev
```

### Run the API and adapters with Docker

```bash
docker build -t sre-kit .
docker run -p 8080:8080 \
  -e SRE_KIT_SECRETS_KEY=$(openssl rand -hex 32) \
  -v sre-kit-data:/app/data \
  sre-kit
```

The current image does not contain or serve the web build. Use the frontend development command
above until the M11 distribution work defines the combined artifact.

### Adding a source

Once the server is running, add a source through the UI. The form validates the manifest-backed
configuration before saving; its current “Test connection” action does not probe the target
because no dedicated endpoint exists. SSH adapters need target credentials but no exporter or
agent installation. See `docs/SPEC.md` §3/§6 for the config and secrets model.

For an external producer, rotate a Source token through the authenticated API and send a
schema-versioned batch to `POST /api/sources/{id}/records` with `Authorization: Bearer …` and a
unique `Idempotency-Key`. Tokens belong in the producer's protected environment, never git.

### Recover the admin password

With the same `SRE_KIT_SECRETS_PATH` and `SRE_KIT_SECRETS_KEY` as the server, stop the core and run:

```bash
go run ./cmd/admin rotate-password
```

The command prints the replacement once. Save it immediately; the encrypted store contains only
its hash. Existing in-memory sessions disappear when the server restarts.

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

The core v1 feature set is implemented: unified contract, six production pull adapters, live
dashboard and Telegram alerting. The six infraegev2 Sources have passed bounded end-to-end
reconciliation. M6 remains in progress through explicit operator-started sessions: polling,
publisher delivery and alerts run only while `sre-kit-local` is active, so no evidence accumulates
while the workstation or session is off. Change 22 completed the M9 Projects, generic push,
presentation, bounded-retention and dashboard slice; Change 23 restored Umami v3 dimension
compatibility. A combined deployable distribution remains a later milestone. See
[`docs/SPEC.md`](docs/SPEC.md) §9 for the roadmap and
[`docs/changes/archive/`](docs/changes/archive/) for the history of how each piece was built.

## Development workflow

This project is built through a spec-driven development (SDD) process: `docs/SPEC.md` is the
product contract, and each unit of work is tracked as a `docs/changes/NN-slug.md` file carrying
its own backlog, files-touched list, and review notes, archived once shipped. The rules for that
process live in [`AGENTS.md`](AGENTS.md); see [`CONTRIBUTING.md`](CONTRIBUTING.md) for how to
propose and land a change.

## License

[MIT](LICENSE)
