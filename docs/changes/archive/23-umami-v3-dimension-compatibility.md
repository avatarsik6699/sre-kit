# CHANGE 23 — Umami v3 Dimension Compatibility

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `23` |
| Slug | `umami-v3-dimension-compatibility` |
| Title | Umami v3 Dimension Compatibility |
| Status | `archived` |
| Branch | `feature/23-umami-v3-dimension-compatibility` |

---

## Goal

Restore the `umami-http` pull source against the deployed Umami 3.2 API without changing sre-kit's
public `url` dimension or telemetry metric contract. The adapter must translate that stable local
dimension to the upstream API's accepted page-path type and retain focused regression coverage.

---

## Backlog

### Backend
- [x] `B1` Map the configured `url` dimension to Umami 3.2's `path` metrics type while preserving `analytics.url_count`, and add focused request/output regression tests — _Depends on:_ —

### Frontend
None

### Infra
None

### Data
None

---

## Files

### Create / modify
~~~
adapters/umami-http/main.go
adapters/umami-http/main_test.go
~~~

### Do NOT touch
- `adapters/umami-http/manifest.json` and the public dimension/metric contract
- Core API, frontend, database schema, and other adapters

---

## Contracts

See `docs/SPEC.md` §3–§4 and the Files list above.

---

## Gate Checks

```bash
go test ./adapters/umami-http
```

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

None

---

## Commit Message

```
feat(change-23): restore Umami v3 dimension collection
```
