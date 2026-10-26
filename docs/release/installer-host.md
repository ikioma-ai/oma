# Installer Host Deployment

Status: deployed and verified at `https://get.oma.ikioma.ai` using `v0.5.0`.

## Inputs

- Final installer hostname: `get.oma.ikioma.ai`.
- One or more Linux servers with SSH access, Docker, and ports 80/443 reachable.
- DNS control for `ikioma.ai` to create the `get.oma` record.
- The GHCR installer image tag from the release.
- A public GHCR package or registry credentials available to the servers.

## Configure

The committed `installer/config/deploy.yml` is a template. Do not put live server addresses or credentials in it.

```sh
cd installer
cp config/deploy.yml config/deploy.local.yml
```

Edit `config/deploy.local.yml`:

- Set `image` to the immutable release tag, for example:
  `ghcr.io/ikioma-ai/oma-installer-service:v0.5.0`.
- Set `servers` to the real SSH destinations.
- Leave `proxy.host` set to `get.oma.ikioma.ai`.
- Set `builder.arch` to match the servers.
- Set `ssh.user` to the deployment user.

`config/deploy.local.yml` is ignored by git.

## DNS and ports

- DNS is configured in DNSimple: `get.oma.ikioma.ai` A → `85.9.222.162` (record `87812721`, TTL 300).
- Allow inbound TCP 22 for deployment, and TCP 80/443 for public installer traffic.
- Let Kamal provision HTTPS through its proxy; do not point the hostname at an old installer.

## Deploy

```sh
cd installer
kamal -c config/deploy.local.yml deploy
```

## Current deployment

- Server: `85.9.222.162`
- Kamal application: `oma-installer`
- Release image: `ghcr.io/ikioma-ai/oma-installer-service:v0.5.0`
- HTTPS: Let's Encrypt through Kamal proxy
- Health: `https://get.oma.ikioma.ai/up` returns `OK`
- Routes: `/`, `/apps.json`, and `/apps/{id}` verified
- Release: `v0.5.0` is the current stable GitHub release; `oma-*` assets and checksums are published

## Verify

Use the hostname, not a local installer process:

```sh
export OMA_INSTALLER_URL=https://get.oma.ikioma.ai
make installer-health
```

The check verifies:

- `GET /up` returns `200 OK`.
- `GET /apps.json` returns the embedded catalog.
- `GET /` returns the install script.
- The script contains the expected repository and checksum asset references.

## Rollback

Change `image` in `config/deploy.local.yml` to the previous immutable release tag, then redeploy:

```sh
cd installer
kamal -c config/deploy.local.yml deploy
```

Verify `/up` and the install routes again. A binary release is forward-fixed with a new patch release; the self-updater does not downgrade.

## First-release checklist

- [x] `get.oma.ikioma.ai` resolves to `85.9.222.162`.
- [x] TCP 22, 80, and 443 are reachable.
- [x] HTTPS `/up` returns `200 OK`.
- [ ] Ports 80/443 reachable.
- [ ] GHCR installer package is public or server credentials are configured.
- [ ] `config/deploy.local.yml` points at an immutable release tag.
- [ ] `kamal -c config/deploy.local.yml deploy` succeeds.
- [ ] `make installer-health` passes.
- [ ] `GET /apps/campfire` renders the expected install script.
- [ ] Rollback tag is recorded and tested.
