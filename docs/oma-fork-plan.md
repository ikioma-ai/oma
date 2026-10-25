# Oma Fork Plan

Status: accepted; fork initialized at the repository root
Upstream: `basecamp/once` at `v0.3.4` (`864ec7c`, 2026-10-07)
Purpose: record what ONCE does today, what Oma must add, and how to fork and maintain it without a rewrite.

## 1. Decision

Fork ONCE. Do not build a clean-room replacement.

Reasons:

- Oma's requirements are additive: a data-driven app catalog and remote machine targets. Neither changes ONCE's core model.
- ONCE is approximately 24,000 lines of Go including tests, with 354 commits and a 1,545-line Docker integration suite.
- The TUI alone is 65 Go files. Backup/restore, zero-downtime proxy handoff, registry auth, service installation, self-update, and installer behavior are already implemented and tested.
- The app contract is too narrow, but the deployment engine underneath it is sound.
- MIT permits forking and rebranding. Retain the original copyright notice, add modification notices, and note that MIT does not grant trademark rights.

A rewrite is justified only if the product becomes multi-tenant, multi-host/Kubernetes-native, supports non-Docker runtimes, or requires a different implementation language. None of those apply to the current goal.

### 1.1 Repository state

The ONCE checkout has been moved from `reference/once` to the repository root, preserving its full git history at `v0.3.4` (`864ec7c`).

Remotes:

- `origin`: `git@github.com:ikioma-ai/oma.git` — push target.
- `upstream`: `git@github.com:basecamp/once.git` — read-only upstream.

Current baseline, recorded 2026-10-25:

- `go build ./...` passes with Go 1.27.0 on darwin/arm64.
- `go test ./internal/...` passes with `TZ=UTC` and a short temporary directory such as `TMPDIR=$HOME/.tmp`.
  - Without `TZ=UTC`, `TestTrimBackups` fails because the test parses backup filenames as UTC while retention compares against local time.
  - With the default macOS long temp path, `TestConnectionError` fails because the generated unix socket path exceeds the platform limit.
  - These are upstream test-environment defects, not regressions from the move.
- Docker 29.4.0 is available. The integration suite has not been run yet.
- `docs/oma-fork-plan.md` is currently untracked and should be included in the first fork commit.

---

## 2. Findings: what ONCE does today

### 2.1 Runtime architecture

- One instance manages one Docker daemon and one or more namespaces.
- All Docker access goes through `client.New(client.FromEnv)` in `internal/docker/namespace.go:48`.
  - `DOCKER_HOST`, `DOCKER_TLS_VERIFY`, and `DOCKER_CERT_PATH` already work.
  - There is no first-class target or remote-machine concept.
- Container naming:
  - Proxy: `{namespace}-proxy`
  - App: `{namespace}-app-{appName}-{6hex}`
  - Volume: `{namespace}-app-{appName}`
- `kamal-proxy` handles host routing, automatic TLS, and zero-downtime handoff.
- Deploy sequence in `internal/docker/application.go:332`:
  1. Create a new app container.
  2. Start it.
  3. Call `kamal-proxy deploy --target <container-id> --host <host> [--tls]`.
  4. Wait for the proxy health check.
  5. Remove old containers.
- Configuration state:
  - App settings are JSON in the Docker label `once` on each container (`internal/docker/application.go:508`).
  - Proxy settings and operation timestamps are in `once-state.json` inside the proxy volume (`internal/docker/proxy.go:233`).
  - Backups contain app settings, legacy volume settings, and the primary mount's data (`internal/docker/application_backup.go:163-232`).
- Background service (`internal/background/runner.go`):
  - Runs every 5 minutes.
  - Self-update, app update, and backup are due every 24 hours (`AutomaticTaskInterval`, `internal/docker/application.go:51`).
  - Installed via systemd on Linux or launchd on macOS.
- Self-update downloads the latest GitHub release binary and replaces the executable without checksum or signature verification (`internal/version/updater.go`).

### 2.2 Installer architecture

- `installer/main.go` is a Go HTTP service deployed at `get.once.com`.
- Routes:
  - `GET /up`
  - `GET /{image...}`: validates the image reference with `distribution/reference`, then renders `installer/templates/install.sh`.
- `install.sh`:
  - Detects OS and architecture.
  - Installs Docker if absent.
  - Downloads `once-{os}-{arch}` from the latest GitHub release.
  - Installs to `/usr/local/bin/once`.
  - Installs the background service.
  - Runs `once --install=<image>`.
- The installer does not verify `checksums.txt`, although `make dist` generates it (`Makefile:49`).
- The app list is not served by the installer. The installer only carries an image reference into the binary.

### 2.3 Current built-in app catalog

Hardcoded in `internal/ui/install_app_list.go:16`:

```go
var knownApps = []KnownApp{
    {Name: "Campfire", Alias: "campfire", ImageRef: "ghcr.io/basecamp/once-campfire"},
    {Name: "Fizzy", Alias: "fizzy", ImageRef: "ghcr.io/basecamp/fizzy"},
    {Name: "Writebook", Alias: "writebook", ImageRef: "ghcr.io/basecamp/writebook"},
}
```

Consequences:

- Adding an app to the TUI requires a code change and a new binary release.
- Metadata is limited to name, alias, and image.
- Aliases are expanded in `expandAlias` for the TUI and `--install` path.
- Arbitrary images are already accepted through the custom image screen or `--install=<image>`.

### 2.4 Current app contract and its limits

| Contract | Current behavior | Source |
| --- | --- | --- |
| HTTP only | App must be reachable through kamal-proxy over HTTP | `internal/docker/proxy.go` |
| Container port | Bare container ID is passed to the proxy, so the target port defaults to 80 | `internal/docker/application.go:374`, `internal/docker/proxy.go:219` |
| Health check | Fixed at `/up` | `internal/docker/application.go:52`, `internal/docker/application.go:427` |
| Persistent data | One volume mounted at `/storage` and `/rails/storage` | `internal/docker/application.go:58`, `internal/docker/application.go:466` |
| Backup coverage | Only the first mount target is backed up | `internal/docker/application_backup.go:225` |
| Injected env | `SECRET_KEY_BASE`, VAPID keys, `BASE_URL`, `DISABLE_SSL`, SMTP, `NUM_CPUS` | `internal/docker/application.go:478-506` |
| Install-time configuration | Only image and hostname; env vars are available later in settings | `internal/ui/install_activity.go:157-210` |
| Container command/user | Not configurable | `internal/docker/application.go:508` |
| Multi-container apps | Not supported; one `Application` owns one container | `internal/docker/application.go` |
| Verification failure | Removes the app and destroys its data volume | `internal/docker/application.go:236-245` |
| Hostname | Required; TLS enabled unless `*.localhost` | `internal/docker/application_settings.go:172` |
| Updates | Pulls the configured image tag; daily auto-update | `internal/docker/application.go:175`, `internal/background/runner.go` |

The proxy itself already supports the missing deployment controls:

- `--target <host>:<port>`
- `--health-check-path`
- `--health-check-port`
- `--health-check-timeout`
- `--health-check-interval`

ONCE does not expose them.

### 2.5 Remote-machine gap

There is no target abstraction, but the state model is remote-friendly: all app and proxy state lives in the target daemon's labels and volumes.

A zero-code escape hatch already works:

```sh
ssh -fNL /tmp/vps-docker.sock:/var/run/docker.sock root@vps
DOCKER_HOST=unix:///tmp/vps-docker.sock once --install=ghcr.io/basecamp/once-campfire
```

Deploy, list, start, stop, exec, logs, and backup work through the Docker API. What breaks:

- `internal/system/scraper.go` reads the controller's CPU, memory, and disk, not the target's.
- `internal/metrics/scraper.go:102` fetches `http://127.0.0.1:<metricsPort>/metrics` on the controller.
- The background service needs the same `DOCKER_HOST`.
- The installer always checks and installs Docker on the local machine, even for a controller-only setup.

### 2.6 Security and correctness risks to address

- `VerifyHTTPOrRemove` deletes the data volume on a DNS or HTTP verification failure.
- Installer and self-updater downloads are not checksum- or signature-verified.
- Secrets, including registry and SMTP credentials and generated keys, are stored in Docker labels and visible to anyone with Docker access.
- A remote catalog would be executable supply-chain input: image, env, volumes, and lifecycle settings.
- Docker socket access is root-equivalent; SSH targets do not reduce that privilege.
- `Backup.Path` is an absolute host path. With both a controller and a remote runner, ownership of the backup directory is ambiguous.

---

## 3. Required capabilities

### 3.1 Data-driven app catalog

#### 3.1.1 Manifest format

Use YAML for authoring and JSON for the installed/remote catalog. Schema:

```yaml
id: writebook
name: Writebook
description: Publish books from Markdown
category: publishing
image: ghcr.io/basecamp/writebook
version: latest                  # optional tag or digest
hostname: books.example.com      # suggested value, not enforced

web:
  port: 80                       # container port
  healthPath: /up
  healthPort: 80                 # defaults to port
  healthTimeout: 5s
  tls: auto                      # auto | off
  verify: true

storage:
  - name: data
    path: /storage
    backup: true
    primary: true
  - name: data
    path: /rails/storage

env:
  - name: SECRET_KEY_BASE
    generate: hex32
  - name: BASE_URL
    value: "{{ base_url }}"
  - name: DATABASE_URL
    prompt: PostgreSQL URL
    secret: true
    required: true
  - name: SMTP_ADDRESS
    from: smtp.server

hooks:
  preBackup: /hooks/pre-backup
  postRestore: /hooks/post-restore

resources:
  cpus: 0
  memoryMB: 0
```

Validation rules:

- `id`: `^[a-z0-9][a-z0-9-]*$`, unique after merge.
- `image`: valid normalized reference via `distribution/reference`.
- Volume paths: absolute container paths only. No host bind mounts.
- Env names: `^[A-Za-z_][A-Za-z0-9_]*$`.
- Health paths: absolute, no query strings.
- Ports: 1 through 65535.
- Hooks and volumes must not escape the container.
- Unknown fields should fail validation rather than be silently ignored.

#### 3.1.2 Resolution order

1. Embedded `apps/*.yaml` compiled into the binary.
2. `~/.config/once/apps/*.yaml` (or `$XDG_CONFIG_HOME/once/apps`).
3. `$ONCE_CATALOG_PATH`, which may be a file, directory, or URL.
4. Optional remote `ONCE_CATALOG_URL`, cached with ETag and a last-known-good copy.

Merge by `id`; later sources override earlier ones. This lets users patch an image tag or health path without forking.

#### 3.1.3 Installer integration

- Serve `GET /apps.json` with the merged catalog.
- Add `GET /apps/{id}` before the catch-all image route.
  - Validate IDs with the catalog ID regex.
  - Emit `--install=<id>`.
- Keep `GET /{image...}` for raw image references.
- `--install` resolves in this order: catalog ID, built-in alias, raw image reference.
- TUI fetches `/apps.json` at startup and falls back to embedded apps when offline.

#### 3.1.4 Installer and TUI UX

- App list displays name, description, and category.
- Hostname placeholder comes from the manifest.
- A dynamic environment form appears after hostname when the manifest declares required or prompted env vars.
- Generated secrets are created at install time and stored with the app settings.
- Settings screens pre-populate catalog defaults and allow per-app overrides.
- The app's source (`builtin`, `user`, `url`) is visible in the UI.

#### 3.1.5 Persistence and backward compatibility

Extend `ApplicationSettings` with optional fields. Zero values must reproduce current behavior:

- `CatalogID string`
- `ServicePort int` (default 80)
- `HealthCheckPath string` (default `/up`)
- `HealthCheckPort int`
- `Volumes []VolumeMount`
- `Command []string`
- `TLSMode string` (`auto` or `off`)
- `Verify *bool` (nil means current behavior)

Old labels continue to unmarshal. Existing apps keep the Rails-style key generation and current mount targets until their settings are migrated or they are redeployed with a catalog ID.

#### 3.1.6 Multi-volume scope

Phase 1 supports **one named volume with multiple mount targets**. This covers the common Rails-style `/storage` plus `/rails/storage` case and preserves the current backup format.

Full multiple-volume support requires:

- Replacing `Application.Volume` with a list.
- Tar prefixes per volume in backup and restore.
- Restoring each volume into the correct container path.
- Removing all volumes on app deletion.

Treat this as a later, separately tested change. Do not half-implement it.

### 3.2 Deploy contract extensions

| Capability | Implementation |
| --- | --- |
| Custom container port | Pass `id:<port>` to `Proxy.Deploy`; add `--target` port handling in `deployArgs` |
| Custom health check | Add health path, port, and timeout to `DeployOptions`; use the same values in `verifyHTTP` |
| Multiple mount targets | Drive `volumeMounts` from settings, keeping the existing single-volume name |
| Catalog env | Extend `BuildEnv` and the install form; preserve existing env injection for legacy apps |
| Disable TLS | Add `TLSMode`; use it in `TLSEnabled` and pass `--tls` only when enabled |
| Optional verification | Add `Verify`; make verification failures remove the container and proxy entry only, never the volume |
| Custom command/user | Add `Command` and `User` to `container.Config` only when set |
| Non-web apps | Out of scope for the first catalog release; requires skipping proxy deployment and relaxing the hostname requirement |
| Multi-container apps | Out of scope; requires a services/stack model |

### 3.3 Remote targets

#### 3.3.1 Target configuration

```yaml
# ~/.config/oma/config.yaml
targets:
  this-mac:
    docker: unix:///var/run/docker.sock
  homelab:
    ssh: deploy@192.168.1.20
    dockerSocket: /var/run/docker.sock
    namespace: oma
  vps:
    ssh: root@203.0.113.10
    dockerSocket: /var/run/docker.sock
    namespace: oma
default: this-mac
```

CLI:

```sh
oma target add vps --ssh root@203.0.113.10
oma target list
oma --target vps deploy ghcr.io/foo/bar --host app.example.com
oma --target vps
```

Environment override: `OMA_TARGET`; `DOCKER_HOST` remains the fallback for the local target.

#### 3.3.2 Transport

Use SSH as the transport for the first implementation.

Recommended approach:

- Dial the remote Docker unix socket over SSH.
- Configure the Docker client with `client.WithHost("http://docker")` and `client.WithDialContext`.
- Authenticate with ssh-agent, identity files, and `known_hosts`.
- Alternatively shell out to OpenSSH with `ssh -L <local-socket>:<remote-socket>` to inherit the user's SSH config. This avoids a new crypto dependency and matches the approach used by Docker contexts.

Refactor `NewNamespace` and `RestoreNamespace` to accept an injected `*client.Client` via an option. All Docker API operations, including stats, logs, events, exec, and container copies, then work unchanged.

#### 3.3.3 Target-aware observability

| Feature | Work |
| --- | --- |
| Container stats | Already works through the Docker API |
| Logs and exec | Already works through the Docker API |
| Events | Already works through the Docker API |
| Host CPU/RAM/disk | Add a host-metrics interface: local gopsutil plus an SSH implementation reading `/proc` and `df`; or use Docker `/info` and `/system/df` and mark partial data |
| Proxy metrics | Inject an `http.Client` whose transport dials `127.0.0.1:<metricsPort>` through SSH |
| HTTP verification | Run from the controller by default; support `--no-verify` |
| Backups | Controller-driven for phase 1; decide remote ownership separately |
| Background automation | Add `--target` to `background run`; one service per target or a controller loop |

#### 3.3.4 Provisioning

For a VPS:

```sh
ssh root@vps 'command -v docker >/dev/null || curl -fsSL https://get.docker.com | sh'
ssh root@vps 'usermod -aG docker deploy'
# Firewall: 22/tcp, 80/tcp, 443/tcp
# DNS: app.example.com -> VPS
```

The controller needs SSH access and permission to read the Docker socket. The target does not need the Oma binary for the Docker-tunnel model.

For a home machine:

- Public DNS pointing to a LAN address, or
- mDNS `.local` with TLS disabled, or
- `/etc/hosts` entries for every client.

Automatic TLS requires public DNS and reachable ports 80 and 443. Add a per-app TLS mode for LAN deployments; `*.localhost` is currently the only hostname treated as local.

#### 3.3.5 Remote model decision

Preferred first model: Docker-over-SSH.

- Preserves all existing ONCE behavior.
- Keeps state entirely on the target daemon.
- Requires only Docker on the target.
- Cannot run unattended updates or backups while the controller is offline.

Alternative: remote agent with an HTTP or RPC API.

- Enables 24/7 updates and backups on a VPS.
- Requires installing and updating Oma on the target.
- Requires authentication, version negotiation, and a new data layer for the TUI.

Do not build the agent until the Docker-tunnel model is shipped and the automation gap is proven to matter.

### 3.4 Security hardening

- Verify release checksums in `install.sh` and the self-updater. Use a portable SHA-256 helper because macOS does not ship `sha256sum`.
- Prefer signed releases if Oma is distributed commercially.
- Make verification failures non-destructive.
- Treat catalogs as trusted configuration. Reject unsafe paths, validate all image refs, and never allow host bind mounts.
- Document that Docker access is root-equivalent, including over SSH.
- Decide whether secrets remain in Docker labels or move to Docker secrets. Keep the current behavior for compatibility until the migration path is clear.

### 3.5 Rebranding

Fork identity points:

| Item | Current location |
| --- | --- |
| Go module path | `go.mod` |
| Release repository | `installer/templates/install.sh:4`, `internal/version/updater.go:20` |
| Installer image | `installer/config/deploy.yml:2` |
| Install host | `installer/config/deploy.yml:12` |
| Proxy image | `internal/docker/proxy.go:23` |
| Label key and namespace default | `internal/docker/proxy.go:24`, `internal/docker/namespace.go:15` |
| launchd label prefix | `internal/service/launchd.go:15` |
| Log and state directories | `internal/logging/log.go:45`, `internal/userstats/store.go` |
| Built-in app list | `internal/ui/install_app_list.go:16` |
| Release artifacts | `Makefile`, `.github/workflows/release.yml` |

Current surface: 175 `basecamp` references across 80 files, mostly imports and test fixtures.

Proposed Oma identifiers:

- Module: `github.com/ikioma-ai/oma`
- Binary: `oma`
- Default namespace: `oma`
- Container prefix: `oma-`
- Label key: `oma`
- Background service: `oma-background`
- launchd label: `com.ikioma.oma`
- State dir: `~/.local/state/oma`
- Install host: to be confirmed
- Proxy image: pin an existing `basecamp/kamal-proxy` tag or publish an Oma mirror

Freeze runtime identifiers before the first real installation. Changing the label key, namespace, or volume prefix later orphans existing apps.

---

## 4. Fork execution plan

### Phase 0: Repository and baseline

1. [x] Move `basecamp/once` into `ikioma-ai/oma`, preserving history at `v0.3.4` (`864ec7c`).
2. [x] Set `origin` to `ikioma-ai/oma` and `upstream` to `basecamp/once`.
3. [x] Remove the nested `reference/once` checkout. The repository root is now the fork.
4. [x] Run `make test` / `go test ./internal/...`.
   - Passes with `TZ=UTC` and a short temp directory. Records the two upstream environment-sensitive tests in section 1.1.
5. [ ] Run `make integration` on a Docker host. Docker 29.4.0 is available locally.
6. [ ] Publish an Oma-controlled integration app image instead of `ghcr.io/basecamp/once-integration-app`.
7. [x] Record the baseline commit and test result in this document.

Exit criteria:

- Unit tests pass under the documented environment. Done.
- Integration tests pass. Pending.
- Release artifacts build for Linux and macOS, amd64 and arm64. Pending.

### Phase 1: Hard rename

Do this in one reviewable commit.

1. Rename the module path and all imports.
2. Rename the binary and command paths.
3. Replace repository, installer host, image namespaces, service labels, state directories, and user-visible product strings.
4. Centralize new branding in `internal/brand`.
5. Update license headers and add a modification notice while retaining the MIT notice.
6. Update GitHub workflows, Kamal config, installer image references, and release asset names.

Exit criteria:

- `go build ./...` succeeds.
- Tests pass.
- `oma version` reports the Oma version.
- `oma background install` installs an Oma-named service.
- `oma` on first run uses the Oma state directory.

### Phase 2: Catalog

1. Add `internal/catalog`.
2. Add the manifest schema and validation.
3. Replace `knownApps` with the merged catalog store.
4. Extend `ApplicationSettings` with backward-compatible fields.
5. Pass target port and health-check settings to `kamal-proxy`.
6. Drive mount targets from settings.
7. Add install-time environment forms.
8. Add installer `/apps.json` and `/apps/{id}`.
9. Add CLI commands: `oma catalog list`, `oma catalog show`, `oma catalog validate`, `oma catalog add`.
10. Update the app list and settings screens.

Exit criteria:

- An app on port 3000 with `/healthz` deploys successfully.
- An app with data at `/data` survives restart and backup/restore.
- A required env var blocks installation until supplied.
- A user catalog file overrides a built-in image tag.
- Existing image and alias installs still work.

### Phase 3: Remote targets

1. Add `internal/target` and config loading.
2. Add `oma target add`, `list`, and `remove`.
3. Add `--target` to root commands and the TUI.
4. Inject a Docker client into namespace construction.
5. Implement SSH socket dialing.
6. Add target-aware proxy metrics and host metrics.
7. Add `--no-verify` and non-destructive failure handling.
8. Add `--target` to `background run`.

Exit criteria:

- `oma --target vps list` shows apps from the remote daemon.
- Deploy, stop, start, logs, exec, backup, and restore work against a remote daemon.
- The dashboard shows remote container stats.
- Remote host metrics are either correct or explicitly marked unavailable, never silently local.
- A failed DNS verification leaves the app container or deployment intact and reports a clear error.

### Phase 4: Automation and hardening

1. Decide backup ownership for remote targets.
2. Add remote backup download or a remote runner path.
3. Add checksum verification to the installer and self-updater.
4. Add release signing if distributing binaries.
5. Add catalog signature or trust policy if remote catalogs are supported.
6. Add LAN TLS mode.
7. Add installer support for controller-only installs.

### Phase 5: Release

1. Publish Oma release artifacts.
2. Deploy the Oma installer service.
3. Test a clean install on Linux and macOS.
4. Test a clean VPS target.
5. Test a home/LAN target.
6. Document migration from ONCE if the product ever needs it. Oma starts clean, so no migration is planned for now.

### Upstream maintenance policy

- Merge or cherry-pick upstream release tags on a fixed cadence.
- Keep a list of modified upstream files in `UPSTREAM.md`.
- Prefer new packages and new files over edits to existing files.
- Do not reformat upstream code.
- Use `git rerere` for recurring conflicts.
- Upstream generic features such as catalog metadata and a target transport when practical; keep Oma defaults and branding local.
- Never let branding changes and feature changes share a commit.

---

## 5. Acceptance test matrix

| Area | Scenario | Expected result |
| --- | --- | --- |
| Baseline | Existing v0.3.4 deploy/list/stop/backup/restore on local Docker | Unchanged |
| Catalog | Built-in app resolves by ID and alias | Same image and defaults |
| Catalog | User override wins over built-in | Overridden image or health path is used |
| Catalog | Invalid manifest | Rejected with a field-specific error |
| Catalog | Installer `/apps/{id}` | Renders `--install=<id>` |
| Catalog | Installer `/ghcr.io/foo/bar` | Continues to work |
| Deploy | Custom port 3000 | Proxy routes to port 3000 |
| Deploy | Custom health path `/healthz` | Deployment waits on the correct path |
| Deploy | Required env var | Install blocks until provided |
| Deploy | Multiple mount targets | Same volume mounted at both paths |
| Backup | App data under custom path | Backup and restore preserve it |
| Remote | SSH target list | Shows remote apps only |
| Remote | SSH deploy | Container and proxy exist on the remote daemon |
| Remote | Remote logs and exec | Work through the tunnel |
| Remote | Remote metrics | Values come from the remote proxy |
| Remote | Remote host metrics | Correct data or explicit unavailability |
| Safety | DNS verification failure | No data volume is removed |
| Security | Corrupt installer checksum | Installation aborts |
| Security | Malicious catalog path or image | Rejected |

---

## 6. Out of scope for the first release

- Multi-tenant accounts, teams, or RBAC.
- Multi-host scheduling or Kubernetes.
- Compose-style multi-container applications.
- Non-HTTP applications.
- Non-Docker runtimes.
- Native Windows support. WSL2 is the documented path unless a separate decision changes it.
- A web dashboard. The TUI and CLI remain the interfaces.
- Changing the state-in-labels model.

---

## 7. Open decisions

1. Remote model: Docker-over-SSH only, or Docker-over-SSH plus a remote agent for 24/7 automation.
2. Backup ownership: controller disk, target disk, or object storage.
3. LAN TLS: per-app `tls: off`, private CA, or DNS-01.
4. Catalog trust: embedded only, trusted URL, or signed manifests.
5. Windows support: WSL2 only, or a native build and installer.
6. Proxy image: continue using `basecamp/kamal-proxy` or publish a pinned Oma image.
7. Branding: final install host, GitHub organization, and license/notice wording.

---

## Appendix A: File-level change map

| File or area | Change |
| --- | --- |
| `go.mod` | Module path |
| `cmd/once` | Binary path and name |
| `internal/brand` | New; product strings and identifiers |
| `internal/catalog` | New; manifest types, parsing, merge, validation |
| `internal/target` | New; target config and resolution |
| `internal/docker/connect.go` | New; local and SSH Docker clients |
| `internal/docker/namespace.go` | Accept an injected Docker client |
| `internal/docker/application.go` | Settings fields, port, health, env, mounts, verification |
| `internal/docker/proxy.go` | Target port and health flags |
| `internal/docker/application_backup.go` | Configurable primary mount; later multi-volume |
| `internal/ui/install_app_list.go` | Replace hardcoded apps with catalog |
| `internal/ui/install.go` | Catalog IDs, env form state |
| `internal/ui/install_activity.go` | Catalog-aware settings construction |
| `internal/ui/install_image_form.go` | Resolve catalog IDs |
| `internal/ui/app.go` | Target-aware scrapers and header |
| `internal/metrics/scraper.go` | Injectable HTTP client |
| `internal/system/scraper.go` | Host-metrics interface |
| `internal/background/runner.go` | Target-aware runs |
| `internal/version/updater.go` | Checksum verification |
| `internal/service/*` | Oma labels and names |
| `installer/main.go` | Catalog routes |
| `installer/templates/install.sh` | Repo, checksums, controller-only mode |
| `installer/config/deploy.yml` | Oma service and host |
| `integration/*` | Oma fixture image and target tests |
| `.github/workflows/*` | Oma release and registry wiring |
| `Makefile` | Artifact names |

## Appendix B: Rebrand audit command

Use a dry run first:

```sh
grep -RIn "basecamp\|get\.once\.com\|com\.basecamp\|once-" \
  --include='*.go' --include='*.sh' --include='*.yml' --include='*.yaml' \
  --include='*.toml' --include='Makefile' .
```

The current snapshot has 175 matches across 80 files. Most are import paths and test fixtures; the rest are runtime identifiers and user-visible strings. Review every match before replacing, especially:

- Docker label and namespace names.
- State file and directory names.
- Systemd and launchd service names.
- GitHub release URLs.
- Installer host and image references.
- Shell variable names that appear in generated scripts.
