---
name: hardening
type: feature
priority: 2
slices: [release-checksums, target-backup-destination, target-backup-transfer, lan-tls, controller-only-installer]
---

{% section name="problem" %}

Oma can install and manage applications locally and on remote targets, but the surrounding operational surface is still trusting and controller-centric. Installer and self-update downloads are executed or installed without an integrity check, so a corrupted or substituted download can replace a working binary. Remote backups default to controller-side storage, so a sleeping or failed controller silently stops protecting a remote target. LAN deployments can disable TLS, but there is no way to present a trusted certificate without public ACME. The installer also requires a local Docker daemon, which blocks a user whose only job is to manage a remote target.

These gaps are independent, but they share a failure mode: the safe path is implicit and the unsafe path is a default. An operator should be able to verify downloads, choose backup ownership, terminate TLS with their own certificate, and install a controller without local Docker, while existing local and remote behavior stays unchanged unless a new mode is selected.

{% /section %}

{% section name="desired-outcome" %}

Every downloaded Oma binary is checked against the release checksum before it is installed or used to replace a running binary. A remote application can store scheduled backups on its target while the controller is offline, and the controller can later list, download, and restore those archives. A LAN application can serve HTTPS with an operator-provided certificate and key, with invalid material rejected before proxy state changes. An operator can install Oma on a Docker-free controller, configure a remote target, and manage it through the normal command surface.

The default remains controller-side backups for existing applications, `auto` or `off` TLS, and the existing installer behavior. Release signing, catalog signatures, self-signed certificate generation, ACME DNS-01, object storage, and automated target-runner installation remain outside this contract.

{% /section %}

{% section name="context" %}

This contract extends the release path, the backup and restore path, application settings, proxy deployment, and the installer. It depends on the remote-target contract for target selection and transport, and on the catalog/application contract for settings persistence and apply behavior.

### Cross-references

- `specs/remote-targets.md` — target selection, SSH transport, operation lock, remote background runner, and controller-side backup defaults. This contract adds target-side backup ownership and keeps the existing transport unchanged.
- `specs/app-catalog.md` — application settings persistence and apply behavior. This contract extends settings with backup destination and custom TLS while preserving existing fields.
- `docs/hardening/shaping.md` — selected workstreams, alternatives, fit checks, and breadboard.
- `docs/hardening/slices.md` — V1–V5 implementation slices, edge cases, and demos.
- `docs/oma-fork-plan.md` — Phase 4 scope and deferred work.

### Implementation reference

- `installer/templates/install.sh` — release asset download, checksum, Docker check, install, and first-run behavior.
- `internal/version/updater.go` — release asset lookup, download, and binary replacement.
- `internal/command/self_update.go` — self-update command entry point.
- `internal/docker/application_settings.go` — persisted app settings and validation.
- `internal/docker/application_backup.go` — backup creation, retention, and restore.
- `internal/background/runner.go` — scheduled update and backup runner.
- `internal/docker/proxy.go` — proxy creation and deployment TLS flags.
- `internal/command/backup.go` and `internal/command/restore.go` — existing manual backup and restore commands.
- `internal/ui/settings_form_backups.go` — backup settings surface.
- `internal/ui/settings_form_*` — application settings surfaces for TLS.

### Key architectural decisions

- **Checksums first, signatures later** — SHA-256 verification closes the corrupt-download gap without release key management.
- **Controller-side backup remains the default** — existing installs do not silently change ownership.
- **Target-side backup is opt-in and runner-based** — the target-local runner is installed manually in this phase; automated installation is deferred.
- **Custom certificates, not self-signed** — the operator supplies trusted material; Oma validates and copies it.
- **Controller-only install is an environment flag** — no second binary or separate distribution channel.

{% /section %}

{% section name="technical-direction" %}

### Release integrity

- A release publishes `checksums.txt` with one entry per binary asset. Each entry is a 64-character lowercase SHA-256 digest, whitespace, and the asset basename. [AC#1,AC#2]
- The installer fetches `checksums.txt` from the same release as the selected asset and verifies the asset before installing or replacing anything. [AC#1,AC#2]
- The self-updater fetches `checksums.txt` from the release being installed, verifies the downloaded binary, and only then replaces the current binary. [AC#1,AC#2]
- A missing checksum file, missing entry, duplicate entry, malformed digest, or digest mismatch aborts the operation before installation or replacement. [AC#2]
- A checksum verification failure leaves the existing binary and installed background service intact. [AC#2]
- Checksums are fetched over HTTPS from the release channel. Do not accept a plaintext fallback or a checksum from a different release. (guidance)

### Backup ownership and transfer

- Application settings expose `backup.destination` with values `controller` and `target`. The default is `controller` for new and existing applications. [AC#3,AC#9]
- `backup.path` is an absolute path on the machine that performs the backup: the controller for `controller` and the target for `target`. [AC#3,AC#4]
- Scheduled backups with destination `controller` are written by the controller runner. Scheduled backups with destination `target` are written by the target-local runner. [AC#3,AC#4]
- When an application selects target-side backup but no target runner is available, the controller must not silently write a controller-side archive. It reports that the target runner is required, and the UI or CLI marks the destination as requiring a runner. [AC#3,AC#4]
- Target-side backup writes acquire the target operation lock and release it on success, failure, or cancellation. [AC#4]
- A target-side backup archive is either complete and discoverable or absent. Partial archives are not left with the final archive name. [AC#4,AC#5]
- Backup archives never overwrite an existing archive. [AC#4]
- `oma backup list [--target <name>]` lists target-side archives without mutating the target. [AC#5]
- `oma backup download <archive> <destination> [--target <name>]` writes a complete archive to the controller destination; a failed transfer leaves no final destination file. [AC#5]
- Downloaded archives restore through the existing `oma restore <file>` path. [AC#5]
- The existing `oma backup <host> <filename>` command continues to write the explicitly supplied controller path and does not silently switch to target-side storage. [AC#3,AC#9]
- Target-side archive names are validated as basenames and may not traverse outside `backup.path`. [AC#5]
- Manual target-runner installation is documented; automated binary/service installation on the target is outside this contract. (guidance)

### LAN TLS

- Application settings expose `tlsMode` with values `auto`, `off`, and `custom`. `auto` and `off` retain their existing behavior. [AC#6]
- `custom` requires `tlsCertificatePath` and `tlsPrivateKeyPath`, read from the controller filesystem at deploy or apply time. [AC#6]
- Certificate and key are validated before any proxy mutation: readable, PEM-decodable, matching key pair, currently valid, and covering the application host. [AC#6]
- Custom certificate material is copied into the proxy volume and referenced by the proxy TLS flags. Certificate contents are not stored in application settings labels. [AC#6,AC#8]
- `custom` enables TLS for the application host even when the host is private or localhost-shaped; an explicit `off` mode still wins. [AC#6]
- Apply-diff includes TLS mode and certificate changes. [AC#6]
- Missing, unreadable, mismatched, expired, not-yet-valid, or host-mismatched material fails before the proxy is changed and leaves the previous TLS configuration active. [AC#6]
- Private key contents never appear in logs, error messages, or persisted labels. [AC#8]

### Controller-only installer

- `OMA_LOCAL_DOCKER=false` makes the installer skip local Docker installation, Docker daemon checks, and the local first-run TUI launch. [AC#7]
- Controller-only installation still installs the Oma binary and background service, then prints target setup instructions. [AC#7]
- Without the flag, installer behavior is unchanged and local Docker remains required. [AC#8]
- Local-target commands against a controller-only install fail with an actionable Docker error and do not mutate target configuration. [AC#7]
- The installer does not configure or mutate a remote target on its own. [AC#7]

### Cross-cutting

- New modes do not silently change existing defaults: controller-side backup, `auto`/`off` TLS, and normal installer behavior remain the default. [AC#8,AC#9]
- Backup, TLS, and installer diagnostics never include secret values, private key contents, or registry credentials. [AC#8]
- Persisted settings without `backup.destination`, `tlsMode: custom`, or certificate paths remain readable and retain their behavior. [AC#9]
- Release signing, remote catalog signatures, self-signed certificate generation, ACME DNS-01, object storage, automated target-runner installation, and a controller-only setup wizard are out of scope. (guidance)

{% /section %}

{% section name="domain-rules" %}

- **R1:** A checksum failure is closed: no install, no replacement, no service change.
- **R2:** Controller-side backup is the default; an existing application must not be migrated silently.
- **R3:** A target-destination backup is written by the target runner, never silently written to the controller instead.
- **R4:** Target-side backup writes use the shared target operation lock.
- **R5:** A target-side backup archive is complete or absent; partial archives never use the final archive name.
- **R6:** Existing backup archives are never overwritten.
- **R7:** Custom TLS material is validated before any proxy mutation.
- **R8:** Certificate and private-key contents are not stored in application settings or emitted in logs.
- **R9:** `OMA_LOCAL_DOCKER=false` means no local Docker requirement for installation.
- **R10:** Existing settings load without the new fields and keep their previous behavior.
- **R11:** The target-local runner is installed manually in this phase; the controller does not install a remote Oma binary.
- **R12:** `oma backup list` is read-only on the target; `oma backup download` mutates only its controller destination.
- **R13:** Checksums come from the same release as the binary and are fetched over HTTPS.
- **R14:** Release signing, catalog signatures, self-signed certificates, ACME DNS-01, and object storage are outside this contract.

{% /section %}

{% section name="edge-cases-and-failure-modes" %}

- **Checksum file missing:** Install and self-update abort with an actionable error; the existing binary remains.
- **Checksum file malformed or duplicated:** The operation aborts rather than choosing one entry arbitrarily.
- **Checksum verifier unavailable:** The installer aborts because it cannot prove integrity; it does not fall back to an unverified install.
- **Download interrupted after verification but before replacement:** The temporary file is removed and the current binary remains in place.
- **Self-update directory is read-only:** The update aborts before touching the current binary.
- **Target runner missing:** The controller reports that target-side backup requires a target runner and does not write a controller-side archive.
- **Target storage full:** The partial target archive is removed; the previous archive remains intact.
- **Operation lock held:** The backup waits or fails with the lock owner; it does not bypass the lock.
- **Target offline between listing and download:** The download fails and no final destination file is presented as complete.
- **Destination switched with existing archives:** Archives on the previous side remain; no automatic migration deletes or moves them.
- **Certificate chain incomplete:** Validation fails before proxy mutation.
- **Key does not match certificate:** Validation fails before proxy mutation.
- **Certificate expired or not yet valid:** Validation fails before proxy mutation.
- **Certificate covers the wrong hostname:** Validation fails before proxy mutation.
- **Certificate files disappear after settings are saved:** The next deploy or apply fails before changing the proxy.
- **Proxy recreated after custom TLS:** Certificate material is restored from the configured source before the proxy serves traffic.
- **Controller-only local command:** The command exits non-zero with a Docker-specific message and does not write target configuration.
- **Local Docker installed later:** Local commands start working without reinstalling Oma or changing the installed binary.
- **Secret-bearing error path:** Backup, TLS, and installer errors redact private keys, passwords, and tokens.

{% /section %}

{% section name="examples" %}

### Example 1: Corrupt release binary

- **Setup:** A release publishes a binary and a matching `checksums.txt`.
- **Action:** The installer downloads the binary, but the downloaded bytes do not match the checksum.
- **Result:** The installer exits non-zero, does not install the binary or change the service, and reports the checksum mismatch. A self-update under the same condition leaves the running binary unchanged.

### Example 2: Target-side backup while the controller is offline

- **Setup:** An application selects `backup.destination: target` and a target-local runner is installed and running. The controller is stopped.
- **Action:** The scheduled backup becomes due on the target.
- **Result:** The target runner acquires the operation lock and writes a complete archive under the target backup path. The controller does not create a controller-side archive. When the controller returns, `oma backup list --target <name>` shows the archive.

### Example 3: Download and restore a target-side archive

- **Setup:** A target-side archive exists and the controller can reach the target.
- **Action:** The operator runs `oma backup download <archive> <destination> --target <name>`, then `oma restore <destination>`.
- **Result:** The download writes a complete controller-side file, and restore restores the application from that file. An interrupted download leaves no final destination file.

### Example 4: Trusted LAN certificate

- **Setup:** An application targets a LAN hostname. The operator has a certificate and key valid for that hostname and selects `tlsMode: custom`.
- **Action:** The application is deployed.
- **Result:** The proxy serves HTTPS with the supplied certificate and the browser trusts it. Reapplying with an expired certificate fails before the proxy changes, leaving the previous certificate active.

### Example 5: Controller-only install

- **Setup:** A host has SSH and no Docker. A remote target is already reachable.
- **Action:** The operator runs the installer with `OMA_LOCAL_DOCKER=false`, then runs `oma --target <name> list`.
- **Result:** Oma and its background service are installed, the installer does not launch the local TUI or require Docker, and the remote list works. Running `oma list` without a remote target fails with a Docker-specific error and does not change target configuration.

{% /section %}

{% section name="boundaries" %}

- **In scope:** release checksum generation and verification in the installer and self-updater; controller and target backup destinations; target-side backup listing, download, and restore; custom certificate TLS mode; controller-only installer mode; compatibility with existing settings and defaults.
- **Out of scope:** release signing; remote catalog signatures or trust policy; self-signed certificate generation; ACME DNS-01; object-storage backups; automated target-runner installation; a controller-only setup wizard; Windows support; remote agent transport.
- **Do not modify:** target selection and transport semantics, the backup archive format, app data volumes, release asset names, or the existing manual backup semantics without an explicit compatibility path.

{% /section %}

{% section name="acceptance-criteria" %}

- [ ] AC#1: Given a release that publishes a matching `checksums.txt` entry for the selected platform asset, when the installer or `oma self-update` runs, then it verifies the downloaded bytes and completes the installation or replacement.
- [ ] AC#2: Given a missing checksum file, missing or duplicate entry, malformed digest, or digest mismatch, when the installer or `oma self-update` runs, then it exits non-zero, does not install or replace the binary, and leaves the existing binary and background service intact.
- [ ] AC#3: Given an existing application whose settings omit a backup destination, when its settings are loaded and a scheduled backup runs, then the destination is controller-side and the archive is written under the configured controller path; given `backup.destination: target`, the controller does not write a controller-side archive.
- [ ] AC#4: Given an application with `backup.destination: target` and a running target-local runner, when a scheduled backup is due while the controller is offline, then the target runner creates one complete archive under the target backup path, holds the shared operation lock, and never overwrites an existing archive.
- [ ] AC#5 (integration): Given a target-side archive, when the operator runs `oma backup list --target <name>`, `oma backup download <archive> <destination> --target <name>`, and `oma restore <destination>`, then the archive is listed, the download produces a complete controller-side file, and restore succeeds; an interrupted download leaves no final destination file.
- [ ] AC#6: Given valid custom certificate material covering the application host, when the application is deployed or applied, then the proxy serves trusted HTTPS with that certificate; given missing, unreadable, mismatched, expired, or host-mismatched material, then the operation fails before any proxy mutation and the previous TLS configuration remains active.
- [ ] AC#7 (integration): Given a host without Docker, when the installer runs with `OMA_LOCAL_DOCKER=false`, then it installs the Oma binary and background service, skips Docker checks and the local first-run TUI, prints target setup instructions, and `oma --target <name> list` works; running `oma list` without Docker fails with an actionable Docker error and does not mutate target configuration.
- [ ] AC#8: Given the normal installer mode, controller-side backup default, or `auto`/`off` TLS mode, when the new features are present, then existing behavior is unchanged; backup, TLS, and installer diagnostics never emit private key contents, passwords, tokens, or registry credentials.
- [ ] AC#9: Given persisted settings from a previous release that omit backup destination, custom TLS fields, or certificate paths, when settings are loaded and saved, then existing fields and behavior are preserved and no automatic migration changes the backup destination or TLS mode.

{% /section %}

{% slice name="release-checksums" %}

Deliver checksum generation and verification for the installer and self-updater. The release artifact set includes the checksum manifest, and both download paths fail closed before changing an installed binary.

**Acceptance Criteria:**
- The release publishes `checksums.txt` covering every platform binary.
- The installer verifies the selected binary against the same release's checksum before installing it or changing the service.
- `oma self-update` verifies the downloaded binary against the same release's checksum before replacing the current binary.
- Missing, malformed, duplicate, or mismatched checksum data aborts without replacing the existing binary.
- Verification failure leaves the existing binary and background service intact.
- Installer and updater tests cover matching, mismatched, missing, and malformed checksum cases.

{% /slice %}

{% slice name="target-backup-destination" %}

Deliver explicit controller or target backup ownership, defaulting to controller, and a target-local runner path for target-side scheduled backups.

**Acceptance Criteria:**
- Application settings expose `backup.destination` with `controller` and `target`, defaulting to `controller`.
- Existing applications without the field continue to write controller-side backups.
- Target-side scheduled backups are written by the target-local runner under the target `backup.path`.
- The controller runner does not write a controller-side archive for a target-destination application and reports when a target runner is required.
- Target-side writes acquire and release the shared operation lock and never overwrite an existing archive.
- The UI or CLI shows which destination is active and warns when target-side backup requires a target runner.
- Tests cover the default, target-side write, controller skip, missing runner, lock contention, and overwrite refusal.

{% /slice %}

{% slice name="target-backup-transfer" %}

Deliver controller-side listing, download, and restore of target-side archives.

**Acceptance Criteria:**
- `oma backup list [--target <name>]` lists target-side archives without mutating the target.
- `oma backup download <archive> <destination> [--target <name>]` writes a complete controller-side file.
- Downloaded archives restore through `oma restore`.
- Interrupted downloads leave no final destination file.
- Archive names are validated as basenames and cannot traverse outside the backup path.
- Target-offline and partial-transfer failures produce actionable errors.
- CLI and integration tests cover list, download, restore, traversal rejection, and partial-transfer cleanup.

{% /slice %}

{% slice name="lan-tls" %}

Deliver a custom certificate TLS mode with validation, proxy deployment, and apply behavior.

**Acceptance Criteria:**
- `tlsMode` accepts `auto`, `off`, and `custom`; existing modes retain their behavior.
- `custom` requires `tlsCertificatePath` and `tlsPrivateKeyPath`.
- Certificate material is validated for readability, PEM decoding, key match, validity period, and host coverage before proxy mutation.
- Custom material is copied into the proxy volume; certificate and key contents are not stored in settings labels.
- The proxy serves HTTPS with the custom certificate for private or localhost-shaped hosts.
- Apply-diff includes TLS mode and certificate changes.
- Invalid or missing material fails without changing the active proxy configuration.
- Tests cover valid deployment, each validation failure, apply-diff, and secret redaction.

{% /slice %}

{% slice name="controller-only-installer" %}

Deliver a controller-only installation path for hosts without local Docker.

**Acceptance Criteria:**
- `OMA_LOCAL_DOCKER=false` skips Docker installation, Docker daemon checks, and the local first-run TUI launch.
- Controller-only installation still installs the Oma binary and background service and prints target setup instructions.
- The installer does not configure or mutate a target on the operator's behalf.
- Without the flag, installer behavior remains unchanged.
- Local-target commands on a controller-only host fail with an actionable Docker error and do not mutate target configuration.
- Remote target commands work after the controller-only install.
- Tests cover the flag, skipped Docker path, local error, remote list, and unchanged default path.

{% /slice %}
