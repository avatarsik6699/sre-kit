# TECHNICAL SPECIFICATION (SPEC.md): `sre-kit`

> **For AI agent**: Read this file in full before starting any change. Confirm understanding of
> constraints before running `/plan` or `/work`. When this file changes in a way that affects an
> active `docs/changes/*.md`, note it in that change's Implementation Notes rather than
> hand-syncing a separate contract file — there isn't one.

## Metadata

| Field | Value |
|-------|-------|
| Document Version | `v1.0` |
| Date | `2026-08-13` |
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

### 1.3 Project Boundaries

| Included (v1) | Excluded (v1) |
|----------|----------|
| Unified data contract: Metric, Check, Event, Alert | `Action` primitive (reverse commands — unban IP, restart container) — reserved for v2/v3, see §2.2 |
| `host-metrics-ssh` adapter (CPU/RAM/disk/network via SSH) | Docker container adapter |
| `uptime-http` adapter (HTTP/TCP check + TLS expiry) | Backup / dead-man-switch adapter |
| `fail2ban-ssh` adapter (optional, time-permitting) | Web analytics adapter (Umami-style) |
| Live dashboard via WebSocket push (Sources, Dashboard, Source detail) | Multi-user / RBAC / audit log |
| Alert rules with single Telegram notification channel | Escalation / repeat notifications on unresolved alerts |
| Single admin-password auth | OAuth / full user management |
| Pagination / retention policy tooling beyond simple TTL deletion | Downsampling / rollup usage (schema reserved, not active) |

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

`Source → (Metric | Check | Event) → Alert`

- **Source** — a configured adapter instance (`sources` table): which adapter, its config, enabled
  state, last-seen status.
- **Metric** — a time-series data point (`{name, timestamp, value, labels}`), e.g. `cpu.usage_percent`.
- **Check** — a discrete status snapshot (`{name, timestamp, status: ok|warn|critical, meta}`), e.g.
  a TLS-expiry check.
- **Event** — a discrete log/occurrence (`{timestamp, level, message, labels}`), e.g. a fail2ban ban.
- **Alert** — derived by the core's Alert router from rules applied to Metric/Check/Event; has a
  full firing → resolved lifecycle (§10.7 semantics captured in §6 below).
- **Action** *(reserved, not implemented in v1)* — a future fifth primitive for reverse commands
  (unban an IP, restart a container). The contract must stay additive-only so this can be added in
  v2/v3 without a breaking change (see `contract.schema.json` versioning rule in §4).

---

## 3. Data Model

SQLite. Draft schema below is authoritative for v1; migrations happen through explicit schema
changes tracked in `docs/changes/*.md` Files sections, not a separate ORM-managed history unless
one is adopted later.

```sql
CREATE TABLE sources (
  id TEXT PRIMARY KEY,          -- UUID generated by the core, never by the adapter (see rationale below)
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
  notify_channel TEXT,
  enabled BOOLEAN
);

-- Reserved for v2, not populated/used in v1 — see §9 retention note.
CREATE TABLE metrics_rollup (
  source_id TEXT,
  name TEXT,
  bucket_ts DATETIME,            -- 5-minute bucket
  agg_json TEXT                  -- min/max/avg/count for the bucket
);
```

Secrets (SSH keys, third-party API tokens) are **not** stored in SQLite. They live in a separate
`secrets.enc.json`, symmetrically encrypted with a key from an environment variable or a
0600-permission file, never committed to git. `sources.config_json` only stores a `secret_ref` id,
never the secret value itself — this lets the main DB be freely backed up/copied without leaking
credentials.

Retention: v1 uses a configurable TTL on `metrics`/`events` (default 30 days), purged by a daily
background job. `metrics_rollup` is reserved in the schema now so a v2 downsampling feature doesn't
require a migration, but it is not written to or read from in v1.

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
(`name`, `version`, `mode: pull|stream`, `emits`, `config_schema`, and for `stream` mode,
`heartbeat_seconds`). The core's Adapter runner validates every NDJSON line against
`contract.schema.json` before writing to storage; invalid lines are logged to a per-source error
log, and 10 consecutive invalid lines (default) auto-disables the source and raises an
"adapter misbehaving" Alert.

**HTTP/WS API surface:**

| Verb / Method | Path | Auth | Payload / Response |
|---------------|------|------|---------------------|
| GET | `/api/sources` | session | list of sources + current status |
| POST | `/api/sources` | session | `{adapter_id, config}` → created source |
| PATCH | `/api/sources/:id` | session | enable/disable/update config |
| DELETE | `/api/sources/:id` | session | remove source |
| GET | `/api/metrics?source=&name=&from=&to=` | session | time-series slice |
| GET | `/api/checks?source=` | session | current statuses |
| GET | `/api/events?source=&limit=` | session | event feed |
| GET | `/api/alerts?status=` | session | active/resolved alerts |
| GET | `/api/alert-rules?source=` | session | list of alert rules |
| POST | `/api/alert-rules` | session | `{source_id, target_name, condition, threshold, debounce_seconds, notify_channel}` |
| PATCH | `/api/alert-rules/:id` | session | update/enable/disable a rule |
| DELETE | `/api/alert-rules/:id` | session | remove a rule |
| POST | `/api/auth/login` | none (this *is* the login) | admin password → session cookie |
| POST | `/api/webhooks/:source_id` | source-scoped token | push-mode adapter data (dead-man-switch style) |
| WS | `/api/stream` | session | live pub/sub feed of new Metric/Check/Event/Alert, filtered by subscribed `source_id`s |
| GET | `/api/adapters` | session | installed adapters + their manifests |

Adapter execution modes:
- **Pull** (default) — Scheduler runs the subprocess on a per-source interval; non-zero exit or
  timeout marks the source `unreachable` (with debounce, §6).
- **Stream** — Adapter supervisor keeps the subprocess alive for as long as the source is enabled;
  restarts on crash with exponential backoff; requires a periodic heartbeat line (per
  `heartbeat_seconds`) so the core can distinguish "quiet" from "hung/dead."

`source_id` is always a UUID generated by the core at source-creation time, never derived from
adapter config (host/port) — this keeps historical metric attribution stable even if a source's
underlying host/IP changes later.

---

## 5. Frontend / Client Contract

### 5.1 Pages

| Page | Route | Purpose |
|------|-------|---------|
| Dashboard | `/` | Grid of status tiles across all sources + a persistent recent-alerts rail; live-updating |
| Sources | `/sources` | List of connected sources, per-source status, enable/disable, add/remove |
| Source detail | `/sources/:id` | Live metric chart (with 24h/7d historical toggle) + live event feed for one source |
| Notifications | `/notifications` | Configure notification channels (Telegram v1) and alert rules |
| Add source | drawer over `/sources` | Schema-driven form generated from the chosen adapter's `manifest.json` `config_schema`, with a test-connection step before saving |
| Login | `/login` | Single admin-password form |

### 5.2 Components / Stores

| Component / Store | Purpose | Notes |
|--------------------|---------|-------|
| WS stream store | Single `/api/stream` connection; pub/sub by visible `source_id`s | On reconnect: one REST snapshot fetch for currently-visible sources, then re-subscribe. Seconds-scale data loss during a reconnect gap is accepted (monitoring, not a ledger). |
| TanStack Query cache | Layered over the WS store for historical/paginated reads (`/api/metrics`, `/api/events`) and cache invalidation | No polling — cache updates are driven by WS push, not refetch intervals |
| Status tile | Renders one source: name, status-pulse dot, sparkline, check summary | Reused on Dashboard and Sources |
| Live chart | `@mantine/charts` (Recharts) time-series with live-append + historical window toggle | Used on Source detail |
| Add-source form | Renders from a `config_schema` JSON shape | Must support the manifest's declared field types plus a test-connection action |

### 5.3 Design System

No screenshots or existing brand exist for this project — direction below was derived via the
`frontend-design` skill from the domain, audience, and the architect's stated preferences
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

**v1 decision: single admin password, no multi-user.** On first run, the core generates or accepts
an admin password and stores its bcrypt hash in `secrets.enc.json` (same encrypted-file mechanism
used for adapter secrets, §3). Login issues an `HttpOnly`, `Secure`-when-HTTPS session cookie.
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

See [docs/STACK.md](./STACK.md) for concrete commands and tooling. Summary: Go backend (single
static binary, `x/crypto/ssh`), SQLite storage, React + TanStack Start + Mantine UI v9+ frontend,
NDJSON-over-stdio adapter protocol (language-agnostic, easy to debug by hand:
`echo config | ./adapter`).

**Architectural style**: the Go backend follows a modular DDD-like layout (bounded-context
packages under `internal/`, each split into `domain`/`application`/`infrastructure`/`interfaces`,
communicating across module boundaries through ports rather than direct imports); the React
frontend follows an FSD-like layout (`routes/pages/widgets/features/entities/shared`, one
`index.ts` public API per slice, ESLint-enforced import boundaries). Both were derived by porting
applicable conventions from a reference project on a matching stack. Full trees and rationale live
in `docs/STACK.md` §§ Backend Architecture / Frontend Architecture — this is a pointer, not a
duplicate.

### 7.2 Deploy / CI

Target deploy shape: one Docker container (or a plain binary + systemd unit) — matches the
"run locally now, self-host on a VPS later" trajectory. No CI pipeline is configured yet (see
`docs/STACK.md` Full/Release Gate — rows are `n/a` pending tooling setup). `/ship --release`'s
deploy verification step is not yet defined; this is an open item (§11).

---

## 8. Non-Functional Requirements

| Concern | Requirement |
|---------|-------------|
| Security headers / CORS | Single-origin app (UI served by the same Go binary as the API) — no cross-origin API access in v1, so CORS is `n/a` by design; standard security headers (`X-Content-Type-Options`, `X-Frame-Options`, HSTS when served over TLS) still apply |
| Accessibility target | WCAG 2.2 AA (see §5.3) |
| Performance budget | Not a marketing/content site — budget expressed as live-update latency instead: new data visible in the UI within a few seconds of an adapter emitting it (§4 WS push design), not classic LCP/INP/CLS thresholds |
| Observability | Per-source adapter error log (§4); `unreachable`/`error` status distinction (§6) is the primary operational signal; no separate metrics/tracing system for the core itself in v1 |
| Backup / restore | Back up `sqlite` DB file + `secrets.enc.json` separately (the split exists specifically so the DB can be freely copied without leaking credentials, §3); no automated backup adapter in v1 (explicitly out of scope, §1.3) |
| Other (compliance, SLOs) | n/a — single-user self-hosted tool, no compliance targets for v1 |

---

## 9. Roadmap

| Milestone | Goal | Key Outputs |
|-----------|------|-------------|
| `M0` | Contract frozen | `contract.schema.json` committed and versioned |
| `M1` | Core skeleton | SQLite schema, HTTP API stubs, adapter runner (pull + stream, NDJSON validation), admin-password auth + sessions; a stub adapter's test data is visible via `/api/metrics` and the API is inaccessible without a session |
| `M2` | `host-metrics-ssh` adapter | Real CPU/RAM/disk data from a test VPS lands in the DB |
| `M3` | `uptime-http` adapter (+ TLS expiry) | Both adapters run concurrently without a contract change |
| `M4` | Minimal UI (Sources, Dashboard, Source detail) | A source can be added and its data seen with no manual API calls |
| `M5` | Alert router + Telegram channel | A real alert fires when a test service goes down |
| `M6` | Dogfooding (2–4 weeks on the architect's own VPS) | Concrete, prioritized v2 backlog |
| `M7` (v2, post-review) | `fail2ban-ssh` adapter, Docker adapter, second notification channel | Scoped from dogfooding findings |

---

## 10. Out of Scope

- Competing with Grafana/Prometheus on scale (retention, high cardinality, clustering).
- Distributed tracing / APM.
- A from-scratch metrics collector — only adapters to existing tools/protocols.
- A plugin marketplace/registry — local adapter folder only.
- Multi-tenant / RBAC — single user, single instance, for v1.
- The `Action` primitive (reverse commands: unban IP, restart container, etc.) — reserved for
  v2/v3; the contract must stay additive-only so adding it later isn't a breaking change.
- Docker-container adapter, backup/dead-man-switch adapter, web-analytics adapter — second
  iteration, after the contract is proven on the first 2–3 adapters.
- Pagination/retention policy tooling beyond simple TTL deletion.
- Escalation/repeat notifications for unresolved alerts.
- Adapter sandboxing — accepted risk while all adapters are first-party (see §11).
- Strict SSH host-key verification — accepted trust-on-first-connect (TOFU) risk for a
  single-user tool adding its own servers (see §11).

---

## 11. Open Questions

- **Deploy/release verification target** — `docs/STACK.md`'s Release Gate (container scan,
  health-check verification, `gh` auth) is currently all `n/a`/`no`; needs concrete commands before
  `/ship --release` can be used meaningfully. `[NEEDS_CLARIFICATION: what does "deploy verified"
  mean for a self-hosted single-binary/container target — a manual VPS step, or something
  scriptable?]`
- **Adapter sandboxing** — accepted as a known v1 compromise (adapters run as unsandboxed
  subprocesses with secret access via env/stdin). Explicitly deferred, not blocking, but flagged
  here so it isn't silently forgotten once third-party adapters become a possibility (v2+).
  `[NEEDS_CLARIFICATION: revisit before any third-party adapter is accepted]`
- **SSH host-key verification** — TOFU accepted for v1 as a single-user convenience trade-off;
  same treatment as above, revisit if the trust model changes (e.g. shared/team use).
