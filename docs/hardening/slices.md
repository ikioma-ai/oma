# Hardening — Slices

Status: Phase 4 in progress; V1–V4 complete, V5 next
Shape: [`shaping.md`](shaping.md) · Spec: [`../../specs/hardening.md`](../../specs/hardening.md) · Roadmap: [`../oma-fork-plan.md`](../oma-fork-plan.md)

## Slice Grid

| V1: Release Checksums | V2: Target-Side Backups | V3: Backup Transfer | V4: LAN TLS | V5: Controller-Only Installer |
|---|---|---|---|---|
| ✅ COMPLETE | ✅ COMPLETE | ✅ COMPLETE | ✅ COMPLETE | ⏳ PENDING |
| • Installer verifies SHA-256<br>• Self-updater verifies SHA-256<br>• Fail closed | • Explicit backup destination<br>• Target runner path<br>• Target-side writes | • List target backups<br>• Download archive<br>• Restore download | • TLS `custom` mode<br>• Certificate/key validation<br>• Proxy TLS flags | • Skip local Docker<br>• Install binary/service<br>• Clear local errors |
| *Demo: corrupt download is rejected* | *Demo: target writes a backup while controller sleeps* | *Demo: download and restore a target backup* | *Demo: LAN app with trusted certificate* | *Demo: install controller without Docker* |

## V1: Release Checksums

**Goal:** Every downloaded Oma binary is verified against the release’s published SHA-256 before installation or replacement.

### Acceptance criteria

- The release publishes a checksum file covering all platform binaries.
- The installer downloads the checksum file and verifies the selected asset before executing or replacing anything.
- The self-updater downloads the checksum file from the same release and verifies the new binary before replacement.
- A missing checksum entry or a mismatch fails closed; the existing binary and service remain intact.
- The checksum file is fetched over HTTPS from the release channel; no unsigned plaintext fallback is accepted.
- Existing installation behavior remains unchanged when verification succeeds.

### Edge cases

- Checksum file missing or malformed.
- Binary asset missing from the checksum file.
- Multiple assets with the same name.
- Download interrupted after checksum verification but before replacement.
- Self-update on a read-only install directory.

### Out of scope

- Release signatures.
- Sigstore/cosign identity verification.

### Demo

Corrupt a downloaded binary and confirm the installer aborts; corrupt a self-update download and confirm the current binary remains.

### Status

COMPLETE

## V2: Target-Side Backups

**Goal:** Operators can choose controller-side or target-side backup storage, with target-side backups surviving controller downtime.

### Acceptance criteria

- Backup destination is explicit in app settings: `controller` or `target`.
- Controller-side remains the default for existing installs.
- Target-side requires an Oma runner on the target; manual installation is documented.
- The target runner writes backups to a target-local path under the background service.
- Target-side backup writes use the shared operation lock.
- Failure to write a target-side backup leaves any previous archive intact and reports a clear error.
- No secret values are written to logs.

### Edge cases

- Target runner not installed or not running.
- Target storage full.
- Operation lock held by deploy or update.
- Backup destination changed with an existing archive on the other side.
- Rootless Docker on the target.

### Out of scope

- Automated target runner installation.
- Object storage.
- Fleet scheduling.

### Demo

Stop the controller after starting a target-side backup; confirm the target runner completes the archive and the controller can later discover it.

### Target runner setup (manual)

1. Install Oma on the target host.
2. Reinstall its background service as the target backup role if the installer already created the default controller-role service:
   ```sh
   sudo oma background uninstall
   sudo oma background install --backup-role target
   ```
3. Set the application's backup destination to `target` and its backup location to an absolute path writable on the target.
4. Keep the controller's background service on the default `controller` role; it skips target-side backups and reports that the target runner is required.

### Status

COMPLETE

## V3: Backup Transfer

**Goal:** A controller can list and download target-side backups and restore them.

### Acceptance criteria

- `oma backup list --target <name>` lists target-side archives without mutating the target.
- `oma backup download --target <name> <archive>` streams the archive to a controller path.
- A downloaded archive restores through the existing restore path.
- Download failure leaves no partial archive presented as complete.
- Paths and permissions are explicit; no archive is silently overwritten.
- The target connection is reused or cleanly closed.

### Edge cases

- Target offline.
- Archive disappears between listing and download.
- Partial transfer.
- Insufficient controller disk space.
- Archive name collisions.

### Demo

List a target-side backup, download it, remove the target copy, and restore from the downloaded archive.

### Status

COMPLETE

## V4: LAN TLS

**Goal:** A LAN app can serve trusted HTTPS without ACME or self-signed browser warnings.

### Acceptance criteria

- TLS mode is explicit: `auto`, `off`, or `custom`.
- `custom` takes certificate and key paths from the controller.
- Certificate and key are validated before any proxy mutation.
- Certificate material is copied into the proxy volume and referenced by proxy TLS flags.
- `auto` retains ACME behavior; `off` retains HTTP behavior.
- Apply-diff includes TLS mode and certificate changes.
- Missing, unreadable, mismatched, or expired certificate material fails with an actionable error.
- Private key contents never appear in logs.

### Edge cases

- Certificate chain missing an intermediate.
- Key does not match certificate.
- Certificate expired or not yet valid.
- Certificate covers the wrong hostname.
- Proxy restart or recreate loses mounted files.
- Apply failure restores previous TLS settings.

### Out of scope

- Self-signed certificate generation.
- ACME DNS-01.
- Local CA installation.

### Demo

Deploy a LAN app with a custom certificate, open it without a browser warning, then apply an invalid certificate and confirm no proxy mutation occurs.

### Status

COMPLETE

## V5: Controller-Only Installer

**Goal:** A user can install Oma on a controller without local Docker and then manage remote targets.

### Acceptance criteria

- `OMA_LOCAL_DOCKER=false` skips local Docker installation and checks.
- Binary and background service installation still succeed.
- First-run documentation covers `oma target add`, `target setup`, and `--target`.
- Local-target commands fail clearly when Docker is absent.
- Default behavior without the environment flag is unchanged.
- The installer does not silently modify target configuration.

### Edge cases

- Docker installed but not running.
- Controller-only install followed by later local Docker installation.
- Background service starts without a target.
- Remote target unavailable during first run.

### Demo

Install with `OMA_LOCAL_DOCKER=false` on a Docker-free host, add a fake SSH target, and run `oma --target vps list`.

### Status

PENDING

## Deferred

| Item | Reason |
|---|---|
| Release signing | Separate key-management and verification project |
| Catalog signatures/trust | Remote catalogs are deferred |
| Self-signed certificates | Browser trust is the hard problem |
| ACME DNS-01 | Requires DNS provider credentials |
| Object-storage backups | Adds provider, credentials, and lifecycle support |
| Automated target runner install | Manual runner path first; automation later |
| Controller-only setup wizard | Layer on top of the environment flag later |

## Cross-Slice Notes

- Keep controller-side backup as the default to avoid silent migration.
- Validate TLS material before mutating proxy state.
- Never log secrets or private key contents.
- Reuse the existing operation lock for target-side writes.
- Checksum and self-update failures must be fail-closed.
