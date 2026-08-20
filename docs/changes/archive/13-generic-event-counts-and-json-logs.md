# CHANGE 13 — Generic Event Counts and Structured-Log Parsing

<!-- TOKEN BUDGET: keep this file under 10,000 tokens. Be concise. -->

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `13` |
| Slug | `generic-event-counts-and-json-logs` |
| Title | Generic Event Counts and Structured-Log Parsing |
| Status | `archived` |
| Branch | `feature/13-generic-event-counts-and-json-logs` |

---

## Goal

Close two coverage gaps found while evaluating whether infraegev2's local `apps/ops` dashboard can
be retired in favor of sre-kit: a custom Umami event-count breakdown, and pretty-printed structured
(JSON) journal log lines. Both are added as generic, opt-in adapter capabilities — no
project-specific event names or log schemas hardcoded into sre-kit — so any user configures them for
their own needs. Additive-only: new optional `config_schema` fields on two already-shipped adapters,
no `contract.schema.json`/`docs/SPEC.md` change.

---

## Design References

<!-- none provided -->

---

## Backlog

### Backend
- [x] `B1` `umami-http`: add optional `tracked_events: string[]` config field (default empty); for each name, query `GET /api/websites/{id}/events?event=<name>&startAt=&endAt=` within the existing `lookback_seconds` window and emit a generic `analytics.event_count` metric with `labels: {event: "<name>"}` — _Depends on:_ —
- [x] `B2` `journal-http`: add optional `parse_json_message: bool` config field (default false); when true, JSON-decode each entry's `MESSAGE`, flatten top-level scalar fields into the emitted event's `labels`, and prefer a `message`/`msg` key (case-insensitive) as the displayed message text over the raw JSON blob. Unparsable/non-object messages fall back to today's exact behavior — _Depends on:_ —

<!-- Test execution is governed by `docs/STACK.md`'s Fast Gate (per task) and Full Gate (per ship). -->

---

## Files

### Create / modify
~~~
adapters/umami-http/manifest.json   (modify — add tracked_events field)
adapters/umami-http/main.go         (modify — fetch + emit per-event counts)
adapters/umami-http/main_test.go    (modify — cover tracked_events)
adapters/journal-http/manifest.json (modify — add parse_json_message field)
adapters/journal-http/main.go       (modify — JSON message flatten/decorate)
adapters/journal-http/main_test.go  (modify — cover parse_json_message)
~~~

### Do NOT touch
- `internal/**`, `web/**` — pure adapter-level change, no core/UI contract change (config_schema is already rendered generically by the existing schema-driven Add Source form).

---

## Contracts

See `docs/SPEC.md` §4 (adapter contract — unchanged wire shape) and the Files list above.

---

## Gate Checks

None.

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

None

---

## Commit Message

```
feat(change-13): generic Umami event counts + journal-http structured-log parsing
```
