---
name: release
type: feature
priority: 2
slices: [release-artifacts, installer-host, clean-local-installs, clean-vps-target, home-lan-controller, docs-rollback]
---

{% section name="problem" %}

Oma is functionally complete but not yet distributable as a hosted product. A tag can produce binaries, but there is no published installer image, the installer service still points at a placeholder host, and no clean-host, VPS, or LAN acceptance run has proved the public install path. The README still instructs users to build from source.

The release also has no explicit rollback story. Installer users and the self-updater follow the latest stable release, so a bad artifact can reach every new install before the maintainer can react. The operator needs a repeatable release, a public HTTPS installer, a clean validation record, and clear boundaries for what the first release does not support.

{% /section %}

{% section name="desired-outcome" %}

A stable tag publishes four Oma binaries, a verified checksum manifest, and a pullable installer image. The installer is reachable at a chosen HTTPS hostname and serves the current install script and catalog routes. A clean Linux host, a clean macOS host, a Docker-free controller, a VPS target, and a home/LAN target can install and operate Oma using the published artifacts.

The release can be repeated from documented commands. A bad installer image can be rolled back to a previous immutable tag, while a bad binary release follows a documented forward-fix path. ONCE migration is explicitly unsupported; Oma starts clean.

{% /section %}

{% section name="context" %}

This contract covers the release pipeline, installer image publication, installer hosting, clean-environment acceptance, and release documentation. It depends on checksum verification, remote targets, the application catalog, custom TLS, and controller-only installation delivered in earlier work.

### Cross-references

- `specs/hardening.md` — release checksums, target backups, backup transfer, custom TLS, and controller-only installation are prerequisites for clean acceptance.
- `specs/remote-targets.md` — target configuration, SSH transport, setup, and operation locking used by VPS and LAN validation.
- `specs/app-catalog.md` — installer catalog routes and app installation behavior.
- `docs/release/shaping.md` — selected shape, alternatives, fit check, and breadboard.
- `docs/release/slices.md` — V1–V6 slices, demos, and acceptance criteria.
- `docs/oma-fork-plan.md` — Phase 5 scope, release decisions, and the no-migration boundary.

### Implementation reference

- `.github/workflows/release.yml` — tag-triggered release and artifact publication.
- `.github/workflows/ci.yml` — test, lint, build, and integration gates.
- `Makefile` — release builds and checksum generation.
- `installer/Dockerfile` — installer image build.
- `installer/main.go` — installer HTTP routes and catalog rendering.
- `installer/templates/install.sh` — hosted install script, checksum verification, and controller-only mode.
- `installer/config/deploy.yml` — deployment template; currently contains placeholders.
- `README.md` — public installation instructions.

### Key architectural decisions

- **GHCR plus Kamal** — reuse the existing Kamal deployment shape and a public container registry; no new installer runtime.
- **Immutable installer tags** — deploy a release-tagged image; `latest` is only a convenience for stable releases.
- **Stable and pre-release channels** — the installer and self-updater follow stable latest; pre-releases are for validation.
- **Installer hostname is an input** — the repository keeps a placeholder; the live host and servers are supplied at deployment time.
- **No ONCE migration** — there is no in-place state import in the first release.
- **Forward-fix binary releases** — the self-updater does not downgrade; installer images can be rolled back by redeploying a prior tag.

{% /section %}

{% section name="technical-direction" %}

### Release artifacts

- Stable release tags use `vMAJOR.MINOR.PATCH`; pre-release tags use the same form with a hyphenated suffix such as `-rc.1`. [AC#1,AC#2]
- A stable release publishes `oma-linux-amd64`, `oma-linux-arm64`, `oma-darwin-amd64`, `oma-darwin-arm64`, and `checksums.txt`. [AC#1]
- The release verifies every generated checksum before publishing any asset. [AC#1]
- The installer image is published to GHCR with an immutable release-tag reference. [AC#1]
- The installer image is built for `linux/amd64` and `linux/arm64`. [AC#1]
- A stable release publishes or updates the `latest` installer tag; a pre-release does not. [AC#2]
- A pre-release is marked as a GitHub pre-release and is not returned by the stable latest-release lookup used by the installer. [AC#2]
- Release notes identify the binary assets, checksum manifest, installer image tag, and validation status. [AC#10]

### Installer hosting

- The final installer hostname is configured before deployment; the placeholder host is not used on a live service. [AC#3]
- DNS points at the installer hosts and ports 80 and 443 are reachable from the public Internet. [AC#3]
- HTTPS terminates successfully for the installer hostname. [AC#3]
- `GET /up` returns HTTP 200. [AC#3]
- The install script, `/apps.json`, and `/apps/{id}` routes serve the expected content. [AC#3]
- The deployment uses the immutable installer image tag selected for the release. [AC#1,AC#3]
- Committed deployment files contain no live server addresses or credentials. [AC#9]
- Redeploying a previous immutable installer tag restores the prior installer service. [AC#8]

### Clean acceptance

- A clean Linux host starts without Oma and receives the binary, background service, and requested app through the public installer. [AC#4]
- A clean macOS host with Docker Desktop receives the binary, background service, and requested app through the public installer. [AC#4]
- The installer verifies the release checksum before installing the binary. [AC#4]
- A Docker-free controller installs with `OMA_LOCAL_DOCKER=false`, prints target setup instructions, and does not launch the local TUI. [AC#7]
- A clean VPS target is prepared through target setup and deploys an app through the released controller. [AC#5]
- A home/LAN target deploys an app with a trusted custom certificate. [AC#6]
- Invalid custom TLS material fails before proxy mutation. [AC#6]
- Local commands on a Docker-free controller fail with an actionable Docker error. [AC#7]
- Clean acceptance runs use the public installer hostname rather than a local installer process. [AC#3,AC#4]

### Rollback and documentation

- A documented release checklist covers tag, CI, artifacts, installer image, deployment, health, and clean-host checks. [AC#10]
- The README shows the hosted install command and the source-build fallback. [AC#10]
- The README documents controller-only installation. [AC#10]
- The docs describe installer-image rollback and the forward-fix path for a bad binary release. [AC#8]
- The docs state that ONCE migration is not supported and Oma starts clean. [AC#9]
- The docs identify the support boundary and the final installer hostname. [AC#3,AC#10]

{% /section %}

{% section name="domain-rules" %}

- **R1:** A stable release is immutable and complete; no asset is replaced after publication.
- **R2:** A pre-release never becomes the installer's stable latest release.
- **R3:** The installer serves the current install script and catalog routes over HTTPS.
- **R4:** The install script validates the binary checksum before installation.
- **R5:** The installer image is deployed by an immutable tag, never a floating branch tag.
- **R6:** Live server addresses and credentials are deployment inputs, not committed files.
- **R7:** A Docker-free controller uses only remote targets until local Docker becomes available.
- **R8:** A LAN certificate is validated before proxy mutation.
- **R9:** A failed clean acceptance run blocks the release promotion.
- **R10:** ONCE migration is not attempted or implied.
- **R11:** A bad binary release is fixed forward with a new patch release; automatic downgrade is not supported.
- **R12:** The release checklist is the authoritative promotion gate.

{% /section %}

{% section name="edge-cases-and-failure-modes" %}

- **Tag pushed but CI fails:** No release assets or installer image are published; the tag remains for inspection but is not promoted.
- **Checksum manifest missing or incomplete:** Release publication aborts before anyone can install from it.
- **Pre-release accidentally gets `latest`:** The release job must not publish or move `latest` for a pre-release tag.
- **Installer image push fails:** The release is incomplete; binaries are not promoted as a working hosted release until the image exists.
- **DNS not propagated:** The deployment health check fails and the installer is not announced.
- **Ports 80/443 blocked:** TLS issuance or HTTPS health checks fail; the firewall must be corrected before promotion.
- **Installer image tag missing:** Deployment fails before replacing the running installer.
- **Bad installer image deployed:** Redeploy the previous immutable tag and verify `/up`; users are unaffected by the binary channel.
- **Bad binary release already installed:** Publish a fixed patch release; the self-updater moves forward, not backward.
- **Docker installed but not running on macOS:** The installer fails with Docker Desktop guidance before installing Oma.
- **No TTY on a clean host:** Installation completes but the first-run TUI does not launch; the binary and service remain installed.
- **Docker-free controller with no target:** Local commands fail with the Docker error; target setup instructions remain available.
- **LAN certificate wrong for the hostname:** Deployment fails before the proxy changes.
- **Clean test machine has a previous Oma install:** The run is not a clean acceptance result and must be repeated on a fresh host.
- **Live deploy file committed:** The release check rejects it and requires the placeholder template to remain in the repository.

{% /section %}

{% section name="examples" %}

### Example 1: Stable release

- **Setup:** CI is green and a release operator is ready to publish version `v1.0.0`.
- **Action:** The operator pushes the stable tag and follows the release checklist.
- **Result:** GitHub publishes four binaries and `checksums.txt`; GHCR publishes `oma-installer:v1.0.0` and `latest`; the installer deploys from the immutable tag; `/up` returns 200; the clean-host checklist passes.

### Example 2: Pre-release validation

- **Setup:** The next version is `v1.1.0-rc.1`.
- **Action:** The operator pushes the pre-release tag.
- **Result:** The release is marked pre-release, binaries and a tagged installer image are available, `latest` is unchanged, and the installer still serves `v1.0.0` as stable.

### Example 3: Clean controller and VPS

- **Setup:** A fresh controller has no Docker; a fresh VPS is reachable over SSH.
- **Action:** Install with `OMA_LOCAL_DOCKER=false`, run `oma target add` and `oma target setup`, then deploy an app to the VPS.
- **Result:** The controller runs Oma without local Docker, the VPS gets Docker and an app, HTTPS works, and `oma --target vps list` shows the app.

### Example 4: LAN certificate failure

- **Setup:** A home/LAN target is reachable, but the supplied certificate is valid for a different hostname.
- **Action:** Deploy the app with custom TLS.
- **Result:** The deployment fails before proxy mutation, the previous app configuration remains active, and the error names the hostname mismatch.

### Example 5: Installer rollback

- **Setup:** `v1.0.1` of the installer image is unhealthy after deployment.
- **Action:** Redeploy `oma-installer:v1.0.0` using the documented command.
- **Result:** The previous installer image serves `/up` and the install routes again; the binary release channel is unchanged.

{% /section %}

{% section name="boundaries" %}

- **In scope:** release artifacts, checksums, installer image publication, installer HTTPS deployment, clean Linux/macOS/controller/VPS/LAN acceptance, release documentation, installer rollback, and the no-migration statement.
- **Out of scope:** release signing, installer artifact signing, multi-region or fleet deployment, automatic binary downgrade, ONCE state migration, and a web dashboard.
- **Do not modify:** existing binary names, checksum file name, installer route contracts, target transport semantics, or the no-migration boundary.

{% /section %}

{% section name="acceptance-criteria" %}

- [ ] AC#1: Given a stable release tag, when the release workflow completes, then four Oma binaries and `checksums.txt` are published, the checksums verify, and an immutable installer image is available from GHCR for `linux/amd64` and `linux/arm64`.
- [ ] AC#2: Given a pre-release tag, when the release workflow completes, then the release is marked pre-release and the stable installer `latest` tag and stable latest-release lookup remain unchanged.
- [ ] AC#3 (integration): Given the final installer hostname is configured, when the installer is deployed, then HTTPS `/up` returns HTTP 200 and `GET /`, `GET /apps.json`, and `GET /apps/{id}` serve the expected content from the release-tagged image.
- [ ] AC#4 (integration): Given clean Linux and macOS hosts, when the public install command is run, then Oma and its background service are installed after checksum verification, and an embedded catalog app can be installed.
- [ ] AC#5 (integration): Given a clean controller and a fresh VPS target, when target setup and deploy run, then the target app is reachable over HTTPS, survives controller disconnect, and produces a restorable backup.
- [ ] AC#6 (integration): Given a home/LAN target and valid custom TLS material, when an app is deployed, then it serves trusted HTTPS; given invalid material, the deployment fails before proxy mutation.
- [ ] AC#7 (integration): Given a Docker-free controller, when the installer runs with `OMA_LOCAL_DOCKER=false`, then the binary and service are installed without launching the local TUI, and `oma --target <name> list` works while `oma list` fails with an actionable Docker error.
- [ ] AC#8: Given a bad installer image, when the previous immutable image tag is redeployed, then `/up` and the install routes recover; the documentation states that bad binary releases are forward-fixed.
- [ ] AC#9: Given the committed repository and deployment template, when the release check runs, then no live server addresses or credentials are present, and ONCE migration is explicitly documented as unsupported.
- [ ] AC#10: Given a reader following the release documentation, when they use the documented install command and release checklist, then the command targets the final installer hostname and the checklist covers tag, CI, artifacts, image, deployment, health, and clean-host validation.

{% /section %}

{% slice name="release-artifacts" %}

Deliver stable/pre-release artifact publication, checksum verification, and the installer image in GHCR.

**Acceptance Criteria:**
- A stable tag produces four platform binaries and a complete `checksums.txt`.
- Generated checksums are verified before release assets are published.
- The installer image is built for `linux/amd64` and `linux/arm64`.
- The installer image is pushed to GHCR with an immutable release tag.
- Stable releases publish `latest`; pre-releases do not.
- Pre-release tags create GitHub pre-releases and remain outside stable latest lookup.
- Release notes list the assets, checksums, image tag, and validation status.

{% /slice %}

{% slice name="installer-host" %}

Deliver a publicly reachable HTTPS installer at the final hostname.

**Acceptance Criteria:**
- The final installer hostname is chosen and no live service uses the placeholder host.
- DNS resolves and ports 80/443 are reachable.
- The deployment pulls the immutable GHCR installer image.
- HTTPS `/up` returns HTTP 200.
- `/`, `/apps.json`, and `/apps/{id}` serve the expected content.
- The committed deployment template contains no live server addresses or credentials.
- Redeploying a previous image tag restores the prior installer service.

{% /slice %}

{% slice name="clean-local-installs" %}

Deliver clean Linux and macOS installation validation through the public host.

**Acceptance Criteria:**
- A clean Linux host installs Oma and the background service.
- A clean macOS host with Docker Desktop installs Oma and the background service.
- The installed binary verifies the release checksum before installation.
- The installed binary reports the release version.
- An embedded catalog app installs through the installed Oma.
- A corrupt or missing checksum aborts without installing the binary.

{% /slice %}

{% slice name="clean-vps-target" %}

Deliver a clean VPS target validation.

**Acceptance Criteria:**
- A fresh VPS target completes target setup with Docker and SSH access.
- The released controller deploys an app to the target.
- The app is reachable over HTTPS through the target proxy.
- The app remains reachable after the controller disconnects.
- A backup is created and restored against the target.
- The operation lock prevents overlapping deploy and backup work.

{% /slice %}

{% slice name="home-lan-controller" %}

Deliver home/LAN target and Docker-free controller validation.

**Acceptance Criteria:**
- A Docker-free controller installs with `OMA_LOCAL_DOCKER=false`.
- The controller-only install prints target setup instructions and skips the local TUI.
- A home/LAN target deploys an app with a trusted custom certificate.
- Invalid custom certificate material fails before proxy mutation.
- Local commands fail with an actionable Docker error on the Docker-free controller.
- Remote target commands succeed after the controller-only install.

{% /slice %}

{% slice name="docs-rollback" %}

Deliver the install, release, rollback, and no-migration documentation.

**Acceptance Criteria:**
- README shows the hosted install command and the source-build fallback.
- README documents controller-only installation.
- The release checklist covers tag, CI, artifacts, image, deployment, health, and clean-host validation.
- The rollback runbook covers installer image rollback and binary forward-fix.
- ONCE migration is documented as unsupported and Oma as starting clean.
- The support boundary and final installer hostname are documented.

{% /slice %}
