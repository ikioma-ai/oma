# Phase 6 Rollout and Compatibility

## Upgrade path

Existing installs keep their defaults. Backup destinations that are unset or set to `target` continue to write target-side archives, and targets whose automation owner is unset remain controller-owned. No labels are rewritten during upgrade.

Checksum-only clients can still install and update until they reach a release that requires signatures. The first signed release must therefore keep publishing `checksums.txt`; once clients have upgraded, later releases may fail closed when `checksums.txt.sig` is missing. Test the upgrade from the oldest supported checksum-only release before changing signing policy.

## Agent rollout

1. Add the target and run `oma target setup <name>`.
2. Run `oma target agent install <name>`; this downloads the signed Linux binary, verifies its checksum, installs the service, and changes ownership to `agent`.
3. Check `oma target agent status <name>` and confirm `agent=true` in the service log. No new public listener is opened.
4. To return ownership to the controller, run `oma target agent disable <name>`. The service is disabled and the target resumes controller-driven automation.
5. To remove an agent completely, disable it, then remove `/usr/local/bin/oma` and the `oma-background` service on the target. Existing app containers and volumes are not touched.

If installation fails after upload, the installer restores the previous binary and restarts the service. If the service will not start, the installer fails without leaving the target agent-owned.

## Signing key rotation

- Key ID: `oma-release-2026-10`.
- Private key: GitHub Actions secret `OMA_RELEASE_SIGNING_KEY`; never commit or print it.
- Public key: embedded in the installer and self-updater.
- Rotation: retain the old public key until clients are upgraded, publish a release signed by the old key that includes the new key ID, then switch the signing secret. A rotation must ship in a release signed by a key clients already trust.
- Revocation: publish a replacement release and mark the compromised key ID revoked. Do not reuse a revoked key.

## Object-storage rollout

1. Create the bucket and any server-side encryption policy or lifecycle rules outside Oma. Oma does not create buckets or manage IAM.
2. Run `oma target storage configure <name> ... --secret-key-stdin`. The config is written to `/etc/oma/object-storage.json` with mode `0600`; credentials are never written to labels or logs.
3. Set affected apps to `--backup-destination object --auto-backup` using `oma deploy` or `oma update`.
4. Confirm the agent writes an archive with `oma target backups list <name>`.
5. To roll back, set the app destination back to `target`, update its path, and optionally remove `/etc/oma/object-storage.json`. Archives already in the bucket are not deleted.

Retention removes object archives under the configured prefix older than 30 days. Deleting the prefix or changing it can orphan existing archives; list and migrate them before changing the prefix.

## Support boundaries

- S3-compatible storage must implement SigV4 and support HTTPS endpoints, `PutObject`, `GetObject`, `StatObject`, `ListObjectsV2`, and `RemoveObject`.
- Server-side encryption is requested, but provider key management and encryption-at-rest guarantees remain the provider's responsibility.
- Object credentials live on the target; the controller only asks the target agent to list or download. The controller cannot read object storage while the target is offline.
- Agent-owned targets are Linux only for automated installation. A target has one Docker daemon and one backup owner at a time.
- Object retention is age-based and does not protect against a misconfigured prefix or bucket; operators should enable provider lifecycle/versioning policies where needed.
- Native Windows controllers and ONCE migration remain unsupported.
