# CHANGE 22 — Analytics Core and Dashboard Redesign

<!-- TOKEN BUDGET: keep this file under 10,000 tokens. Be concise. -->

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `22` |
| Slug | `analytics-core-dashboard-redesign` |
| Title | Analytics Core and Dashboard Redesign |
| Status | `active` |
| Branch | `feature/22-analytics-core-dashboard-redesign` |

---

## Goal

Turn sre-kit into a useful multi-project analytics and observability workspace: accept generic
push telemetry, retain bounded raw and hourly data, expose adapter-authored presentation metadata,
and render the full signal set in a dark minimalist Dashboard and Source detail UI without
Mantine. This change is coordinated with infraegev2 Change 48.

Historical development telemetry may be discarded. No compatibility migration is required, but
the runtime SQLite file must not be deleted until its exact path is shown to and confirmed by the
architect; secrets and unrelated volumes are never reset as part of this change.

---

## Backlog

### Core and integrations

- [x] `B1` Replace the development schema with project-scoped Sources, idempotent raw records,
  hourly metric rollups and retention metadata; implement 30-day raw and 13-month hourly
  maintenance without touching a live database during development — _Depends on:_ —
- [x] `B2` Add owner-only admin password rotation/recovery, project CRUD, bounded telemetry reads,
  presentation-schema responses and an authenticated generic Metric/Check/Event push ingress —
  _Depends on:_ B1
- [x] `B3` Extend `umami-http` with privacy-safe traffic, acquisition, geography, device, page and
  allowlisted event measurements while preserving labels and explicitly classifying traffic as
  `known_bot`, `suspected_automation`, `browser_analytics` or `unclassified` — _Depends on:_ B1

### Frontend

- [x] `F1` Record the product/design contract, replace Mantine/Recharts with Base UI/native
  primitives and uPlot, and preserve the enforced frontend boundaries, accessibility and dark
  minimalist visual language — _Depends on:_ B2
- [x] `F2` Build project-aware, schema-driven Dashboard and Source detail surfaces that expose
  overview KPIs, traffic composition, acquisition, geography, devices, pages, product events,
  infrastructure health, checks, alerts and event feeds with useful loading/empty/error states —
  _Depends on:_ B2, B3, F1

### Verification and contracts

- [x] `T1` Regenerate API types, update manifests/examples/README/SPEC/STACK, add backend and web
  contract tests, run required LSP and browser review, then pass the affected Fast Gate without
  release, deploy or runtime data deletion — _Depends on:_ B1, B2, B3, F1, F2

---

## Files

### Create / modify

~~~
README.md
PRODUCT.md
docs/SPEC.md
docs/STACK.md
docs/FRONTEND_CONVENTIONS.md
docs/changes/22-analytics-core-dashboard-redesign.md
internal/**
adapters/umami-http/**
api/**
web/**
~~~

### Do NOT touch

- Any resolved live SQLite path until the architect confirms the exact destructive reset
- `secrets.enc.json`, environment secrets, target volumes or infraegev2 deployment state
- Target installation, deployment, rollback, backup or credential lifecycle
- Fingerprinting, covert tracking, persistent cross-session browser identifiers or claims that
  inferred traffic is a known human
- Files in sibling infraegev2 (owned by Change 48)

---

## Contracts

- sre-kit stays a generic read-only observability core. A `Project` groups Sources; manifests
  describe measurements and recommended dashboard groups without embedding product-specific UI.
- Push producers authenticate per Source, submit versioned batches with an idempotency key, and
  enter the same validation/storage/alert/live-update pipeline as adapters.
- Raw Metric/Check/Event data is retained for 30 days. Hourly metric aggregates are retained for
  13 months. All list/time-series reads are bounded and expose the selected resolution.
- Browser analytics means an explicitly consented client signal, not proof of personhood. Server
  classifiers may label known/suspected automation; everything else remains unclassified.
- Dashboard and Source detail render all declared measurements. Charts have textual summaries or
  tables so uPlot canvas output is not the only accessible representation.

---

## Gate Checks

Run the affected Fast Gate, generated API drift check, Go and adapter tests, web lint/type/test/build,
TypeScript and Go LSP diagnostics, and authenticated browser journeys for Dashboard, project/source
navigation and Source detail at desktop and narrow widths. Validate every shipped manifest and the
linked infraegev2 example configuration. Do not run `/ship`, deploy or mutate production.

---

## Architect Review Notes

- [x] The architect approved the consolidated scope, dark theme, disposable development data and
  explicit-consent analytics direction in chat.

---

## Implementation Notes

- Change 22 is a clean development baseline and intentionally has no in-place upgrade path from
  the old migration ledger. Activation requires explicit confirmation before resetting
  `/home/niquetamerewsl/.local/share/sre-kit/infraegev2-dogfood/sre-kit.db`; the separate
  `secrets.enc.json` and repository-local `data/sre-kit.db` remain untouched.
- Browser review used a disposable temporary database and covered desktop Dashboard/Source detail,
  narrow navigation and a clean final console; no installed runtime state was mutated.

---

## Commit Message

```
feat(analytics): add project telemetry and redesign dashboards
```
