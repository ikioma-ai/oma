# Release Checklist and Runbook

## Before tagging

- [ ] CI on `main` is green.
- [ ] The release commit is the intended commit.
- [ ] The installer image builds locally or the release workflow's CI image step is green.
- [ ] The release version follows `vMAJOR.MINOR.PATCH`; pre-release versions use a hyphenated suffix.
- [ ] The clean-host acceptance results in `slices.md` are current or explicitly noted as pending.
- [ ] No live server addresses, SSH credentials, or registry tokens are committed.

## Publish

```sh
git tag -a vX.Y.Z -m "Oma vX.Y.Z"
git push origin vX.Y.Z
```

The release workflow runs CI, builds the four binaries, verifies `checksums.txt`, pushes the installer image to GHCR, and creates the GitHub release.

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

## Clean acceptance

- [ ] Clean Linux host installs Oma, the background service, and an app.
- [ ] Clean Docker-free controller installs with `OMA_LOCAL_DOCKER=false`.
- [ ] Remote target setup, deploy, and HTTPS work.
- [ ] Home/LAN target works with a trusted custom certificate.
- [ ] Backup and restore work against the target.
- [ ] macOS clean install is recorded or explicitly pending.

## Rollback

### Installer image

Set `image`/`--version` back to the previous immutable installer tag, redeploy, and re-run `make installer-health`. The installer is stateless; the previous image is the rollback unit.

### Binary release

A bad binary release is fixed forward with a new patch release. Oma's self-updater does not downgrade installed clients. If an installer image is unhealthy, users are protected by fixing the image tag before the binary channel is affected.

## Support boundary

- One Docker daemon per target.
- Linux and macOS controllers; native Windows is not supported.
- Remote targets are Linux for automated setup.
- No multi-tenant accounts, fleet scheduling, or non-Docker runtimes.
- ONCE migration is not supported. Oma starts clean; there is no in-place import of ONCE labels, volumes, or services.
