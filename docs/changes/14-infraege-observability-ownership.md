# CHANGE 14 — infraege Observability Ownership

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `14` |
| Slug | `infraege-observability-ownership` |
| Title | infraege Observability Ownership |
| Status | `active` |
| Branch | `feature/14-infraege-observability-ownership` |

---

## Goal

Establish `sre-kit` as the first-party sibling repository that owns the observability core and
deployment capabilities used by infraegev2. Define the cross-repository boundary and coordination
rule without moving the currently running Beszel/Umami services or reviving infraegev2 `apps/ops`.

---

## Backlog

### Backend
None

### Frontend
None

### Infra
- [x] `I1` Document the first-party infraegev2 relationship, ownership boundary and coordinated-change rule in SPEC, STACK, README and KNOWN_GOTCHAS — _Depends on:_ —

### Data
None

### Other
None

---

## Files

### Create / modify
~~~
README.md
docs/SPEC.md
docs/STACK.md
docs/KNOWN_GOTCHAS.md
docs/changes/14-infraege-observability-ownership.md
~~~

### Do NOT touch
- Go/TypeScript source, API/schema contracts, migrations, adapters or presets
- Runtime data, encrypted secrets or live deployments
- infraegev2 application Compose ownership without a separate coordinated change

---

## Contracts

See `docs/SPEC.md` §1.3, §7 and §12.5 and the Files list above. No API, schema or secret contract
changes.

---

## Gate Checks

Documentation formatting and contradiction search only; code Fast Gate rows are not applicable.

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

None

---

## Commit Message

```
docs(change-14): define first-party observability ownership
```
