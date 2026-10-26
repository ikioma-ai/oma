---
name: remote-targets
type: feature
priority: 2
slices: [target-config, ssh-remote-operations, tui-target-switching, remote-observability, target-setup-automation]
---

{% section name="problem" %}

Oma manages one Docker daemon: the local one. Deploying to another home machine or a cloud VPS currently requires logging into that machine and running Oma there. The dashboard, host metrics, proxy metrics, backups, verification, and background updates all assume the controller and the target are the same host.

This contract adds first-class remote targets without adding a remote Oma agent. A local Oma binary controls a remote Docker daemon over SSH, while state remains on the target and local behavior remains the default. The contract also preserves seams for a future agent transport and a target-local background runner.

{% /section %}

{% section name="desired-outcome" %}

An operator can define and select targets, deploy and manage applications on another machine, and see which machine the current TUI session controls. Remote targets require Docker and SSH only. The local target remains the default and behaves as it does today.

Remote metrics come from the target or are explicitly marked unavailable. Verification is skippable and never destroys app data. Backup ownership and background-update behavior are visible rather than implied. Guided setup can prepare a supported Linux host for Docker access, but manual setup remains first-class and no remote Oma binary or service is installed.

{% /section %}

{% section name="context" %}

This contract connects target configuration, command and TUI entry points, Docker namespace construction, telemetry, verification, backups, setup, and background automation.

### Cross-references

- `docs/remote/shaping.md` — requirements, selected shape, fit check, parts, and breadboard.
- `docs/remote/slices.md` — V1–V5 slices, sliced breadboard, and acceptance criteria.
- `docs/oma-fork-plan.md` — Phase 3 roadmap and deferred work.
- `specs/app-catalog.md` — app install and apply operations that must work identically against a selected target.

### Implementation reference

- `internal/docker/namespace.go` — namespace construction and the local Docker client.
- `internal/command/root.go` — persistent flags and command wiring.
- `internal/ui/app.go` — TUI startup, dashboard construction, and telemetry scrapers.
- `internal/system/scraper.go` — local host CPU, memory, and disk sampling.
- `internal/metrics/scraper.go` — proxy metrics fetched from a local port.
- `internal/docker/application.go` — deploy, verification, and settings apply behavior.
- `internal/background/runner.go` — update and backup scheduling.
- `internal/service/systemd.go` and `internal/service/launchd.go` — background service installation.
- `docs/remote/shaping.md` and `docs/remote/slices.md` — shape and slice ground truth.

### Key architectural decisions

- **Controller-driven Docker over SSH** — OpenSSH streamlocal forwarding to the remote Docker socket; no remote Oma.
- **Local target is default** — the existing local client path remains a separate, tunnel-free implementation.
- **Transport seam** — commands and TUI depend on a target transport contract, so a future agent transport does not rewrite command semantics.
- **Background-runner seam** — background jobs accept a target; a future target-local runner can reuse the same state and lock.
- **Explicit ownership** — controller-side backups and controller-driven automation are labeled; remote-side variants are deferred.
- **Non-destructive verification** — a failed check removes the new deployment but preserves the data volume.

{% /section %}

{% section name="technical-direction" %}

- Target configuration is a YAML file at `$XDG_CONFIG_HOME/oma/config.yaml`, falling back to `~/.config/oma/config.yaml`. It contains named targets and a default target. [AC#1]
- A target declares a name, an SSH destination, a Docker socket path, and a namespace. The local target may be implicit and requires no SSH fields. [AC#1]
- No password, private key, token, or passphrase is stored in the target configuration. SSH authentication uses the user's existing agent, identity files, and configuration. [AC#1,AC#6]
- Target selection precedence is `--target`, then `OMA_TARGET`, then the configured default. Unknown target names fail before any Docker call. [AC#1]
- The local target must not start SSH and must preserve existing local Docker behavior. [AC#1,AC#9]
- Target resolution returns a transport implementation. The local transport returns the existing Docker client; the SSH transport starts a tunnel and returns a Docker client connected to the local socket. A future agent transport may implement the same boundary. [AC#2,AC#10]
- The SSH transport uses a short local socket and OpenSSH streamlocal forwarding to the remote Docker socket. It waits for readiness, fails on a missing or unusable SSH client, and removes the socket when the operation ends. [AC#2,AC#7]
- Remote operations must use the injected target client. They must not read or mutate local Docker state. [AC#2]
- SSH host-key and authentication failures, unreachable hosts, missing Docker, stopped Docker, and socket permission denial must produce distinct, actionable errors. [AC#1,AC#2]
- The TUI must show the active target, allow switching, and rebuild namespace, apps, and event subscriptions on a successful switch. Target-aware telemetry is delivered by the observability requirement. A failed switch leaves the previous target active. [AC#3,AC#4]
- Proxy metrics for a remote target must come from the target proxy. Host CPU, memory, and disk must come from the target or be explicitly marked unavailable; controller values must never be shown as remote values. [AC#4]
- Verification must be skippable for deploy and apply. A failed verification removes the new deployment but preserves the app data volume. [AC#5]
- Remote backups default to a controller-side path and must be labeled as such in the TUI. A target-side path must not be implied. [AC#8]
- `background run --target` runs on the controller against the selected target. It must record the target and controller-side backup path and must state that a stopped controller stops automation. [AC#8]
- Interactive and background operations must acquire a shared lock before deploy, update, or backup. The lock must be released on completion and cancellation. [AC#7]
- `oma target setup` is the explicit mutating bootstrap path. It must be confirmed, idempotent, Linux-only for automated bootstrap, and must not install a remote Oma binary or service. [AC#6]
- Guided setup may install Docker and grant Docker-socket access through the remote user's sudo privileges. Without sudo/root it must fail with exact manual commands. [AC#6]
- Setup may report DNS and port 80/443 readiness but must not change DNS records or firewall rules automatically. [AC#6]
- Prefer the user's OpenSSH binary over a new SSH implementation. (guidance)
- Keep the local target code path separate from the SSH transport. (guidance)
- Use short local socket paths to respect platform limits. (guidance)

{% /section %}

{% section name="domain-rules" %}

- **R1:** The local target remains the default and does not start an SSH tunnel.
- **R2:** All app and proxy state for a target lives in that target's Docker daemon. Switching targets switches state.
- **R3:** Target configuration stores no SSH passwords, private keys, tokens, or passphrases.
- **R4:** Remote transport is SSH-based; the Docker TCP port is not exposed or required.
- **R5:** A remote target requires Docker only; no remote Oma binary or service is part of this contract.
- **R6:** Remote host and proxy metrics come from the target or are explicitly unavailable.
- **R7:** Verification failure never removes the app data volume.
- **R8:** Backup ownership is visible; remote targets default to controller-side backups.
- **R9:** One operation lock per target prevents concurrent deploy, update, and backup work.
- **R10:** Target setup is explicit, confirmed, idempotent, and never stores credentials.
- **R11:** A failed TUI target switch leaves the previous target active.
- **R12:** Background automation respects the selected target and the operation lock.
- **R13:** Target names are unique and the configured default must exist.
- **R14:** Setup does not modify DNS records or firewall rules.
- **R15:** Command and TUI behavior do not depend on the concrete transport implementation.
- **R16:** Remote agent transport and target-local background runner are outside this contract, but their seams remain possible.

{% /section %}

{% section name="edge-cases-and-failure-modes" %}

- **Missing config:** An absent configuration file produces an implicit local target and does not write a file.
- **Unknown target:** Selection fails before any Docker call with the available target names.
- **Removing the default target:** The removal is rejected unless the operator explicitly selects a new default.
- **Unknown SSH host key:** The operation fails closed with instructions; no automatic host-key acceptance.
- **SSH authentication failure:** The error distinguishes authentication from host-key and network failures.
- **Docker missing:** The target reports Docker as absent and suggests setup or manual installation.
- **Docker stopped or unreachable:** The error identifies the daemon state rather than reporting a generic connection failure.
- **Docker socket permission denied:** The error explains docker-group or root requirements; it does not silently fall back to local Docker.
- **Tunnel exits mid-operation:** The operation fails, the local socket is cleaned up, and the next operation starts a fresh tunnel.
- **Long local socket path:** The tunnel uses a short temporary path and removes it on exit or cancellation.
- **Remote metrics unavailable:** The dashboard shows an explicit unavailable state; it never substitutes controller metrics.
- **Failed verification:** The new deployment and proxy route are removed, while the data volume remains.
- **Setup without sudo:** Setup stops before mutation and prints exact manual commands.
- **Non-Linux bootstrap target:** Automated setup is refused with guidance; manual setup may still work if Docker and SSH permissions already exist.
- **Controller offline:** Remote apps are not updated or backed up by a controller-driven runner; the UI and documentation state this.
- **Lock already held:** A second deploy, update, or backup reports the owning operation and fails without changing state.
- **Stale lock:** A lock older than its lease is reported as stale and can be reclaimed explicitly; it is not silently ignored forever.
- **Switch during an operation:** The TUI prevents switching while an operation is active or cancels it with confirmation.

{% /section %}

{% section name="examples" %}

### Example 1: Manual VPS setup and deploy

- **Setup:** The operator has SSH access to a VPS and installs Docker plus docker-group access manually.
- **Action:** The operator runs `oma target add vps --ssh deploy@vps`, `oma target check vps`, then `oma --target vps deploy ghcr.io/foo/bar --host app.example.com`.
- **Result:** The app is deployed on the VPS; local Docker state is unchanged; the target is listed as non-default.

### Example 2: Guided setup

- **Setup:** A fresh Linux VPS has SSH access and a sudo-capable user, but no Docker.
- **Action:** The operator runs `oma target setup vps --ssh deploy@vps --yes`.
- **Result:** Oma installs Docker, grants socket access, reconnects, verifies the socket, reports DNS/firewall readiness, and saves the target. No remote Oma is installed.

### Example 3: TUI target switch

- **Setup:** The laptop and VPS are configured targets, with the laptop as default.
- **Action:** The operator switches to the VPS in the dashboard.
- **Result:** The header shows the VPS target, the app list shows VPS apps, metrics come from the VPS, and switching back restores the laptop state.

### Example 4: Remote observability

- **Setup:** A remote target is active.
- **Action:** The operator opens the dashboard.
- **Result:** Proxy request metrics come from the remote proxy; target host metrics are shown or explicitly marked unavailable; controller metrics are never displayed.

### Example 5: Safe verification failure

- **Setup:** A new deployment fails its post-deploy health check.
- **Action:** The operator used `--no-verify` for one deploy and, separately, allows verification on another.
- **Result:** The skipped deploy remains running; the failed verified deploy is removed without deleting its data volume.

### Example 6: Operation lock contention

- **Setup:** A controller-driven backup is running against a target.
- **Action:** The operator starts a deploy against the same target.
- **Result:** The deploy reports that the backup holds the operation lock and does not change the target.

{% /section %}

{% section name="boundaries" %}

- **In scope:** Target configuration and resolution; local default preservation; SSH Docker transport; remote app operations; TUI target switching; target-aware metrics; `--no-verify`; non-destructive verification; controller-side backup labeling; target check/setup; controller-driven `background run --target`; operation lock; transport and runner seams.
- **Out of scope:** Remote Oma agent or control API; target-local background runner; fleet scheduling or multi-host placement; Docker TCP/TLS transport; remote-side backup storage; Windows controller support; automatic DNS or firewall changes.
- **Do not modify:** Existing app and proxy state formats beyond an operation-lock entry; existing catalog install/apply semantics; the local Docker client path; public app labels and backup archive layout.
- This is a runtime-component contract; integration acceptance criteria apply.

{% /section %}

{% section name="acceptance-criteria" %}

- [ ] AC#1 (integration): Given no target configuration, when the operator runs `oma target list` and `oma --target local list`, then `local` is the default and behaves as before; after adding a named target, `--target`, `OMA_TARGET`, and the configured default resolve in that order, and an unknown target fails before any Docker call.
- [ ] AC#2 (integration): Given a configured SSH target, when the operator runs list, deploy, start, stop, update, logs, exec, backup, restore, remove, and settings commands against it, then each operation succeeds through the remote Docker daemon, remote state changes are visible, and local Docker state is unchanged.
- [ ] AC#3 (integration): Given a running TUI on the local target, when the operator switches to a remote target, then the header, app list, and event subscriptions reflect the remote target; when the switch fails, the previous target remains active. Target-aware metrics are verified by AC#4.
- [ ] AC#4 (integration): Given an active remote target, when the dashboard loads telemetry, then proxy metrics come from the target proxy, host metrics come from the target or show an explicit unavailable state, and controller metrics are never labeled as remote.
- [ ] AC#5 (integration): Given a remote deploy or apply, when `--no-verify` is used, then post-deploy verification is skipped; when verification runs and fails, then the new deployment is removed and the app data volume remains.
- [ ] AC#6 (integration): Given a supported Linux host with SSH and sudo access, when `oma target setup` runs with confirmation, then it installs Docker if missing, grants socket access, reconnects, verifies the socket, and reports DNS/firewall readiness without installing remote Oma; without sudo it fails with manual commands; on unsupported platforms automated setup is refused.
- [ ] AC#7 (integration): Given two operations on the same target, when one holds the operation lock for deploy, update, or backup, then the other fails without changing target state, and cancellation or completion releases the lock; a stale lock is reported and can be reclaimed explicitly.
- [ ] AC#8 (integration): Given a remote target, when the operator views backup settings and runs `background run --target`, then the backup path is labeled controller-side, the background run records the target, and the UI/docs state that a stopped controller stops automation.
- [ ] AC#9: Given the local target, when the full deploy, backup, restore, settings, and TUI flows run, then behavior matches the pre-target contract and no SSH process or remote metrics are used.
- [ ] AC#10: Given the target boundary, when the local and SSH transports are exercised, then command and TUI semantics are identical and the boundary leaves room for a future agent transport and target-local runner without changing app or state formats.
- [ ] AC#11: Given the repository, when the unit and CLI test suites run, then they pass and the existing catalog tests remain unchanged.

{% /section %}

{% slice name="target-config" %}

Deliver the target configuration, resolver, local target, and CLI target management. The transport boundary exists, with the local implementation only.

**Acceptance Criteria:**
- Configuration loads from `$XDG_CONFIG_HOME/oma/config.yaml` or `~/.config/oma/config.yaml`; missing config yields an implicit `local` target.
- `oma target add`, `target list`, and `target remove` work for named targets.
- `--target`, `OMA_TARGET`, and the configured default resolve in that order.
- Unknown targets fail before any Docker call.
- Removing the default target is refused unless a new default is selected.
- The local target uses the existing Docker client path and starts no SSH.
- Target resolution returns a transport contract; the local implementation is the only implementation in this slice.
- CLI and unit tests pass.

{% /slice %}

{% slice name="ssh-remote-operations" %}

Deliver the SSH transport and remote Docker operations. Commands and TUI code continue to use the same target contract.

**Acceptance Criteria:**
- An SSH target stores a destination, Docker socket, and namespace.
- The SSH transport uses a short local socket and OpenSSH streamlocal forwarding, waits for readiness, and cleans up on exit or cancellation.
- Docker operations use the remote client and do not read or mutate local Docker state.
- List, deploy, start, stop, update, logs, exec, backup, restore, remove, and settings work against a remote daemon.
- SSH host-key/auth failures, missing/stopped Docker, and socket permission denial produce distinct errors.
- A basic connection probe reports SSH, Docker, and socket results.
- The local target path remains unchanged.
- Unit, CLI, and integration tests pass, including a real SSH target when available.

{% /slice %}

{% slice name="tui-target-switching" %}

Deliver the active-target display and target switcher. The dashboard belongs to exactly one target at a time.

**Acceptance Criteria:**
- The dashboard header shows the active target.
- The switcher lists configured targets and the default.
- A successful switch rebuilds namespace, apps, and event subscriptions; target-aware metrics follow in `remote-observability`.
- A failed switch leaves the previous target active and shows an actionable error.
- Switching is blocked or confirmed while an operation is active.
- TUI tests cover the header, switcher, successful switch, and failed switch.

{% /slice %}

{% slice name="remote-observability" %}

Deliver target-aware telemetry and safety behavior for remote targets.

**Acceptance Criteria:**
- Proxy metrics are read from the target proxy.
- Host CPU, memory, and disk are read from the target or explicitly marked unavailable; controller values are never shown for a remote target.
- `--no-verify` skips verification for deploy and apply.
- A failed verification removes the deployment and preserves the data volume.
- Backup paths are labeled controller-side for remote targets.
- Metrics and backup labels update after a target switch.
- Tests cover remote metrics, unavailable fallback, skipped verification, and non-destructive failure.

{% /slice %}

{% slice name="target-setup-automation" %}

Deliver explicit target setup, read-only target checks, and controller-driven background automation with the operation lock.

**Acceptance Criteria:**
- `target add` does not mutate the remote host.
- `target check` is read-only and reports SSH, Docker, socket, and provisioning readiness.
- `target setup` is explicit, confirmed, idempotent, and Linux-only for automated bootstrap.
- Setup installs Docker when missing, grants docker-group access when permission is denied, and reconnects to verify.
- Setup without sudo/root fails with exact manual commands and never stores credentials.
- Setup reports DNS and ports 80/443 readiness without changing firewall or DNS.
- `background run --target` uses the selected target and records the target and controller-side backup path.
- Interactive and background operations acquire the shared operation lock and release it on completion or cancellation.
- The UI/docs state that a stopped controller stops remote automation.
- Tests cover setup and check outcomes, background target selection, lock contention, and offline-controller messaging.

{% /slice %}
