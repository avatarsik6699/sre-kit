# CHANGE 18 — CI Clean Checkout Reproducibility

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `18` |
| Slug | `ci-clean-checkout` |
| Title | CI Clean Checkout Reproducibility |
| Status | `active` |
| Branch | `feature/18-ci-clean-checkout` |

---

## Goal

Fix the two environment assumptions exposed by Change 17's first remote run: generate TanStack's
ignored route tree before TypeScript checks and use the maintained pnpm v10 setup action runtime.
Keep the correction limited to CI and its documented local gate order.

---

## Backlog

### Backend

None.

### Frontend

None.

### Infra

- [x] `I1` Run the web build before type-checking so a clean checkout generates
  `web/src/routeTree.gen.ts` before TypeScript consumes it — _Depends on:_ —
- [x] `I2` Upgrade the immutable pnpm setup pin to the maintained v6 release and eliminate the
  deprecated action-runtime warning — _Depends on:_ —
- [x] `I3` Reproduce the workflow's build-before-typecheck sequence from a clean generated-file
  state and validate the final workflow syntax locally — _Depends on:_ I1, I2

### Data

None.

### Other

None.

---

## Files

### Create / modify

~~~
.github/workflows/ci.yml
docs/STACK.md
docs/changes/18-ci-clean-checkout.md
~~~

### Do NOT touch

- Product source, routes, generated API contract, adapters, or persisted data
- M11 distribution/deployment scope

---

## Contracts

See `docs/SPEC.md` §7 and the Files list above.

---

## Gate Checks

Temporarily remove the ignored `web/src/routeTree.gen.ts`, run the documented build-before-
typecheck sequence, validate the workflow with
`go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 .github/workflows/ci.yml`, and require
the exact corrective SHA's GitHub Actions run to complete successfully.

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- Change 17's first run (`32387075037`) correctly failed because the local checkout already had
  the ignored generated route tree; it was not valid clean-checkout evidence.

---

## Commit Message

~~~
fix(change-18): make CI reproducible from a clean checkout
~~~
