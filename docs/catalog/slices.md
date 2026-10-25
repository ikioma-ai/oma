# App Catalog — Slices

Selected shape: **A — Local-first single-file manifests**
Ground truth for R, fit check, parts, and breadboard: [`shaping.md`](shaping.md)
Related: [`../../specs/app-catalog.md`](../../specs/app-catalog.md) · [`../oma-fork-plan.md`](../oma-fork-plan.md)

Each slice is vertical: it ends with a demo-able user-visible result. Slices may land as separate commits or pull requests, but each keeps the build and tests green.

## Slice Overview

| # | Slice | Mechanism | Demo |
| --- | --- | --- | --- |
| V1 | Catalog read path and app list | Embedded and user manifests load, validate, merge; TUI and CLI list metadata | The app list is driven by YAML; a local manifest appears without a rebuild |
| V2 | Deploy contract | Port, health, TLS, verification, mount targets, settings snapshot | A nonstandard web app deploys and survives restart |
| V3 | Environment model | Required, default, generated, secret values; install form | An app requiring `DATABASE_URL` prompts for it and installs |
| V4 | Installer and install resolution | `/apps.json`, `/apps/{id}`, `--install` resolution, catalog add | A hosted install URL and a local catalog ID both install |
| V5 | Catalog apply | Diff, preview, confirmation, allowed field subset | Editing a manifest previews changes and applies them without touching env |

V1 is a prerequisite for V2–V5. V2 and V3 are independent after V1. V4 depends on V1. V5 depends on V2 and V3.

## Sliced Breadboard

```mermaid
flowchart TB
  subgraph V1["V1: CATALOG READ PATH AND APP LIST"]
    U1["U1: app row"]
    U13["U13: catalog list"]
    N1["N1: Load"]
    N2["N2: Merge"]
    N3["N3: Validate"]
    N7["N7: List/Show"]
    S1[("S1: embedded")]
    S2[("S2: user")]
    S3[("S3: merged")]
  end

  subgraph V2["V2: DEPLOY CONTRACT"]
    U4["U4: hostname"]
    U7["U7: install"]
    U9["U9: progress"]
    N5["N5: settings snapshot"]
    N6["N6: runInstall"]
    N8["N8: Deploy"]
    N12["N12: Proxy.Deploy"]
    N13["N13: mounts/config"]
    S4[("S4: settings label")]
    S5[("S5: Docker volume")]
  end

  subgraph V3["V3: ENVIRONMENT MODEL"]
    U5["U5: text env"]
    U6["U6: secret env"]
    U8["U8: validation error"]
    N9["N9: BuildEnv"]
  end

  subgraph V4["V4: INSTALLER AND RESOLUTION"]
    U2["U2: custom image"]
    U3["U3: image field"]
    U14["U14: catalog validate"]
    N4["N4: Resolve"]
    N14["N14: GET /apps.json"]
    N15["N15: GET /apps/{id}"]
    N16["N16: Add"]
    N17["N17: --install"]
    S6[("S6: installer binary")]
  end

  subgraph V5["V5: CATALOG APPLY"]
    U10["U10: source badge"]
    U11["U11: apply diff"]
    U12["U12: confirm apply"]
    U15["U15: catalog apply"]
    N10["N10: ApplyDiff"]
    N11["N11: UpdateSettings"]
  end

  U1 --> N4
  U13 --> N7
  U14 --> N3

  N1 --> N2
  S1 -.-> N1
  S2 -.-> N1
  N2 --> N3
  N2 --> S3
  N3 -.-> U8
  N7 --> U1
  S3 -.-> N4
  S3 -.-> N7

  U4 --> N5
  U7 --> N6
  N5 --> N6
  N5 --> S4
  N6 --> N8
  N6 --> U9
  N8 --> N12
  N8 --> N13
  N9 --> N8
  N12 --> S5
  N13 --> S5
  U5 --> N5
  U6 --> N5

  U2 --> P2["P2: Configuration"]
  U3 --> N4
  N17 --> N4
  N4 -.-> P2
  N14 --> S6
  N15 --> S6
  N16 --> S2

  U10 -.-> N7
  U11 -.-> N10
  U12 --> N11
  U15 --> N10
  N10 --> U11
  N10 --> U15
  N11 --> N8

  style V1 fill:#e8f5e9,stroke:#4caf50,stroke-width:2px
  style V2 fill:#e3f2fd,stroke:#2196f3,stroke-width:2px
  style V3 fill:#fff3e0,stroke:#ff9800,stroke-width:2px
  style V4 fill:#f3e5f5,stroke:#9c27b0,stroke-width:2px
  style V5 fill:#fff8e1,stroke:#ffc107,stroke-width:2px

  classDef ui fill:#ffb6c1,stroke:#d87093,color:#000
  classDef nonui fill:#d3d3d3,stroke:#808080,color:#000
  classDef store fill:#e6e6fa,stroke:#9370db,color:#000

  class U1,U2,U3,U4,U5,U6,U7,U8,U9,U10,U11,U12,U13,U14,U15 ui
  class N1,N2,N3,N4,N5,N6,N7,N8,N9,N10,N11,N12,N13,N14,N15,N16,N17 nonui
  class S1,S2,S3,S4,S5,S6 store
```

## V1: Catalog Read Path and App List

Status: complete.

**Goal:** The catalog is data, and both TUI and CLI read it.

**Parts:** A1, A2, A3, A4, A10

**Affordances**

| # | Affordance | Control | Notes |
| --- | --- | --- | --- |
| U1 | App row: name, description, category, source | click | Replaces the hardcoded list |
| U13 | `oma catalog list` table | invoke | Same merged catalog as the TUI |
| U14 | `oma catalog validate` report | invoke | One invalid user file does not break the list |
| N1 | `catalog.Load()` | call | Reads embedded and user manifests |
| N2 | `catalog.Merge()` | call | Whole-entry replacement by `id`, deterministic order |
| N3 | `catalog.Validate()` | call | File-aware errors |
| N7 | `catalog.List()` / `Show()` | call | Returns validated entries |
| S1 | Embedded catalog | store | Built-ins |
| S2 | User catalog | store | `$XDG_CONFIG_HOME/oma/apps` |
| S3 | Merged catalog | store | In memory |

**Demo:** Start Oma with a user manifest in `~/.config/oma/apps`; the app appears in the list with its description and source. `oma catalog list` shows the same entry.

**Acceptance**

- Built-in Campfire, Fizzy, and Writebook still appear.
- A user manifest with a new `id` appears without a rebuild.
- A user manifest with an existing `id` replaces the embedded entry.
- A malformed user manifest is skipped with a file-and-field error; other entries remain.
- Deterministic order: category, then name, then `id`.
- No deploy behavior changes in this slice.

**Out of scope:** Custom ports, health paths, env prompts, installer routes, apply.

## V2: Deploy Contract

Status: complete.

**Goal:** Catalog values control deployment without breaking existing apps.

**Parts:** A5, A6

**Affordances**

| # | Affordance | Control | Notes |
| --- | --- | --- | --- |
| U4 | Hostname field | type | Suggested value comes from the manifest |
| U7 | Install button | click | Uses resolved settings |
| U9 | Progress stages | render | Pull, start, verify |
| N5 | Settings snapshot | call | Copies manifest values into `ApplicationSettings` |
| N6 | `runInstall()` | call | Existing install flow |
| N8 | `Application.Deploy()` | call | Uses snapshot |
| N12 | `Proxy.Deploy()` | call | Receives target port and health flags |
| N13 | Mounts and container config | call | Multiple mount targets on one volume |
| S4 | Settings label | store | Optional new fields |
| S5 | Docker volume | store | One named volume |

**Demo:** Install an app that serves on port 3000 with `/healthz` and stores data at `/data`. It deploys, passes verification, survives restart, and backs up.

**Acceptance**

- `web.port: 3000` produces `--target <id>:3000`.
- `web.healthPath: /healthz` is used by both the proxy and HTTP verification.
- Multiple mount targets point at the same named volume.
- `tls: off` disables `--tls` and sets `DISABLE_SSL`.
- `verify: false` skips HTTP verification without deleting data.
- Existing apps with no catalog fields deploy exactly as before.
- Old settings labels unmarshal; new fields are optional.
- Backup and restore still use the primary mount target.

**Out of scope:** Multiple named volumes, custom command/user beyond the minimal settings field, non-web apps.

## V3: Environment Model

Status: complete.

**Goal:** A manifest can declare the env an app needs, and the install UI collects it.

**Parts:** A7, A10

**Affordances**

| # | Affordance | Control | Notes |
| --- | --- | --- | --- |
| U5 | Text env field | type | Required and optional |
| U6 | Secret env field | type | Masked |
| U8 | Validation error banner | render | Missing required value |
| N5 | Settings snapshot | call | Stores resolved env values |
| N9 | `BuildEnv()` | call | Generated, system-derived, and user values |

**Demo:** An app manifest requires `DATABASE_URL` and a generated `SECRET_KEY_BASE`. The install form asks for the URL, generates the secret, and the app starts.

**Acceptance**

- Required env values block install until supplied.
- Defaults are prefilled.
- Generated values are created once and persisted in settings.
- Secret fields are masked in the TUI and CLI output.
- `BASE_URL`, `DISABLE_SSL`, and SMTP values are derived from existing settings.
- User-provided env overrides generated values only where the schema allows it.
- No secret is printed in normal CLI output or logs.

**Out of scope:** Typed number/boolean/select fields, conditional fields, remote secret stores.

## V4: Installer and Install Resolution

**Goal:** Embedding and user catalogs both resolve at install time, and the hosted installer serves embedded data.

**Parts:** A2, A9, A10

**Affordances**

| # | Affordance | Control | Notes |
| --- | --- | --- | --- |
| U2 | Custom image row | click | Falls back to image entry |
| U3 | Image field | type | Resolves ID, alias, or image |
| U14 | `catalog validate` | invoke | Embedded and user files |
| N4 | `catalog.Resolve()` | call | User, embedded, alias, raw image |
| N14 | `GET /apps.json` | serve | Embedded catalog only |
| N15 | `GET /apps/{id}` | serve | Emits `--install=<id>` |
| N16 | `catalog add` | call | Writes a user manifest |
| N17 | `--install=<id-or-image>` | invoke | Works for scripts |
| S6 | Installer binary | store | Embedded catalog |

**Demo:** `curl https://<installer>/apps/writebook | sh` installs Writebook. A locally added `myapp` installs with `oma --install=myapp`. A raw image reference still works.

**Acceptance**

- `/apps.json` contains embedded entries only.
- `/apps/{id}` returns a shell script with `--install=<id>`.
- `/ghcr.io/foo/bar` remains a valid raw-image route.
- `--install=campfire` resolves the alias.
- `--install=myapp` resolves a user catalog entry.
- `--install=ghcr.io/foo/bar` resolves a raw image.
- A user catalog entry cannot be used to construct a hosted installer URL.
- Invalid IDs are rejected before the shell template is rendered.

**Out of scope:** Remote catalogs, signing, installer UI beyond the shell script.

## V5: Catalog Apply

**Goal:** A manifest change can be reviewed and applied explicitly, without silent redeploys or env overwrites.

**Parts:** A8, A10

**Affordances**

| # | Affordance | Control | Notes |
| --- | --- | --- | --- |
| U10 | Source badge | render | Shows `builtin` or `user` |
| U11 | Apply diff | render | Field-level before and after |
| U12 | Confirm apply | click | Requires confirmation or `--yes` |
| U15 | `oma catalog apply` | invoke | CLI equivalent |
| N10 | `ApplyDiff(host)` | call | Compares settings to manifest |
| N11 | `UpdateSettings()` | call | Redeploys with no pull |

**Demo:** Edit a user manifest to change the image tag and health path. `oma catalog apply books.example.com --dry-run` shows both changes; applying does not touch the app's env or secrets.

**Acceptance**

- Diff includes only the supported subset: image, service port, health settings, TLS mode, verification, resources, and additional mount targets.
- Env values and secrets are not overwritten by default.
- Confirmation is required unless `--yes` is passed.
- Apply uses `UpdateSettings`, not `Deploy`, so it does not pull a new image tag unless the image field itself changed.
- A failed apply restores the previous settings.
- Removing an entry from the catalog does not affect installed apps.
- Installed apps are unchanged when their manifest changes until apply runs.

**Out of scope:** Automatic apply, catalog-driven rollback, env migration.

## Slice Grid

|  |  |  |
| :-- | :-- | :-- |
| **V1: Catalog Read Path and App List**<br>✅ COMPLETE<br><br>• Manifest schema and validation<br>• Embedded and user loaders<br>• Merge by ID<br>• TUI and CLI list metadata<br><br>*Demo: A local manifest appears without a rebuild* | **V2: Deploy Contract**<br>✅ COMPLETE<br><br>• Port and health flags<br>• Mount targets<br>• TLS and verification<br>• Settings snapshot<br><br>*Demo: An app on port 3000 with `/healthz` deploys* | **V3: Environment Model**<br>✅ COMPLETE<br><br>• Required, default, generated values<br>• Secret fields<br>• Install form<br>• Env build<br><br>*Demo: A required `DATABASE_URL` is prompted and installed* |
| **V4: Installer and Install Resolution**<br>⏳ PENDING<br><br>• `/apps.json` and `/apps/{id}`<br>• ID, alias, image resolution<br>• `catalog add`<br>• `--install` paths<br><br>*Demo: Hosted and local catalog installs both work* | **V5: Catalog Apply**<br>⏳ PENDING<br><br>• Diff and preview<br>• Explicit confirmation<br>• Safe field subset<br>• No env overwrite<br><br>*Demo: Edit a manifest and preview the change* | **V6: Deferred Extensions**<br>⏳ OUT OF SCOPE<br><br>• Multiple named volumes<br>• Worker apps<br>• Multi-container stacks<br>• Remote catalogs<br><br>*Demo: Not part of this release* |
