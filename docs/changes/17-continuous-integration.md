# CHANGE 17 — Continuous Integration

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `17` |
| Slug | `continuous-integration` |
| Title | Continuous Integration |
| Status | `active` |
| Branch | `feature/17-continuous-integration` |

---

## Goal

Add a minimal, target-agnostic GitHub Actions pipeline that verifies the Go core, generated API
contract, and web client on pull requests and every push to `main`. Define release verification as
an exact-commit green CI run while the self-contained M11 distribution and deployment target stay
explicitly deferred.

---

## Backlog

### Backend

None.

### Frontend

None.

### Infra

- [x] `I1` Add a least-privilege, concurrency-safe GitHub Actions workflow with immutable action
  pins and reproducible Go, API-contract, and web checks — _Depends on:_ —
- [x] `I2` Replace placeholder local Full/Release Gate rows with the checks CI actually enforces
  and exact-commit GitHub run verification — _Depends on:_ I1

### Data

None.

### Other

- [x] `T1` Update the deploy/CI specification so CI publication is distinct from the deferred M11
  release artifact and management-host deployment — _Depends on:_ I2

---

## Files

### Create / modify

~~~
.github/workflows/ci.yml
docs/STACK.md
docs/SPEC.md
docs/changes/17-continuous-integration.md
~~~

### Do NOT touch

- Product source code, API/schema contracts, adapters, frontend UI, and SQLite data
- Docker distribution packaging or any target-host/infraegev2 deployment automation
- GitHub branch protection, repository secrets, or release tags

---

## Contracts

See `docs/SPEC.md` §7 and the Files list above.

---

## Gate Checks

The workflow must run for pull requests and pushes to `main`, use read-only repository
permissions, cancel superseded runs on the same ref, and pass locally equivalent commands before
publication. Validate its syntax with
`go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 .github/workflows/ci.yml`. After
`--release`, verify the GitHub Actions run for the exact pushed commit SHA.

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

None

---

## Commit Message

~~~
feat(change-17): add reproducible CI and release verification
~~~
