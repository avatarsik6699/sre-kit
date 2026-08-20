# Known Gotchas

> Project memory file. Capture recurring pitfalls that repeatedly waste time during coding,
> testing, or deploys.

## How To Use

- Add only issues that are likely to happen again.
- Prefer concrete symptoms, root cause, and the shortest reliable fix.
- Remove entries that are no longer relevant.

## Gotcha Log

### Cross-repository observability changes can leave a live-only contract

- **Symptoms**: infraegev2 monitoring works after a manual source/database adjustment, but the two
  repositories describe different Source/endpoints; later agents either add target deployment to
  sre-kit or revive a second monitoring dashboard in infraegev2.
- **Root cause**: infraegev2 and sre-kit are separate repositories even though both are first-party
  parts of one operational system.
- **Fix**: create coordinated active Backlog items in both repositories. Keep adapters,
  normalization, alerts and monitoring UI in sre-kit; keep target installation, credentials,
  Compose/systemd lifecycle and backup/restore in infraegev2 `ops/`. Integrate only through
  Source registration and Metric/Check/Event ingestion contracts. Current pair: sre-kit Change 19
  and infraegev2 Change 44; neither may claim six-Source live proof until registration, polling and
  dashboard evidence exist.

### Retired provisioning tables are intentionally inert

- **Symptoms**: an upgraded SQLite database still contains `hosts`, `provisioning_runs` and
  `sources.host_id` although the API and UI no longer expose deployment.
- **Root cause**: Change 15 removes the write-capable product boundary without running destructive
  SQLite cleanup against user data created by the experimental prototype.
- **Fix**: leave migration `0003` and its data unchanged. Inventory it before proposing a separate
  forward-only cleanup migration; never re-enable Host/provisioner runtime merely because the
  tables exist.

### Docker-owned files break host operations (`EACCES` / `EPERM` / read-only)

> Keep this entry only if the project uses Docker bind mounts. Otherwise delete it.

- **Symptoms**: file operations fail with `EACCES`, `EPERM`, "Permission denied", or "Read-only file system". Most common paths: container-generated build/cache directories on the host (`.nuxt/`, `.output/`, `node_modules/.cache/`, `__pycache__/`).
- **Root cause**: a Docker container wrote to a bind-mounted host directory as root.
- **Fix (host)**:
  ```bash
  sudo chown -R $USER:$USER <path>   # reclaim ownership, keep files
  sudo rm -rf <path>                 # OR discard the generated artefact
  ```
- **Agent protocol**: agents MUST NOT run `sudo`, `chmod -R 777`, or loop the failing operation. Instead, stop and post this exact handoff to the user (substituting real `<path>` and `<cmd>`):

  > ⛔ **Permission denied.** I cannot modify `<path>` while running `<cmd>`.
  >
  > This usually happens when a Docker container wrote files to a bind-mounted host directory as root. Please run one of the following on the host:
  >
  > ```bash
  > sudo chown -R $USER:$USER <path>
  > sudo rm -rf <path>
  > ```
  >
  > When the fix is applied, reply with the single word **`continue`** and I will retry the failed operation from the same step.

  On receiving `continue` (case-insensitive), retry the failed operation once. If it fails a second time with the same error, stop again and ask the user to confirm the fix was actually applied — do not loop a third time.

- **Prevention**: run Docker with a matching host UID/GID or use named volumes for cache directories that containers own.

### `host-metrics-ssh` skips SSH host key verification (v1)

- **Symptoms**: not a failure today — flagged so it isn't mistaken for an oversight later. The
  adapter (`adapters/host-metrics-ssh/main.go`) dials with `ssh.InsecureIgnoreHostKey()`, so it
  will happily connect to a spoofed host (MITM) with no warning.
- **Root cause**: v1 has no `known_hosts` store or first-connection TOFU (trust-on-first-use)
  pinning mechanism; building one was out of scope for `M2` (docs/changes/02-host-metrics-ssh-adapter.md).
- **Fix**: none yet — accepted for the current first-party dogfood, but do not describe the adapter
  connection as pinned merely because infraegev2's administrative SSH wrapper is pinned. Revisit
  before shared/team use, third-party adapters or an always-on M11 deployment (e.g. persist the
  host key fingerprint on first successful connect and fail loudly on mismatch after).
- **Prevention**: n/a until a TOFU/pinning implementation exists.

### Pull-adapter outcome reporting must not depend on emitted telemetry

- **Symptoms**: before Change 16, a source whose adapter exited non-zero kept its prior status, and
  a successful event adapter with no records (for example, no recent bans) remained
  `unreachable`. Connectivity-alert debounce therefore never saw either failure or recovery.
- **Root cause**: Source status was updated only as a side effect of telemetry ingestion. A pull
  invocation itself had no outcome port, even though both an empty success and a subprocess error
  carry source-level connectivity information.
- **Fix**: `Scheduler` now reports one normalized outcome after every pull. Spawn/non-zero failures
  become `unreachable`, invalid output becomes `error`, and an empty success becomes `ok`.
  Successful pulls that emitted telemetry resolve connectivity alerts without overwriting the
  finer Source status already derived from that telemetry.
- **Prevention**: every new pull execution path must exercise quiet success, emitted success,
  subprocess failure and invalid-output cases in `scheduler_test.go`; never infer connectivity
  solely from whether an adapter emitted a Metric, Check or Event.

<!--
### [Title — short, punchy, searchable]

- **Symptoms**: [what fails, what error message]
- **Root cause**: [why it happens]
- **Fix**: [shortest reliable fix]
- **Prevention**: [optional — how to avoid hitting it again]
- **Links**: [optional — docs / issue / PR]
-->
