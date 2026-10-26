# Release — Slices

Status: Phase 5 in progress; V1–V2, V4–V6 complete; V3 macOS pending
Shape: [`shaping.md`](shaping.md) · Spec: [`../../specs/release.md`](../../specs/release.md) · Roadmap: [`../oma-fork-plan.md`](../oma-fork-plan.md)

## Slice Grid

| V1: Release Artifacts | V2: Installer Host | V3: Clean Local Installs |
|---|---|---|
| ✅ COMPLETE | ✅ COMPLETE | ⏳ PENDING |
| • Tag-driven CI<br>• Four binaries + checksums<br>• Installer image in GHCR | • Final hostname<br>• DNS and TLS<br>• Kamal deploy + health | • Docker-free Linux<br>• macOS with Docker Desktop<br>• Verified install script |
| *Demo: tagged release and pullable installer image* | *Demo: HTTPS `/up` and install script* | *Demo: install on two clean hosts* |

| V4: Clean VPS Target | V5: Home/LAN + Controller | V6: Docs and Rollback |
|---|---|---|
| ✅ COMPLETE | ✅ COMPLETE | ✅ COMPLETE |
| • Target setup<br>• Deploy app<br>• Verify HTTPS and backup | • Home/LAN target<br>• Custom TLS<br>• Docker-free controller | • README install command<br>• Rollback runbook<br>• No-migration statement |
| *Demo: clean VPS hosts an app* | *Demo: LAN app and controller-only list* | *Demo: a reader installs from the docs* |

## V1: Release Artifacts

**Goal:** A stable or pre-release tag produces a complete, verifiable release and a pullable installer image.

### Acceptance criteria

- A stable `vX.Y.Z` tag runs CI and produces Linux/macOS amd64/arm64 binaries.
- The release publishes `checksums.txt` and every binary in the release is covered.
- The release verifies the generated checksums before publishing.
- The installer image is built for the supported deployment architectures and pushed to GHCR.
- The installer image has an immutable release-tag reference and a stable `latest` only for stable releases.
- Pre-release tags create GitHub pre-releases and do not publish `latest`.
- Release notes identify the installer image tag and binary checksums.

### Edge cases

- CI fails after the tag is pushed.
- A release asset is missing or duplicated in `checksums.txt`.
- A pre-release is accidentally published as a stable release.
- The installer image build succeeds but the registry push fails.
- A platform binary is omitted from the release.

### Demo

Push a test tag, confirm the GitHub release assets and checksums, pull the installer image by its release tag, and verify `latest` is unchanged for a pre-release tag.

### Status

COMPLETE

## V2: Installer Host

**Goal:** The installer service is deployed at its final HTTPS hostname and serves the current install script.

### Acceptance criteria

- The final installer hostname is chosen and documented; `get.oma.example` is removed from live configuration.
- DNS points at the installer servers and ports 80/443 are reachable.
- Kamal pulls the immutable GHCR installer image and starts the service.
- HTTPS terminates correctly and `GET /up` returns `200 OK`.
- `GET /`, `GET /apps.json`, and `GET /apps/{id}` serve the expected installer content.
- The committed deployment template contains no live server addresses or credentials.
- The deployment can be repeated from the documented command.

### Edge cases

- DNS is not propagated.
- Port 80 or 443 is occupied or blocked.
- The installer image tag is missing from GHCR.
- TLS certificate issuance fails.
- A previous installer deployment is still running.

### Demo

From a clean workstation, request `https://<installer-host>/up` and the install script; confirm the script renders the expected repository and catalog routes.

### Status

COMPLETE

## V3: Clean Local Installs

**Goal:** The hosted installer works on clean Linux and macOS machines.

### Acceptance criteria

- A Docker-free Linux host installs Oma, its background service, and the requested app.
- A macOS host with Docker Desktop installs Oma and the requested app.
- The installer verifies the release checksum before installing.
- The installed binary reports the release version.
- The background service is installed and starts.
- The installed Oma can reach the installer catalog and install an embedded app.
- Failure paths preserve the host: a corrupt download aborts without installing.

### Edge cases

- Docker is installed but not running on macOS.
- The user lacks passwordless sudo for the background service.
- The install directory is not writable.
- The checksum file is unavailable.
- The first-run TUI is unavailable because the host has no TTY.

### Demo

Run the public install command on one clean Linux VM and one clean macOS machine; deploy one app on each.

### Status

PARTIAL — clean Linux passed on a fresh Ubuntu 24.04 VM with `v0.4.1`: binary, checksum, systemd service, and a deployed Campfire app. Clean macOS install remains pending because this environment has no passwordless sudo or clean macOS host; the `darwin-arm64` release asset and checksum were verified.

## V4: Clean VPS Target

**Goal:** Oma can prepare and manage a fresh VPS target using the released binary.

### Acceptance criteria

- The VPS target has Docker and SSH access after `oma target setup`.
- A released Oma controller deploys an app to the VPS target.
- HTTPS works through the target proxy.
- The target app survives a controller disconnect and remains reachable.
- A backup is created and restored against the target.
- The target operation lock prevents concurrent deploy/backup work.
- Controller-side and target-side backup behavior is observable and documented.

### Edge cases

- SSH host key is unknown.
- Docker is missing or the Docker socket is not accessible.
- Ports 80/443 are blocked by the provider.
- The controller loses connectivity mid-deploy.
- The target disk fills during backup.

### Demo

From a clean controller install, prepare a fresh VPS, deploy an app, disconnect the controller, then reconnect and restore a backup.

### Status

COMPLETE — validated with released `v0.9.0` against a fresh UpCloud Ubuntu 26.04 VPS at `try.oma.ikioma.ai` (`80.47.237.85`). `target setup` installed Docker 29.9.2; the controller deployed Campfire; HTTP redirects to HTTPS with a Let's Encrypt certificate; the app stayed reachable after the controller commands exited; a controller-side backup was removed/restored successfully; the target agent wrote a target-side backup to `/backups` and the controller listed/downloaded it; a held `deploy` lock rejected a concurrent backup with `another operation is in progress`. Agent service active, no new public listener beyond SSH/80/443.

## V5: Home/LAN and Controller-Only

**Goal:** The release works for a home/LAN target and for a controller that has no local Docker.

### Acceptance criteria

- A Docker-free controller installs Oma with `OMA_LOCAL_DOCKER=false`.
- The controller-only install prints target setup instructions and does not launch the local TUI.
- A home/LAN target is prepared and an app is deployed.
- A trusted custom certificate is used for the LAN app.
- Invalid TLS material fails before proxy mutation.
- Local-target commands on the Docker-free controller fail with an actionable Docker error.
- Remote target commands work after the controller-only install.

### Edge cases

- No local Docker and no remote target configured.
- LAN DNS name does not resolve from the controller.
- Certificate is valid for the LAN name but not the public hostname.
- Docker is installed on the controller after the initial install.
- Target-local runner is absent for target-side backups.

### Demo

Install on a Docker-free controller, prepare a home/LAN target, deploy with a custom certificate, and open the app without a browser warning.

### Status

COMPLETE — validated on a fresh Ubuntu 24.04 VM with no Docker: controller-only install, local-Docker error, remote target list, home/LAN target setup, CA-signed custom-TLS HTTPS, and invalid-certificate failure before proxy mutation.

## V6: Documentation and Rollback

**Goal:** Users can install from the docs and maintainers can recover from a bad release.

### Acceptance criteria

- README shows the hosted install command and the source-build fallback.
- README documents controller-only installation.
- A release checklist covers tag, CI, artifacts, image, deploy, health, and clean-host checks.
- Rollback docs cover redeploying a previous installer image and forward-fixing a bad binary release.
- Docs state that ONCE migration is not supported and Oma starts clean.
- The installer hostname and support boundary are documented.
- No live server addresses or credentials are committed.

### Edge cases

- Installer image is healthy but the latest binary is bad.
- A bad binary release is already installed on a controller.
- DNS or TLS fails during an installer rollout.
- The README install command points at the old host.

### Demo

Follow the README from a clean machine; then redeploy the previous installer tag and confirm `/up` still works.

### Status

COMPLETE
