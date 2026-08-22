# TECHNICAL SPECIFICATION (SPEC.md): `sre-kit`

> **For AI agent**: Read this file in full before starting any change. Confirm understanding of
> constraints before running `/plan` or `/work`. When this file changes in a way that affects an
> active `docs/changes/*.md`, note it in that change's Implementation Notes rather than
> hand-syncing a separate contract file — there isn't one.

## Metadata

| Field | Value |
|-------|-------|
| Document Version | `v1.3` |
| Date | `2026-08-22` |
| Architect / Owner | `avatarsik666@gmail.com` |
| Stack | See [docs/STACK.md](./STACK.md) |
| Domain | Self-hosted SRE/observability aggregator for solo developers and small teams |

---

## 1. Project Overview and Goals

### 1.1 Problem

A solo developer or small team running 1–10 VPS needs 4–6 separate dashboards to get a full
picture of system health: metrics (Beszel-style), uptime (Uptime Kuma-style), container logs,
`fail2ban` via manual SSH, and a separate certificate-expiry script. Each tool has its own data
model, its own UI, and its own alert configuration. There is no single place to see "is everything
okay right now," and no single place to configure notifications once.

### 1.2 Goal and Success Metrics

**v1 (MVP) goal:** prove two things — (1) a single data contract (Metric / Check / Event / Alert)
can describe both continuous time-series and discrete status sources without distortion, and (2)
"SSH connection to a host + one unified UI" saves real time over running separate tools.

Success metrics for v1:
- Two adapters (`host-metrics-ssh`, `uptime-http`) run concurrently against real VPS without
  requiring a contract change.
- An architect can add a new source through the UI in under 5 minutes with zero manual exporter
  setup.
- A real alert (e.g. test service down, cert expiring) reaches Telegram within seconds of the
  triggering condition, followed by a resolved notification when it clears.
- 2–4 weeks of dogfooding on the architect's own VPS produces a concrete v2 backlog rather than
  uncovering v1-blocking gaps.

**Product direction (post-MVP):** sre-kit remains an observability data plane and read-only
management UI. It may run on the operator's workstation or on a dedicated management VPS and
connect to many unrelated applications through adapters. Installation, deployment, rollback and
backup of monitored tools belong to each target application's operations layer, not to sre-kit.

### 1.3 Project Boundaries

| Included (v1) | Excluded (v1) |
|----------|----------|
| Unified data contract: Metric, Check, Event, Alert | Remote actions, deployment, rollback or configuration mutation |
| `host-metrics-ssh` adapter (CPU/RAM/disk/network via SSH) | Docker container adapter |
| `uptime-http` adapter (HTTP/TCP check + TLS expiry) | Backup / dead-man-switch adapter |
| `fail2ban-ssh` adapter | |
| `journal-http` adapter (systemd-journal-gatewayd) | |
| `beszel-api` adapter (host + per-container metrics via PocketBase) | |
| `umami-http` adapter (web analytics, Umami-style) — added post-M6, contract proven on 5 prior adapters (§10) | |
| Live dashboard via WebSocket push (Sources, Dashboard, Source detail) | Multi-user / RBAC / audit log |
| Alert rules with single Telegram notification channel | Escalation / repeat notifications on unresolved alerts |
| Single admin-password auth | OAuth / full user management |
| Pull adapters plus generic authenticated push ingestion | Distributed/high-cardinality metrics storage |
| Projects, bounded retention and hourly metric rollups | Tool installers, Docker Compose presets and target-host credential management |
| Schema-driven analytics/operations dashboards | Fingerprinting or claims that inferred traffic is a known human |

**First-party reference deployment.** `infraegev2` is a sibling repository owned by the same
architect, not an external customer integration. `sre-kit` owns the observability core, adapter
contracts, source configuration, normalization, alerting and monitoring UI. infraegev2 owns its
application telemetry plus the installation and lifecycle of observability components on its VPS.
The two repositories integrate only through versioned source-registration and telemetry-ingestion
contracts. Neither repository imports the other's internal code or assumes its filesystem layout.
For the current infraegev2 target, password-only `root` is an explicitly accepted administration
and SSH-adapter input. sre-kit stores the credential only in its encrypted Source secret store and
does not own, schedule or recommend changes to that target access policy.

---

## 2. Domain Context

### 2.1 Roles and Permissions

| Role | Capabilities | Restrictions |
|------|-------------|--------------|
| `Admin` (the single end user) | Full access: add/remove sources, configure alert rules and notification channels, view all data | v1 is single-user — no roles/permissions to restrict *within* the product itself |
| `Architect` | Defines product intent and scope via `docs/SPEC.md` / `docs/changes/*.md` | Does not write code directly |
| `AI_Agent` | Implements changes via `/work`, runs gates via `/ship` | No direct push to `main` outside `/ship` |

### 2.2 Key Entities

All data from any adapter is normalized to exactly one of four entities before it reaches storage,
UI, or alerting — none of those layers know adapter-specific detail:

`Project → Source → (Metric | Check | Event) → Alert`

- **Project** — an operator-defined application or system boundary grouping Sources and dashboard
  navigation without changing the normalized telemetry contract.
- **Source** — a configured adapter or push producer instance (`sources` table): which adapter, its config, enabled
  state, last-seen status.
- **Metric** — a time-series data point (`{name, timestamp, value, labels}`), e.g. `cpu.usage_percent`.
- **Check** — a discrete status snapshot (`{name, timestamp, status: ok|warn|critical, meta}`), e.g.
  a TLS-expiry check.
- **Event** — a discrete log/occurrence (`{timestamp, level, message, labels}`), e.g. a fail2ban ban.
- **Alert** — derived by the core's Alert router from rules applied to Metric/Check/Event; has a
  full firing → resolved lifecycle (§10.7 semantics captured in §6 below).

Remote actions are deliberately not a fifth core entity. An external operations tool may report
its run as ordinary `Check` and `Event` records, but sre-kit never interprets those records as
authorization to mutate the target.

---

## 3. Data Model

SQLite. The schema below summarizes the active migrations under
`internal/platform/db/migrations/`; those embedded forward migrations, not this prose copy, are the
executable source of truth.

```sql
CREATE TABLE projects (
  id TEXT PRIMARY KEY,
  name TEXT,
  slug TEXT UNIQUE,
  description TEXT
);

CREATE TABLE sources (
  id TEXT PRIMARY KEY,          -- UUID generated by the core, never by the adapter (see rationale below)
  project_id TEXT,              -- FK -> projects.id
  adapter_name TEXT,
  config_json TEXT,
  enabled BOOLEAN,
  last_status TEXT,              -- 'ok' | 'unreachable' | 'error'
  last_seen_at DATETIME
);

CREATE TABLE metrics (
  source_id TEXT,
  name TEXT,
  ts DATETIME,                   -- always stamped by the core (Adapter runner), never by the adapter or remote host
  value REAL,
  labels_json TEXT,
  schema_version TEXT            -- e.g. "1.0" — see contract versioning rule in §4
);

CREATE TABLE checks (
  source_id TEXT,
  name TEXT,
  ts DATETIME,
  status TEXT,
  meta_json TEXT,
  schema_version TEXT
);

CREATE TABLE events (
  source_id TEXT,
  ts DATETIME,
  level TEXT,
  message TEXT,
  labels_json TEXT,
  schema_version TEXT
);

CREATE TABLE alerts (
  id TEXT PRIMARY KEY,
  source_id TEXT,
  rule_id TEXT,
  severity TEXT,
  message TEXT,
  created_at DATETIME,
  resolved_at DATETIME           -- NULL while active; set when the resolved notification fires
);

CREATE TABLE alert_rules (
  id TEXT PRIMARY KEY,
  source_id TEXT,
  target_name TEXT,              -- metric/check name the rule applies to
  condition TEXT,                -- '>' | '<' | '=' | 'status_is'
  threshold TEXT,
  debounce_seconds INTEGER,      -- flap protection, see §6
  notify_channel_id TEXT,        -- FK -> notification_channels.id
  enabled BOOLEAN
);

CREATE TABLE notification_channels (
  id TEXT PRIMARY KEY,
  type TEXT,                     -- 'telegram' only in v1
  config_json TEXT,              -- non-secret config, e.g. {"chat_id": "..."}
  secret_ref TEXT,                -- bot token lives in secrets.enc.json, same pattern as §3
  enabled BOOLEAN
);

CREATE TABLE metrics_rollup (
  source_id TEXT,
  name TEXT,
  labels_json TEXT,
  bucket_ts DATETIME,            -- UTC hour
  min_value REAL,
  max_value REAL,
  avg_value REAL,
  sample_count INTEGER
);

CREATE TABLE ingestion_batches (
  source_id TEXT,
  idempotency_key TEXT,
  received_at DATETIME,
  record_count INTEGER,
  PRIMARY KEY (source_id, idempotency_key)
);

CREATE TABLE maintenance_runs (...); -- status and deletion counts for each retention pass
```

The Change 22 baseline contains no retired Host or provisioning schema. Database files created by
an earlier development baseline are unsupported and must be reset through an explicit operator
action; startup never deletes or rewrites them implicitly.

Migration `0001_init.sql` always owns a protected `default` Project for newly created Sources that
have not yet been assigned. It may remain empty after every Source moves to a named Project; the
Project API intentionally rejects its deletion, so an empty `default` row is expected baseline
state rather than stale dogfood data.

Secrets (SSH keys, third-party API tokens) are **not** stored in SQLite. They live in a separate
`secrets.enc.json`, symmetrically encrypted with a key from an environment variable or a
0600-permission file, never committed to git. `sources.config_json` only stores a `secret_ref` id,
never the secret value itself — this lets the main DB be freely backed up/copied without leaking
credentials.

Change 22 intentionally replaces the development baseline rather than migrating historical rows.
Raw Metric/Check/Event records have a 30-day TTL; hourly Metric rollups have a 13-month TTL.
Maintenance is idempotent and observable. Runtime database deletion is an explicit operator action
outside normal startup and requires confirmation of the resolved file path; secrets are separate
and never included in a telemetry reset.

---

## 4. API / Backend Contract

The **data contract** (`contract.schema.json`, versioned — breaking change = major version bump,
additive-only otherwise) defines the four wire entities adapters emit as NDJSON:

- `metric` — `{type, source_id, name, timestamp, value, labels}`
- `check` — `{type, source_id, name, timestamp, status, meta}`
- `event` — `{type, source_id, timestamp, level, message, labels}`
- `alert` — `{type, severity, message, source_id, created_at, resolved_at}` (core-generated only,
  never emitted directly by an adapter)

Every adapter is an independent subprocess (any language) declaring a `manifest.json`
(`name`, `version`, `mode: pull|stream|push`, `emits`, `config_schema`, optional
`presentation_schema`, and for `stream` mode, `heartbeat_seconds`). Presentation descriptors name
measurements, unit/format, visualization, aggregation, grouping and label dimensions; the UI does
not hardcode Umami or infraegev2 concepts. The core's Adapter runner validates every NDJSON line against
`contract.schema.json` before writing to storage; invalid lines are logged with the Source id,
and 10 consecutive invalid lines (default) auto-disables the source and raises an
"adapter misbehaving" Alert.

**HTTP/WS API surface:**

| Verb / Method | Path | Auth | Payload / Response |
|---------------|------|------|---------------------|
| GET | `/healthz` | none | empty 200 readiness/liveness response |
| GET/POST/PATCH/DELETE | `/api/projects[/{id}]` | session | manage application/system groups |
| GET | `/api/sources` | session | list project-scoped sources + current status and presentation schema |
| POST | `/api/sources` | session | `{project_id, adapter_id, config}` → created source |
| PATCH | `/api/sources/{id}` | session | enable/disable/update config |
| DELETE | `/api/sources/{id}` | session | remove source |
| GET | `/api/metrics?source=&name=&from=&to=&resolution=&limit=` | session | bounded raw/hourly time-series slice with labels |
| GET | `/api/checks?source=` | session | current statuses |
| GET | `/api/events?source=&limit=` | session | event feed |
| GET | `/api/alerts?status=` | session | active/resolved alerts |
| GET | `/api/alert-rules?source=` | session | list of alert rules |
| POST | `/api/alert-rules` | session | `{source_id, target_name, condition, threshold, debounce_seconds, notify_channel_id}` |
| PATCH | `/api/alert-rules/{id}` | session | update/enable/disable a rule |
| DELETE | `/api/alert-rules/{id}` | session | remove a rule |
| GET | `/api/notification-channels` | session | list of configured channels as `{id, type, chat_id, enabled}`; never returns a token or secret reference |
| POST | `/api/notification-channels` | session | `{type: "telegram", chat_id, bot_token}` → stores `bot_token` via the §3 secrets mechanism and returns `{id, type, chat_id, enabled}` |
| PATCH | `/api/notification-channels/{id}` | session | update config / enable / disable / rotate token |
| DELETE | `/api/notification-channels/{id}` | session | remove a channel (blocked if an enabled `alert_rules` row still references it) |
| POST | `/api/auth/login` | none (this *is* the login) | admin password → session cookie |
| WS | `/api/stream` | session | live pub/sub feed of new Metric/Check/Event/Alert, filtered by subscribed `source_id`s |
| GET | `/api/adapters` | session | installed adapters + their manifests |
| POST | `/api/sources/{id}/records` | source token | idempotent versioned Metric/Check/Event batch |

Adapter execution modes:
- **Pull** (default) — Scheduler runs the subprocess on a per-source interval; non-zero exit or
  timeout marks the source `unreachable` (with debounce, §6).
- **Stream** — Supervisor behavior (restart/backoff/heartbeat) is implemented and unit-tested, but
  the composition root does not wire stream Sources because no stream adapter exists. Do not claim
  runtime stream support until one real adapter proves scheduling, shutdown and recovery end to end.
- **Push** — a Source-scoped bearer token admits versioned batches through the same validation,
  storage, alert and live-publish application service used by adapters. `Idempotency-Key` prevents
  duplicate batch application; credentials remain in the encrypted secret store.

`source_id` is always a UUID generated by the core at source-creation time, never derived from
adapter config (host/port) — this keeps historical metric attribution stable even if a source's
underlying host/IP changes later.

---

## 5. Frontend / Client Contract

### 5.1 Pages

| Page | Route | Purpose |
|------|-------|---------|
| Dashboard | `/` | Project-aware overview of all declared KPIs, traffic, acquisition, geography, devices, pages, product events, infrastructure health, checks, alerts and events |
| Sources | `/sources` | List of connected sources, per-source status, enable/disable, add/remove |
| Source detail | `/sources/:id` | Full schema-driven detail for every measurement and label dimension declared by the Source |
| Notifications | `/notifications` | Configure notification channels (Telegram v1) and alert rules |
| Add source | drawer over `/sources` | Schema-driven form generated from the chosen adapter's `manifest.json` `config_schema`; “Test connection” currently validates locally and does not probe the target because no endpoint exists |
| Login | `/login` | Single admin-password form |

### 5.2 Components / Stores

| Component / Store | Purpose | Notes |
|--------------------|---------|-------|
| WS stream store | Single `/api/stream` connection; pub/sub by visible `source_id`s | On reconnect: one REST snapshot fetch for currently-visible sources, then re-subscribe. Seconds-scale data loss during a reconnect gap is accepted (monitoring, not a ledger). |
| TanStack Query cache | Layered over the WS store for historical/paginated reads (`/api/metrics`, `/api/events`) and cache invalidation | No polling — cache updates are driven by WS push, not refetch intervals |
| Status tile | Renders one source: name, status-pulse dot, sparkline, check summary | Reused on Dashboard and Sources |
| Plot | `uPlot` with an owned React lifecycle wrapper, resize handling and textual/table fallback | Used only for time series where a chart materially helps |
| Add-source form | Renders from a `config_schema` JSON shape | Supports declared field types; current test action validates locally, while a real target probe requires a future API contract |

### 5.3 Design System

The UI uses Base UI headless behavior plus repository-owned semantic HTML/CSS; Mantine and Recharts
are not part of the target stack. No screenshots or existing brand exist for this project — direction below was derived via the
`impeccable` skill from the domain, audience, and the architect's stated preferences
(clean/modern SaaS tone, dark-mode default, compact/dense information layout, no brand
constraints, standard WCAG 2.2 AA).

**Palette** (dark-default; status colors are semantic and never repurposed for chrome):

| Token | Hex | Role |
|---|---|---|
| `bg-canvas` | `#0B0E14` | App background — ink-blue-black, not pure black |
| `bg-surface` | `#12161F` | Cards, tiles, panels |
| `bg-surface-raised` | `#1A1F2B` | Drawers, modals, popovers |
| `text-primary` | `#E6E9F0` | Primary text |
| `text-muted` | `#8992A6` | Secondary/meta text |
| `accent-primary` | `#6C8EFF` | Interactive/brand only (links, primary buttons, active nav) |
| `status-ok` | `#3DD68C` | Check/source healthy |
| `status-warn` | `#F5B94A` | Degraded / approaching threshold |
| `status-critical` | `#F2545B` | Alert firing / down |
| `status-unreachable` | `#5B6478` | No signal — visually distinct from `text-muted` so "unknown" never reads as "fine" |

**Typography** — two roles:
- **Instrument Sans** — semibold for page/section titles, medium for UI chrome (nav, buttons, labels).
- **JetBrains Mono** — every literal data value: hostnames, source IDs, metric numbers, timestamps,
  log/event lines. Distinguishes "this is a value" from "this is a control," matching SRE-tool
  convention.

**Layout**
- Left icon+label collapsible rail nav (a tool used all day suits persistent side nav over a
  marketing-style top nav).
- Dashboard: dense responsive grid of compact status tiles (mono source name, status-pulse dot,
  axis-less sparkline, check-count summary) + a slim persistent Recent Alerts rail.
- Source detail: full-width live chart (mono axis labels) over a dense timestamped event feed,
  using the same pulse motif per line.
- Add Source: right-side drawer over the Sources list, not a separate page — keeps list context
  during setup.
- Spacing: 4px base unit; tight line-height (1.3–1.4) in data views, relaxed (1.5–1.6) in
  forms/body copy.

**Signature — the status pulse.** An 8px dot + soft glow ring (subtle 2s scale/opacity loop,
static under `prefers-reduced-motion`) used identically for source connectivity, check status,
alert severity, and adapter health. This is the product's core idea — one data contract, four
entity types, one UI — expressed as a literal recurring visual token: the same dot means the same
thing everywhere, regardless of which adapter produced the signal.

**Accessibility**: WCAG 2.2 AA. All status colors verified ≥4.5:1 against both surface tokens;
status is never conveyed by color alone (each pulse ships with a text label or distinct icon
shape); visible focus rings on all interactive elements; motion respects
`prefers-reduced-motion`.

Later design change requests (component tweaks, palette adjustments) become ordinary Backlog items
in the active change file, not a rewrite of this section.

---

## 6. Auth & Access Model

**v1 decision: single admin password, no multi-user.** On first run without a stored hash, the core
generates an admin password and stores its bcrypt hash in `secrets.enc.json` (same encrypted-file
mechanism used for adapter secrets, §3). Later starts reuse that hash. Login issues an `HttpOnly`,
`Secure`-when-HTTPS session cookie.
Every REST/WS endpoint except `/api/auth/login` and a health-check endpoint requires a valid
session.

*Why not OAuth / full user management:* unjustified complexity for a single-user tool; full
roles/multi-user is explicitly out of scope for v1 (§1.3).

**Network perimeter (deployment guidance, not enforced in code):** self-hosted on a VPS should
default to binding the UI/API to loopback or a WireGuard interface, not public `0.0.0.0`; TLS and
public exposure (if needed) go through a separate reverse proxy (Nginx/Caddy) in front of that.
The password is a second line of defense, not the only one.

**Alert lifecycle** (firing → resolved, no escalation in v1): a rule firing creates an `alerts` row
(`resolved_at = NULL`) and sends one notification; when the condition clears, `resolved_at` is
stamped and one "resolved" notification is sent. No repeat/escalation notifications for
still-unresolved alerts in v1 (reserved as a future `repeat_interval_seconds` field, addable
without a breaking schema change). `unreachable` (transient — subprocess/network failure) is
debounced (default: 3 consecutive failures or 5 minutes continuous) before alerting; `error`
(adapter ran but returned invalid data / bad config) alerts immediately, no debounce — these need
different debounce semantics because one is likely transient and the other almost always needs a
human.

---

## 7. Infrastructure and Deploy/CI

### 7.1 Infrastructure

See [docs/STACK.md](./STACK.md) for concrete commands and tooling. Summary: Go backend, SQLite
storage, separately developed React + TanStack Start + Base UI/uPlot frontend,
NDJSON-over-stdio adapter protocol (language-agnostic, easy to debug by hand:
`echo config | ./adapter`).

infraegev2 is the first dogfood integration, not the reference filesystem or deployment shape.
Its operations layer installs and maintains target-side tools; sre-kit connects through adapters
and versioned ingestion contracts. A cross-repository change is complete only when both owning
contracts are updated through their SDD workflows.

**Architectural style**: the Go backend follows a modular DDD-like layout (bounded-context
packages under `internal/`, each split into `domain`/`application`/`infrastructure`/`interfaces`,
communicating across module boundaries through ports rather than direct imports); the React
frontend follows an FSD-like layout (`routes/pages/widgets/features/entities/shared`, one
`index.ts` public API per slice, ESLint-enforced import boundaries). Both were derived by porting
applicable conventions from a reference project on a matching stack. Full trees and rationale live
in `docs/STACK.md` §§ Backend Architecture / Frontend Architecture — this is a pointer, not a
duplicate.

### 7.2 Deploy / CI

Deploy shape: one self-contained release (Docker container or plain binary + web assets and
adapters) — matches the "run locally now, self-host on a management VPS later" trajectory. The
release must not assume that sre-kit and monitored targets share a filesystem, Docker network or
loopback interface. GitHub Actions validates the Go core and adapters, generated API contract, and
web client for pull requests and every push to `main`; the commands and exact-commit release check
are defined in `docs/STACK.md`. Until M11 ships a self-contained artifact, `/ship --release`
publishes source changes and requires a green CI run for the exact pushed commit, but does not
claim that an artifact was published or a management host was deployed.

---

## 8. Non-Functional Requirements

| Concern | Requirement |
|---------|-------------|
| Security headers / CORS | The current source distribution runs API and web development servers separately; the M11 production artifact targets one origin. Until then no public deployment is claimed. The final edge must set `X-Content-Type-Options`, frame protection and HSTS when TLS renewal is proven |
| Accessibility target | WCAG 2.2 AA (see §5.3) |
| Performance budget | Not a marketing/content site — budget expressed as live-update latency instead: new data visible in the UI within a few seconds of an adapter emitting it (§4 WS push design), not classic LCP/INP/CLS thresholds |
| Observability | Source-tagged adapter logs (§4) and the `unreachable`/`error` status distinction (§6) are the primary operational signals; no separate metrics/tracing system for the core itself in v1 |
| Backup / restore | Back up `sqlite` DB file + `secrets.enc.json` separately (the split exists specifically so the DB can be freely copied without leaking credentials, §3); no automated backup adapter in v1 (explicitly out of scope, §1.3) |
| Other (compliance, SLOs) | n/a — single-user self-hosted tool, no compliance targets for v1 |

---

## 9. Roadmap

| Milestone | Status | Goal | Key Outputs |
|-----------|--------|------|-------------|
| `M0` | complete | Contract frozen | `contract.schema.json` committed and versioned |
| `M1` | complete | Core skeleton | SQLite, HTTP API, pull runner plus tested stream supervisor, admin-password auth and sessions |
| `M2` | complete | `host-metrics-ssh` adapter | Real CPU/RAM/disk collection contract |
| `M3` | complete | `uptime-http` adapter (+ TLS expiry) | Concurrent adapters without a contract change |
| `M4` | complete | Minimal UI | Sources, Dashboard, Source detail, WS updates and manifest-backed source form |
| `M5` | complete | Alert router + Telegram channel | Firing/resolved alert lifecycle and notification-channel management |
| `M6` | in progress | Dogfooding on infraegev2 | Six production pull Sources plus the push Source are healthy in explicit operator-started sessions. Change 20 proves pull polling, quiet success, reversible failure/recovery and authenticated UI rendering; Changes 22–23 and the infraegev2 publisher add the analytics contour. The longer evidence window advances only while `sre-kit-local` is active |
| `M7` | partial | Dogfood extensions | `fail2ban-ssh`, `journal-http`, `beszel-api` and `umami-http` shipped; Docker adapter and second channel are not approved merely because the old row mentioned them |
| `M8` | retired | Host/provisioner prototype | Write-capable deployment was separated from the trust domain and removed from runtime by Change 15; inert migration data remains |
| `M9` | complete | Analytics core and dashboards | Change 22 shipped Projects, authenticated generic push, bounded raw/hourly data and the full schema-driven UI; Change 23 restored the deployed Umami v3 dimension path |
| `M10` | in progress | Adapter extensibility | Change 22 adds presentation capabilities and validates every first-party manifest; sandbox/contributor packaging remains later |
| `M11` | partial | Core distribution | Change 22 adds retention; reproducible artifact, backup/restore and always-on install remain later |

### 9.1 Current execution sequence

| Order | Scope | Exit evidence |
|-------|-------|---------------|
| `0` | Complete documentation Change 19 with linked infraegev2 Change 44 | Complete: six template configs remain manifest-valid; the stale local state and ownership boundaries were recorded without secrets or runtime mutation |
| `1` | Reconcile the six infraegev2 Sources in Change 20 and run a bounded soak | Complete: all six Sources have repeated fresh `ok` outcomes; quiet success, reversible failure/recovery and authenticated browser rendering are proven without target-side mutation |
| `2` | Complete the M9 analytics slice and deployed Umami compatibility | Complete: Change 22 shipped Projects, authenticated push, retention, presentation manifests and dashboards; Change 23 restored Umami v3 dimensions without widening the public contract |
| `3` | Continue bounded M6 dogfood through explicit sessions | The supported owner-only password recovery command already exists. Run the core only under explicit runtime ownership and accumulate enough real evidence to rank recurring gaps without claiming always-on coverage |
| `4` | Harden M10 before accepting external adapters | Manifest versioning, conformance and sandbox/trust decisions have executable acceptance evidence |
| `5` | Build M11 distribution | One reproducible artifact includes API, web and adapters; local install plus the architect-selected always-on path prove backup, restore, upgrade and exact-version health |

Docker collection and a second notification channel remain candidate dogfood findings, not
pre-approved next changes. M11 retention and off-host backup design must be explicit before the
core is treated as an always-on service.

---

## 10. Out of Scope

- Competing with Grafana/Prometheus on scale (retention, high cardinality, clustering).
- Distributed tracing / APM.
- A from-scratch metrics collector — only adapters to existing tools/protocols.
- A plugin marketplace/registry — local adapter folder only.
- Multi-tenant / RBAC — single user, single instance, for v1.
- Remote actions and deployment primitives. External operations tools may report their outcomes as
  telemetry, but core records never authorize a target mutation.
- Docker-container adapter, backup/dead-man-switch adapter — second iteration, after the contract
  is proven on the first 2–3 adapters. (The web-analytics adapter this list originally deferred
  alongside them shipped as `umami-http` in change-11, once that condition was met — see §1.3.)
- High-cardinality/distributed storage, arbitrary query languages or indefinite raw retention.
- Browser fingerprinting, covert analytics or equating non-bot traffic with a verified person.
- Escalation/repeat notifications for unresolved alerts.
- Adapter sandboxing — accepted risk while all adapters are first-party (see §11).
- Strict SSH host-key verification — accepted trust-on-first-connect (TOFU) risk for the existing
  read-only SSH adapters only; deployment credentials are never stored by sre-kit.
- Tool installers, Docker Compose presets, fleet bootstrap and application-specific backup or
  rollback. These belong to target-owned operations repositories regardless of platform.

---

## 11. Open Questions

- **M11 deploy verification target** — source publication is currently verified by an exact-commit
  green GitHub Actions run. M11 must separately choose and verify the self-contained artifact and
  management-host deployment path; CI success must not be reported as a deployed release.
  `[NEEDS_CLARIFICATION: should M11 verify a local install, a dedicated management VPS, or both?]`
- **Adapter sandboxing** — accepted as a known v1 compromise (adapters run as unsandboxed
  subprocesses with secret access via env/stdin). Explicitly deferred, not blocking, but flagged
  here so it isn't silently forgotten once third-party adapters become a possibility (v2+).
  `[NEEDS_CLARIFICATION: revisit before any third-party adapter is accepted]`
- **SSH host-key verification** — TOFU accepted for v1 as a single-user convenience trade-off;
  same treatment as above, revisit if the trust model changes (e.g. shared/team use).

---

## 12. Extension and Operations Boundary

sre-kit's extension surface is observational:

- pull adapters read existing tools and protocols; stream supervision exists as a tested internal
  capability but is not part of the wired runtime until a real adapter proves it;
- the M9 push receiver accepts authenticated Metric/Check/Event records from external producers;
- every input is normalized through the same Source-scoped validation, storage, alerting and UI
  pipeline;
- adapter-specific configuration remains at the transport edge.

Installation and lifecycle automation is explicitly external. A target-owned tool may use SSH,
Docker Compose, systemd, Kubernetes or a provider API, but those credentials, plans, runs and
rollback state never become sre-kit entities. The operator integrates through the Source API/UI
and sanitized Metric/Check/Event ingestion. sre-kit being unavailable must not block
the external operation; delayed telemetry may be delivered after connectivity returns.

The boundary keeps infraegev2 useful as dogfood without making its repository layout, Compose
services, VPS topology or deployment workflow part of the open-source core.

The linked infraegev2 template currently names one Project, six pull Sources (`uptime-http`,
`host-metrics-ssh`, `fail2ban-ssh`, `journal-http`, `beszel-api`, `umami-http`) and one passive
`push` Source. infraegev2 owns endpoint readiness, producers, accounts and network reachability;
sre-kit owns encrypted credentials, Source records, polling/ingestion, status, alerts and UI.

Sanitized local evidence on 2026-08-20 now shows exactly six unique enabled records in
`data/sre-kit.db`, one for each template. Every Source advanced through at least three independent
scheduler outcomes and finished fresh `ok`; fail2ban also proved that a successful pull with no new
records still refreshes Source health. A controlled uptime failure produced critical Checks and
Source `error`, and restoring the exact prior config returned it to fresh `ok`. Authenticated
Dashboard, Sources and all six Source-detail routes rendered without current console errors. The
bounded proof activates M6 dogfooding but does not complete its longer 2–4 week evidence window.
