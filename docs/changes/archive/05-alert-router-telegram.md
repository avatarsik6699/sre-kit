# CHANGE 05 — Alert Router + Telegram Channel

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `05` |
| Slug | `alert-router-telegram` |
| Title | Alert Router + Telegram Channel |
| Status | `archived` |
| Branch | `feature/05-alert-router-telegram` |

---

## Goal

Deliver M5 from `docs/SPEC.md` §9: an Alert router that evaluates `alert_rules` against incoming
Metric/Check data (and source `unreachable`/`error` transitions) per the firing→resolved lifecycle
and debounce semantics in SPEC §6, a Telegram notification channel that sends the firing/resolved
messages, and the `/notifications` page (SPEC §5.1) to configure both — so a real alert (test
service down, cert expiring) reaches Telegram within seconds and a resolved notification follows
when it clears, per SPEC §1.2's success metric.

---

## Design References

<!-- none provided -->

---

## Backlog

### Backend
- [x] `B1` `internal/alertrouter/domain` — `Alert`, `AlertRule` entities; condition evaluation
  (`>`, `<`, `=`, `status_is`); debounce state machine per SPEC §6 (`unreachable`: 3 consecutive
  failures or 5 min continuous; `error`: immediate, no debounce) — _Depends on:_ —
- [x] `B2` `internal/alertrouter/infrastructure` — SQLite repositories for `alerts`,
  `alert_rules`, `notification_channels` (new tables per SPEC §3) — _Depends on:_ B1
- [x] `B3` `internal/alertrouter/application` — `Service.EvaluateMetric`/`EvaluateCheck`/
  `EvaluateSourceStatus`: match against enabled rules for the source, apply debounce, open/resolve
  `alerts` rows, call the notifier port on each transition — _Depends on:_ B1, B2
- [x] `B4` `internal/notify/telegram` — Telegram Bot API client (send message via bot token +
  chat id), implements the application-layer `Notifier` port — _Depends on:_ —
- [x] `B5` Wire `alertrouter/application.Service` into `internal/telemetry/application.Service`
  (new port, mirrors the existing `Publisher`/`SourceStatusUpdater` pattern from change 04) so
  every successful `IngestMetric`/`IngestCheck` triggers evaluation; wire source `unreachable`/
  `error` transitions (from change 04's `MarkSeen`) the same way — _Depends on:_ B3
- [x] `B6` Extend `internal/platform/wshub` publish path to fan out `alert` frames (per SPEC §4
  wire shape) on fire/resolve, alongside existing Metric/Check/Event frames — _Depends on:_ B3
- [x] `B7` `internal/alertrouter/interfaces/http` — `GET /api/alerts?status=`,
  `GET/POST/PATCH/DELETE /api/alert-rules`,
  `GET/POST/PATCH/DELETE /api/notification-channels` (bot token stored via the existing §3
  secrets mechanism, never returned in responses) — _Depends on:_ B2, B4
- [x] `B8` `swag` doc-comment annotations for B7's handlers so `contracts/openapi.json`/
  `schema.ts` cover the new endpoints — _Depends on:_ B7

### Frontend
- [x] `F1` Alert entity/frame handling in the WS stream store (`web/src/shared/lib`) — consume
  `alert` frames alongside existing Metric/Check/Event — _Depends on:_ B6
- [x] `F2` Notification channel form — Telegram bot token + chat id fields, test-send action
  before saving, against the new `/api/notification-channels` endpoints — _Depends on:_ B7
- [x] `F3` Alert rule form — target source/metric-or-check, condition, threshold, debounce,
  channel picker, against `/api/alert-rules` — _Depends on:_ B7
- [x] `F4` Notifications page (`/notifications`) — combines F2 (channel list/config) and F3
  (rule list/config) per SPEC §5.1 — _Depends on:_ F2, F3
- [x] `F5` Recent Alerts rail — persistent slim panel on Dashboard (`/`) showing active/recently
  resolved alerts, live via F1 — _Depends on:_ F1
- [x] `F6` Extend the status-pulse motif to alert severity on the affected source's status tile
  (SPEC §5.3 — same visual token, alert-severity state) — _Depends on:_ F1
- [x] `F7` Rail nav — add Notifications entry — _Depends on:_ F4

<!-- Test execution is governed by `docs/STACK.md`'s Fast Gate (per task) and Full Gate (per ship).
     Do not duplicate that list here. -->

---

## Files

### Create / modify
~~~
internal/alertrouter/domain/**
internal/alertrouter/application/**
internal/alertrouter/infrastructure/**
internal/alertrouter/interfaces/http/**
internal/notify/telegram/**
internal/telemetry/application/service.go (wire alertrouter port)
internal/telemetry/application/ports.go
internal/platform/wshub/hub.go (alert frame fan-out)
internal/sources/application/service.go (surface unreachable/error transitions to alertrouter, if not already sufficient from change 04's MarkSeen)
cmd/server/main.go (wire alertrouter service + telegram notifier + new HTTP routes)
contracts/openapi.json (regenerated)
web/src/routes/_authenticated/notifications.tsx
web/src/pages/notifications/**
web/src/widgets/recent-alerts-rail/**
web/src/widgets/alert-rule-form/**
web/src/widgets/notification-channel-form/**
web/src/entities/alert/**
web/src/entities/notification-channel/**
web/src/shared/lib/ws-stream-store.ts (alert frame handling)
web/src/widgets/rail-nav/rail-nav.tsx
web/src/widgets/status-tile/status-tile.tsx (alert-severity pulse state)
web/src/shared/api/schema.ts (regenerated)
~~~

### Do NOT touch
- `internal/sources/**`, `internal/auth/**`, `internal/telemetry/**` domain logic beyond the new
  port wiring listed above
- `internal/adapterengine/**`
- `docs/changes/archive/**`

---

## Contracts

See `docs/SPEC.md` §3–§6 (Data Model, API/Backend Contract, Frontend/Client Contract, Auth &
Access Model) and the Files list above. Do not hand-copy schema, endpoint, or type details into
this file.

---

## Gate Checks

> Fast Gate runs per task in `/work`; Full Gate and (with `--release`) Release Gate run once in
> `/ship`. Both are defined in [docs/STACK.md](../../STACK.md) — this section only records
> change-specific overrides.

None.

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- `EvaluateSourceStatus`'s "unreachable" branch (SPEC §6: 3 consecutive failures or 5 min
  continuous) is implemented and unit-tested in `internal/alertrouter/application`, but nothing in
  the current codebase actually emits an "unreachable" status transition after a source's initial
  `Create` default — `internal/adapterengine/application.Runner.RunOnce` doesn't call any status
  hook on subprocess non-zero-exit/timeout (confirmed by reading `runner.go`: only the
  10-consecutive-invalid-lines auto-disable path calls a hook, and it fully disables rather than
  marking `unreachable`). Wiring that is out of this change's scope (`internal/adapterengine/**`
  is on the Do NOT touch list). In practice, this change's live trigger points are the ones
  telemetry ingestion actually produces: a check going `critical` → `error` (immediate alert) and
  back to `ok` (resolved) — which covers SPEC §1.2's own example ("a real alert (e.g. test service
  down...) reaches Telegram... followed by a resolved notification when it clears") via the
  `uptime-http` adapter. `unreachable` alerting will start firing for free once a future change
  wires `adapterengine`'s pull-mode failure path to call `EvaluateSourceStatus`/`MarkSeen` —
  worth a `docs/KNOWN_GOTCHAS.md`-style flag if not picked up soon.

---

## Commit Message

```
feat(change-05): alert router — rule evaluation, firing/resolved lifecycle, Telegram channel
```
