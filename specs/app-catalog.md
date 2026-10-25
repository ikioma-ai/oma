---
name: app-catalog
type: feature
priority: 2
slices: [catalog-read-path, deploy-contract, environment-model, installer-resolution, catalog-apply]
---

{% section name="problem" %}

Oma's installable app list is compiled into the binary. Adding an app, changing its health check, exposing a non-default port, or requiring an environment value means changing code and shipping a release. Arbitrary Docker images can already be installed, but the operator must know the image reference and manually supply every deployment setting. The result is that "any compatible app" is technically possible but not practically easy, and a local catalog cannot be shared with the installer without duplicating knowledge.

This spec defines a data-driven catalog contract: embedded built-ins plus a local user catalog, a manifest that describes the deploy contract, an install-time environment form, installer routes for embedded entries, and an explicit apply operation for existing apps. The catalog is data only; it must not become a remote execution mechanism.

{% /section %}

{% section name="desired-outcome" %}

An operator can add or update one app by writing one manifest file, without rebuilding Oma. The TUI, CLI, installer, and deploy engine all read the same catalog. A manifest can declare the image, display metadata, container HTTP port, health check, persistent mount targets, TLS behavior, verification, and required or generated environment values. Installing an app snapshots those defaults into the app's settings. Changing a manifest later never silently changes an installed app; the operator previews and applies allowed changes explicitly. Existing image-based installs and pre-catalog settings continue to work.

The hosted installer serves only its embedded catalog, so a user's local apps are installed through the local binary, not through a hosted URL. Catalog failures are isolated: one invalid user manifest is reported and skipped while other entries remain usable.

{% /section %}

{% section name="context" %}

This contract sits between catalog data, the install workflow, the deploy engine, and the installer HTTP surface. It depends on the existing application settings and deploy behavior and is depended on by future worker and multi-container app kinds.

### Cross-references

- `docs/catalog/shaping.md` — requirements, selected shape, fit check, parts, and breadboard for this contract.
- `docs/catalog/slices.md` — implementation slices V1–V5, demos, and slice acceptance criteria.
- `docs/oma-fork-plan.md` — fork roadmap, catalog decisions, and deferred work.

### Implementation reference

- `internal/ui/install_app_list.go` — currently hardcodes the built-in app list and aliases.
- `internal/ui/install.go` and `internal/ui/install_activity.go` — install flow, hostname entry, and settings construction.
- `internal/docker/application.go` — deploy sequence, mount targets, environment construction, and HTTP verification.
- `internal/docker/proxy.go` — proxy deploy arguments, currently passing a bare container target and fixed health defaults.
- `internal/docker/application_settings.go` — persisted app settings and defaults.
- `internal/docker/application_backup.go` — backup entry names, primary mount selection, restore behavior.
- `installer/main.go` and `installer/templates/install.sh` — installer HTTP routes and the shell script that receives `--install`.
- `internal/command/root.go` and `internal/command/deploy.go` — CLI entry points and deploy flags.
- `internal/brand` — product and runtime identifiers.

### Key architectural decisions

- **Local-first catalogs** — built-ins are embedded; users add or replace manifests in a local directory. Remote catalogs and signing are deferred.
- **One manifest per app** — a user entry replaces the embedded entry with the same identifier as a whole. There is no field-level patching.
- **One volume, many mount targets** — the existing volume and backup format are preserved. Multiple named volumes are deferred.
- **Web apps only in this contract** — worker and multi-container kinds are future schema work.
- **Snapshot, then explicit apply** — installation copies resolved values into app settings. Catalog changes affect existing apps only through an explicit apply operation.

{% /section %}

{% section name="technical-direction" %}

- The catalog has two sources: embedded built-in manifests and user manifests in the Oma configuration directory. Resolution must work without network access. [AC#1]
- The user directory is `$XDG_CONFIG_HOME/oma/apps` when set, otherwise `~/.config/oma/apps`. [AC#1]
- Manifests are YAML. Every manifest requires `schemaVersion`; `kind` defaults to `web`. Unknown kinds and newer schema versions are rejected rather than partially interpreted. [AC#7]
- A manifest requires a stable identifier, image reference, display name, description, category, and may declare a suggested hostname. [AC#1]
- A user manifest with the same identifier as an embedded manifest replaces that entry as a whole. Ordering is deterministic by category, then name, then identifier. [AC#1]
- Manifest identifiers must be unique after merge. Duplicate user identifiers are reported and the affected entries are not used; silent last-wins behavior is forbidden. [AC#7]
- Validation must reject invalid image references, identifiers, ports, absolute-path violations, environment names, schema versions, unsupported kinds, multiple volume names, and host bind mounts. Errors must name the file and field. [AC#7]
- A manifest may declare the container HTTP port, health path, health port, and health timeout. The deployed proxy route and the post-deploy HTTP verification must use those values. [AC#2]
- A manifest may declare TLS mode and a verification toggle. Disabling TLS must disable proxy TLS and signal non-SSL operation to the app. Disabling verification must not delete app data. [AC#2]
- A manifest may declare multiple mount targets backed by one named volume. Backup must use the primary target; restore must preserve the existing archive format. [AC#2]
- A manifest may declare environment values that are required, defaulted, generated, secret, or derived from system settings. Generated values are created once and persisted. Secret values are masked outside the settings store. [AC#3]
- Install resolution order is: user catalog, embedded catalog, built-in alias, raw image reference. [AC#1,AC#4]
- Installation copies resolved manifest values into the app's persisted settings. Deploy, update, restart, and backup must not re-read the manifest for that app. [AC#2,AC#6]
- The installer serves `GET /apps.json` from the embedded catalog only and `GET /apps/{id}` as a shell script containing `--install=<id>`. The raw image route remains supported. A user-local identifier cannot be used to construct a hosted installer URL. [AC#4]
- `oma catalog apply` must show a field-level diff, require confirmation unless `--yes` is passed, apply only the supported field subset, and leave environment values and secrets unchanged by default. [AC#5]
- Supported apply fields are image, service port, health settings, TLS mode, verification, resources, and additional mount targets. Removing an entry or setting `disabled: true` must not alter installed apps. [AC#5]
- Existing settings without catalog fields, image-based installs, aliases, and pre-catalog backup archives must continue to work. [AC#6]
- Catalog loading is isolated per entry: one invalid user manifest is surfaced as an error and skipped while unrelated entries remain available. [AC#7]
- The catalog must be wired into the binary's external surfaces: the TUI app list, `oma catalog` commands, the installer routes, and the install/deploy path. [AC#1,AC#2,AC#3,AC#4,AC#5]
- Prefer a dedicated catalog module that owns parsing, validation, merge, and resolution. (guidance)
- Prefer immutable embedded catalog data and a fresh merged view per load. (guidance)
- Do not add a remote catalog, signature verification, or network fetch in this contract. (guidance)

{% /section %}

{% section name="domain-rules" %}

- **R1:** The catalog is data. No manifest value may execute a shell command or arbitrary code on the host.
- **R2:** `schemaVersion` is mandatory. A newer schema version or unsupported `kind` is rejected; no partial interpretation is allowed.
- **R3:** An identifier is stable and unique after merge. A user entry replaces the entire embedded entry with the same identifier.
- **R4:** Source precedence is user catalog before embedded catalog. Install precedence is catalog identifier before alias before raw image.
- **R5:** Installation snapshots resolved values. Existing apps do not re-resolve catalog data on deploy, update, restart, backup, or settings load.
- **R6:** A manifest may use one named volume and multiple absolute container mount targets. Host bind mounts and multiple volume names are forbidden.
- **R7:** Secret environment values are stored only in app settings and are never printed by normal CLI output or logs.
- **R8:** Catalog changes affect existing apps only through explicit apply. Apply never overwrites environment values or secrets by default.
- **R9:** The installer sees embedded data only. User-local entries cannot produce hosted installer URLs.
- **R10:** An invalid user entry is skipped with a file-and-field error; unrelated entries remain usable.
- **R11:** Pre-catalog settings labels and backup archives remain readable; new catalog fields are optional.

{% /section %}

{% section name="edge-cases-and-failure-modes" %}

- **Malformed user manifest:** The entry is rejected with the file and failing field, and the remaining catalog still loads. A single malformed file must not make the app list unusable.
- **Duplicate identifiers in the user catalog:** The duplicate entries are reported and excluded; the embedded entry remains available. Silent last-wins behavior is forbidden.
- **Newer schema version or unknown kind:** The entry is rejected with an actionable message. The implementation must not guess at unknown fields or partially apply the manifest.
- **Invalid deploy values:** Invalid image references, ports outside 1–65535, relative container paths, invalid environment names, and host bind mounts are rejected before any container or volume is created.
- **Missing required environment value:** Installation remains on the environment form and reports the missing field; no container is created.
- **Generated secret cannot be persisted:** Installation fails and cleans up any container or volume created for the attempt. The UI reports the persistence failure.
- **Apply when the manifest is missing or disabled:** Apply reports that there is no applicable manifest and changes nothing. The installed app remains running.
- **Hosted installer request for a user-local identifier:** The request is rejected; the local CLI can still install that identifier. The installer must not claim to know user-local data.
- **Concurrent manifest edit during install:** The install uses the snapshot it resolved at the start. The next install or apply reads the then-current manifest.
- **Pre-catalog backup archive:** Missing catalog fields take documented defaults; existing archive entries and data remain readable. Rewriting or migrating the archive is not required.

{% /section %}

{% section name="examples" %}

### Example 1: Add a local app and install it by identifier

- **Setup:** The operator writes `myapp.yaml` under `~/.config/oma/apps` with a new identifier, image, description, port, and health path.
- **Action:** The operator starts Oma and selects the new app, or runs `oma --install=myapp`.
- **Result:** The app appears in the list with source `user`, installs with the manifest defaults, and the resolved values are persisted in its settings.

### Example 2: Nonstandard port and health check

- **Setup:** A manifest declares container port 3000, health path `/healthz`, and mount targets `/data` and `/storage`.
- **Action:** The operator installs the app.
- **Result:** The proxy routes to the container on port 3000, health checks use `/healthz`, both mount targets point at the same named volume, and a restart preserves data.

### Example 3: Required and generated environment

- **Setup:** A manifest marks `DATABASE_URL` as required and declares a generated secret.
- **Action:** The operator installs the app and supplies the URL.
- **Result:** Installation is blocked until the URL is present, the secret is generated once and persisted, the app starts, and normal CLI output does not reveal the secret.

### Example 4: Apply a manifest change safely

- **Setup:** An installed app was created from `myapp.yaml`. The operator changes the image tag and health path in that manifest.
- **Action:** The operator runs `oma catalog apply <host>` and reviews the diff, then confirms.
- **Result:** Only the image and health settings change. Existing environment values and secrets are preserved. The app is redeployed without an automatic image pull unless the image field itself changed.

### Example 5: Invalid user manifest

- **Setup:** The user directory contains one valid manifest and one manifest with `kind: worker`.
- **Action:** Oma loads the catalog.
- **Result:** The valid app is listed. The worker entry is reported as unsupported and skipped. The catalog remains usable.

### Example 6: Hosted installer route

- **Setup:** The embedded catalog contains `writebook`.
- **Action:** A client requests `/apps/writebook` from the installer.
- **Result:** The response is a shell script that invokes `--install=writebook`. A request for a user-local-only identifier does not succeed, and a raw image reference route remains supported.

{% /section %}

{% section name="boundaries" %}

- **In scope:** Local and embedded manifest loading, validation, merge, resolution, display metadata, deploy contract mapping, install-time environment, installer embedded routes, explicit catalog apply, and backward compatibility for existing settings and backups.
- **Out of scope:** Remote catalog URLs, manifest signing, worker apps, multi-container stacks, multiple named volumes, automatic apply, a web dashboard, and remote machine targets.
- **Do not modify:** Existing application settings semantics beyond optional fields, existing backup archive entry names and data layout, existing proxy behavior beyond the new target-port and health flags, and the raw-image install path.

{% /section %}

{% section name="acceptance-criteria" %}

- [ ] AC#1 (integration): Given a user manifest in the Oma config directory and the embedded catalog, when the operator starts the TUI or runs `oma catalog list`, then the merged catalog is shown with the user entry taking precedence, deterministic ordering, and the entry's source visible.
- [ ] AC#2 (integration): Given a manifest declaring container port 3000, health path `/healthz`, and a data mount target, when the operator installs it, then the app is reachable through the proxy, verification uses the declared health path, data survives a restart, and backup and restore preserve it.
- [ ] AC#3 (integration): Given a manifest declaring a required environment value and a generated secret, when the operator installs the app, then installation blocks until the required value is supplied, the secret is generated once and persisted, and normal CLI output does not reveal it.
- [ ] AC#4 (integration): Given the embedded catalog, when a client requests `GET /apps.json`, `GET /apps/{id}`, and a raw image install route, then the JSON contains embedded entries, the per-app route emits `--install=<id>`, and the raw image route still succeeds; a user-local-only identifier is not available through the hosted installer.
- [ ] AC#5 (integration): Given an installed app and a changed catalog entry, when the operator runs `oma catalog apply` without confirmation, then a diff is shown and no change occurs; with confirmation or `--yes`, only image, service port, health settings, TLS mode, verification, resources, and additional mount targets change, and environment values and secrets remain unchanged.
- [ ] AC#6: Given an app created before the catalog feature with an image reference and legacy settings, when it is listed, deployed, backed up, and restored, then behavior matches the pre-catalog contract and no catalog fields are required.
- [ ] AC#7: Given an invalid user manifest, a duplicate identifier, a newer schema version, an unsupported kind, an invalid image reference, an invalid port or path, or a manifest with a host bind mount, when the catalog loads, then the affected entry is rejected with a file-and-field error while unrelated entries remain usable.
- [ ] AC#8: Given the full repository, when the unit test suites and the installer test suite run, then they pass and the existing integration contract tests remain unchanged.

{% /section %}

{% slice name="catalog-read-path" %}

Deliver the catalog data path: embedded manifests, user manifests, validation, merge, deterministic ordering, and read-only surfaces in the TUI app list and `oma catalog list`/`show`. Deploy behavior is unchanged in this slice.

**Acceptance Criteria:**
- Built-in apps still appear when no user manifests exist.
- A user manifest with a new identifier appears in the TUI and CLI without a rebuild.
- A user manifest with an existing identifier replaces the embedded entry and reports source `user`.
- A malformed or unsupported user manifest is skipped with a file-and-field error while valid entries remain.
- Ordering is deterministic by category, name, and identifier.
- `oma catalog validate` reports invalid entries without blocking valid ones.
- The existing app list tests and catalog unit tests pass.

{% /slice %}

{% slice name="deploy-contract" %}

Deliver manifest-driven deployment: service port, health settings, TLS mode, verification toggle, mount targets, and the install-time settings snapshot. Existing apps and legacy settings must behave as before.

**Acceptance Criteria:**
- A manifest with container port 3000 routes proxy traffic to that port.
- Health path, health port, and health timeout reach both proxy deployment and post-deploy verification.
- Multiple mount targets use one named volume; data survives restart and backup/restore.
- TLS mode `off` disables proxy TLS and signals non-SSL operation to the app.
- Verification disabled leaves the app and data intact.
- An app with no catalog fields deploys exactly as before.
- Old settings labels unmarshal and new fields are optional.
- The deploy and backup test suites pass.

{% /slice %}

{% slice name="environment-model" %}

Deliver manifest-declared environment values and the install-time form: required, defaulted, generated, secret, and system-derived values, with masking and persistence.

**Acceptance Criteria:**
- A required value blocks installation until supplied.
- A defaulted value is prefilled and can be overridden where allowed.
- A generated value is created once, persisted, and reused across redeploys.
- Secret values are masked in the TUI and normal CLI output.
- System-derived values such as base URL and SSL state are produced from existing settings.
- A persistence failure during generation fails the install and cleans up the attempt.
- Environment unit tests and the install-flow tests pass.

{% /slice %}

{% slice name="installer-resolution" %}

Deliver installer and resolution surfaces: `/apps.json`, `/apps/{id}`, `oma catalog add`, and install resolution across user catalog, embedded catalog, alias, and raw image.

**Acceptance Criteria:**
- `/apps.json` contains embedded entries only.
- `/apps/{id}` returns a shell script containing `--install=<id>`.
- Raw image installer routes remain valid.
- `--install` resolves a user identifier, an embedded identifier, an alias, and a raw image in that order.
- `oma catalog add` writes a valid user manifest that appears in the next catalog load.
- A user-local identifier is rejected by the hosted installer but installs through the local CLI.
- Invalid identifiers are rejected before shell rendering.
- Installer tests and resolution tests pass.

{% /slice %}

{% slice name="catalog-apply" %}

Deliver explicit catalog apply: diff generation, preview, confirmation, the supported field subset, safe settings update, and no environment or secret overwrite.

**Acceptance Criteria:**
- `oma catalog apply <host> --dry-run` prints a field-level diff and changes nothing.
- Apply without confirmation prompts before changing anything.
- `--yes` applies without a prompt.
- The supported subset is image, service port, health settings, TLS mode, verification, resources, and additional mount targets.
- Environment values and secrets are preserved.
- A failed apply restores the previous settings.
- Applying when the manifest is missing or disabled changes nothing.
- Apply and settings tests pass.

{% /slice %}
