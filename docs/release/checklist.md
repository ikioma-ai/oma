# Release Checklist and Runbook

## Before tagging

- [ ] CI on `main` is green.
- [ ] The release commit is the intended commit.
- [ ] The installer image builds locally or the release workflow's CI image step is green.
- [ ] The release version follows `vMAJOR.MINOR.PATCH`; pre-release versions use a hyphenated suffix.
- [ ] The clean-host acceptance results in `slices.md` are current or explicitly noted as pending.
- [ ] No live server addresses, SSH credentials, registry tokens, object-storage secrets, or signing keys are committed.
- [ ] `OMA_RELEASE_SIGNING_KEY` is present and the public key in the installer/self-updater matches the key ID.
- [ ] The target agent artifact is present for both Linux architectures and passes signature/checksum verification.
- [ ] Existing checksum-only clients can still upgrade to this release; signing policy changes are documented.

## Publish

```sh
git tag -a vX.Y.Z -m "Oma vX.Y.Z"
git push origin vX.Y.Z
```

The release workflow runs CI, builds the four binaries, verifies `checksums.txt`, signs it with `OMA_RELEASE_SIGNING_KEY`, publishes `checksums.txt.sig` and `checksums.txt.keyid`, pushes the installer image to GHCR, and creates the GitHub release.

Verify:

```sh
gh release view vX.Y.Z
curl -fsS https://api.github.com/repos/ikioma-ai/oma/releases/latest
```

## Deploy the installer

Use the ignored local Kamal config:

```sh
cd installer
cp config/deploy.yml config/deploy.local.yml   # first time only
# Set servers, ssh.user, and the immutable image tag in deploy.local.yml
kamal -c config/deploy.local.yml deploy --skip-push --version=vX.Y.Z
```

Verify:

```sh
export OMA_INSTALLER_URL=https://get.oma.ikioma.ai
make installer-health
curl -fsS https://get.oma.ikioma.ai/apps/campfire | grep INSTALL_REF
```

## Agent and object-storage rollout

- [ ] `oma target agent install <name>` is idempotent and leaves `agent=true` in the service log.
- [ ] `oma target agent disable <name>` returns ownership to `controller`.
- [ ] A target with `target` backup destination keeps working without an object-storage config.
- [ ] Object-storage config is target-local, mode `0600`, and absent from labels and logs.
- [ ] Object list/download/restore were exercised against an agent-owned target.
- [ ] The rollout and rollback steps in [`docs/phase-6/rollout.md`](../phase-6/rollout.md) are current.

## Signing key

- Key ID: `oma-release-2026-10`.
- Private key: GitHub Actions secret `OMA_RELEASE_SIGNING_KEY`; never commit it.
- Public key: embedded in the installer template and self-updater.
- Rotation: retain the previous public key until clients are upgraded, publish a new key ID, and only then switch the signing secret. A rotation must ship in a release signed by a key clients already trust.
- Revocation: mark the key ID revoked and publish a replacement release; do not silently reuse a revoked key.

## Clean acceptance

- [ ] Clean Linux host installs Oma, the background service, and an app.
- [ ] Clean Docker-free controller installs with `OMA_LOCAL_DOCKER=false`.
- [x] Remote target setup, deploy, and HTTPS work.
- [ ] Home/LAN target works with a trusted custom certificate.
- [x] Backup and restore work against the target.
- [ ] macOS clean install is recorded or explicitly pending.

## Rollback

### Installer image

Set `image`/`--version` back to the previous immutable installer tag, redeploy, and re-run `make installer-health`. The installer is stateless; the previous image is the rollback unit.

### Binary release

A bad binary release is fixed forward with a new patch release. Oma's self-updater does not downgrade installed clients. If an installer image is unhealthy, users are protected by fixing the image tag before the binary channel is affected.

### Object storage and agents

Set affected apps back to the `target` backup destination and update their backup path, then remove or leave `/etc/oma/object-storage.json` on the target. Existing objects are not deleted by Oma. Disable an agent with `oma target agent disable <name>` before removing its binary/service; container and volume data are untouched. See [`docs/phase-6/rollout.md`](../phase-6/rollout.md).

## Support boundary

- One Docker daemon per target.
- Linux and macOS controllers; native Windows is not supported.
- Remote targets are Linux for automated setup; the target-local agent is installed only on Linux.
- Object storage must be S3-compatible with SigV4; Oma does not create buckets or manage IAM.
- Object credentials stay on the target, so object listing/downloading requires the target to be online.
- No multi-tenant accounts, fleet scheduling, or non-Docker runtimes.
- ONCE migration is not supported. Oma starts clean; there is no in-place import of ONCE labels, volumes, or services.
