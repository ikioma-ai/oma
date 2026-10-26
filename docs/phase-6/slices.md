# Phase 6 — Slices

Status: Phase 6 complete; V1–V6 complete
Shape: [`shaping.md`](shaping.md) · Roadmap: [`../oma-fork-plan.md`](../oma-fork-plan.md)

Workstreams are ordered **supply chain → agent → object storage**.

## Slice Grid

| V1: Release Signing | V2: Agent Install | V3: Agent Automation |
|---|---|---|
| ✅ COMPLETE | ✅ COMPLETE | ✅ COMPLETE |
| • RSA-SHA256 signature<br>• Installer/updater verify<br>• Key rotation metadata | • Signed binary upload<br>• Target service<br>• Agent role | • Agent-owned updates<br>• Agent-owned backups<br>• Status and logs |
| *Demo: tampered checksum/signature is rejected* | *Demo: signed agent installed over SSH* | *Demo: controller offline; agent updates and backs up* |

| V4: Object Backups | V5: Object Restore | V6: Rollout and Compatibility |
|---|---|---|
| ✅ COMPLETE | ✅ COMPLETE | ✅ COMPLETE |
| • Bucket/prefix config<br>• 0600 credentials<br>• Upload, encryption, retention | • List remote archives<br>• Download/restore<br>• Partial-transfer cleanup | • Migration from checksum-only<br>• Key rotation runbook<br>• Existing defaults preserved |
| *Demo: backup while controller is offline* | *Demo: restore from object storage* | *Demo: existing target keeps working* |

## V1: Release Signing

**Goal:** A tampered checksum manifest or binary is rejected by every install/update path.

### Acceptance criteria

- The release signs `checksums.txt` and publishes signature plus key metadata.
- The installer verifies the signature before verifying checksums and before installing.
- The self-updater verifies the signature before replacing a binary.
- Missing, malformed, unknown-key, or invalid signatures fail closed.
- Existing checksum-only artifacts remain readable until signing is mandatory.
- Release documentation covers key rotation and revocation.

### Edge cases

- Signature file missing.
- Public key ID unknown.
- Signature valid but checksum mismatch.
- Key rotated between installer download and agent install.
- Offline installer with no signature file.

### Demo

Tamper with `checksums.txt` and a binary; confirm installer and self-updater abort without changing the installed binary.

### Status

COMPLETE — `v0.5.0` publishes `checksums.txt.sig` and `checksums.txt.keyid`; installer and self-updater verify before use. Agent artifact verification is part of V2. Key rotation/revocation remains a V6 runbook item.

## V2: Agent Install

**Goal:** A target can run the signed Oma binary as a background agent installed over SSH.

### Acceptance criteria

- The controller verifies the signed checksum manifest before uploading the agent binary.
- The controller installs the signed agent binary and target-local service over SSH.
- No new public listener is opened.
- Installation is idempotent and records the agent service.
- Agent status and logs are reachable through the controller.
- Agent updates are signed and roll back on failure.
- Existing controller-driven targets are unchanged until agent ownership is selected.

### Edge cases

- SSH target is reachable but sudo is unavailable.
- Agent service is already installed with a different version.
- Agent binary signature fails after upload.
- Target reboots during installation.
- Controller disconnects after upload but before service start.

### Demo

Install the agent on a clean target, inspect its status over SSH, and reinstall idempotently.

### Status

COMPLETE — `v0.6.0` installs the signed Linux agent over SSH, records `agentVersion`, starts the target service, and exposes status/logs. No new listener was opened. Reinstall is idempotent; the update path restores the prior binary if service startup fails.

## V3: Agent Automation

**Goal:** An agent-owned target continues updating apps and writing backups without the controller.

### Acceptance criteria

- Automation ownership is explicit per target: controller or agent.
- Agent-owned targets run self-update, app updates, and backups locally.
- The controller does not duplicate updates or backups for agent-owned targets.
- Agent logs and status show ownership and last operation results.
- An agent-owned target continues working after the controller disconnects.
- Switching ownership back to the controller is explicit and safe.

### Edge cases

- Both controller and agent start an operation during ownership change.
- Agent cannot reach the object-storage bucket.
- Agent self-update fails.
- App update fails; previous app remains running.
- Controller reconnects while agent is updating.

### Demo

Disconnect the controller, let the agent update an app and write a backup, then reconnect and verify status.

### Status

COMPLETE — automation ownership is explicit (`controller` or `agent`); agent service runs with `--agent`, controller status shows owner, and disabling the agent returns ownership and stops the service. Controller/agent ownership lookup is unit-tested; the agent service is verified active with `agent=true` and no public listener.

## V4: Object Backups

**Goal:** Backups can be written to an S3-compatible bucket while the controller is offline.

### Acceptance criteria

- Backup destination accepts `object` with endpoint, bucket, prefix, and credentials.
- Credentials are stored in a target-local 0600 secret file, not labels or logs.
- Existing tar.gz archives upload unchanged with server-side encryption.
- Retention removes expired object archives.
- Upload failure leaves the existing archive and reports a clear error.
- Object backups work from the agent-owned target with no controller running.

### Edge cases

- Bucket does not exist.
- Credentials expire or are wrong.
- Network drops during upload.
- Object key already exists.
- Retention deletes the wrong prefix.

### Demo

Stop the controller, trigger an agent backup, and verify the object exists in the bucket.

### Status

COMPLETE — object backups are configured on the target, upload existing tar.gz archives with SSE from the agent, and remove expired archives by prefix. Verified in `v0.8.0`–`v0.8.1` with an agent-owned target and an S3-compatible mock; upload failures leave no partial archive and report the object key.

## V5: Object Restore

**Goal:** Object archives can be listed, downloaded, and restored.

### Acceptance criteria

- The controller lists object archives for a target.
- Download obtains a complete local archive and removes partial files on failure.
- Downloaded archives restore through the existing restore command.
- Archive names and prefixes are validated; traversal outside the prefix is rejected.
- Empty bucket and missing archive cases return actionable errors.
- Restore failure preserves the existing app volume.

### Edge cases

- Target offline.
- Archive disappears between list and download.
- Partial transfer.
- Insufficient controller disk.
- Bucket credentials only available on the agent.

### Demo

List an object archive, download it, and restore the app from the downloaded file.

### Status

COMPLETE — `v0.9.0` lists and downloads archives through the target agent, rejects traversal names, removes partial downloads, and restores through the existing `Namespace.Restore` path. Verified with a real object archive from an agent-owned target.

## V6: Rollout and Compatibility

**Goal:** Existing users can adopt signing, agents, and object storage without breaking current installs.

### Acceptance criteria

- Existing controller-side and target-side backups keep their defaults.
- Existing checksum-only installs continue to work during migration.
- The release checklist covers signing keys, agent rollout, and object-storage setup.
- Documentation covers key rotation, agent removal, and object-storage rollback.
- No live credentials or private keys are committed.
- Support boundaries for S3-compatible storage and agent-owned targets are documented.

### Edge cases

- Installer upgrade with an old checksum-only release.
- Key rotation while an old client is installed.
- Agent removal while automation ownership is still set to agent.
- Object-storage credentials removed while backups are due.

### Demo

Upgrade an existing installation, switch one target to agent ownership, then roll it back to controller ownership.

### Status

COMPLETE — defaults remain controller/target-side backups, the rollout/runbook covers signing keys, agent removal, object storage, and rollback, and support boundaries are documented in `rollout.md`.
