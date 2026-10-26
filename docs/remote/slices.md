# Remote Targets — Slices

Selected shape: **A — Controller-driven Docker over SSH, transport-abstracted**
Ground truth for R, fit check, parts, and breadboard: [`shaping.md`](shaping.md)
Related: [`../../specs/remote-targets.md`](../../specs/remote-targets.md) · [`../oma-fork-plan.md`](../oma-fork-plan.md)

Each slice is vertical and ends with a demo-able operator surface. Slices may land as separate commits or pull requests, but each keeps the build and tests green.

## Slice Overview

| # | Slice | Mechanism | Demo |
| --- | --- | --- | --- |
| V1 | Target config and local selection | Config file, resolver, `target` CLI, `--target`, local default | `oma target list` and `oma --target local list` behave as today |
| V2 | SSH tunnel and remote operations | OpenSSH forward, Docker client injection, remote namespace, connection probe | Deploy and manage an app on a VPS |
| V3 | TUI target switching | Switcher, active-target badge, rebuilt app state | Switch from laptop to VPS in the dashboard |
| V4 | Remote observability and safety | Proxy metrics, host metrics or unavailable, `--no-verify`, backup labels | Remote dashboard and a safe verification failure |
| V5 | Target setup, provisioning, automation | Explicit `target setup`, provisioning readiness, `background run --target` | Bootstrap a VPS, then run scheduled updates from the controller |

V1 is the prerequisite for V2. V3 depends on V2. V4 depends on V2. V5 depends on V2 and V4.

## Sliced Breadboard

```mermaid
flowchart TB
  subgraph V1["V1: TARGET CONFIG AND LOCAL SELECTION"]
    U1["U1: target list"]
    U3["U3: --target"]
    N1["N1: load/save config"]
    N2["N2: resolve target"]
    N4["N4: local docker client"]
    N5["N5: namespace"]
    N14["N14: transport connect"]
  end

  subgraph V2["V2: SSH TUNNEL AND REMOTE OPERATIONS"]
    U2["U2: target check"]
    U7["U7: app actions"]
    N3["N3: start SSH forward"]
    N11["N11: stop and cleanup"]
  end

  subgraph V3["V3: TUI TARGET SWITCHING"]
    U4["U4: active target badge"]
    U5["U5: target switcher"]
    N6["N6: TUI target switch"]
  end

  subgraph V4["V4: REMOTE OBSERVABILITY AND SAFETY"]
    U6["U6: metrics or unavailable"]
    U8["U8: --no-verify"]
    N7["N7: proxy metrics"]
    N9["N9: host metrics"]
    N8["N8: verification"]
    N12["N12: backup label"]
  end

  subgraph V5["V5: TARGET CHECK, PROVISIONING, AUTOMATION"]
    U9["U9: provisioning readiness report"]
    N10["N10: target check"]
    N13["N13: background run --target"]
    N15["N15: operation lock"]
  end

  S1[("S1: target config")]
  S2[("S2: local socket")]
  P5["P5: remote Docker"]
  S4[("S4: metrics/backup metadata")]

  U1 --> N2
  U3 --> N2
  N1 --> S1
  N2 --> N1
  N2 --> N14
  N14 --> N4
  N4 --> N5
  N5 --> P5

  U9 --> N10
  N14 --> N3
  N3 --> S2
  N3 --> N4
  N4 --> N5
  N11 --> N3

  U5 --> N6
  N6 --> N2
  N6 --> U4

  U8 --> N8
  N7 --> S4
  N9 --> S4
  N8 --> P5
  N12 --> S4
  S4 -.-> U6

  N10 --> N1
  N10 --> N3
  N10 --> P5
  N13 --> N2
  N13 --> N5
  N13 --> N15
  N15 --> P5

  style V1 fill:#e8f5e9,stroke:#4caf50,stroke-width:2px
  style V2 fill:#e3f2fd,stroke:#2196f3,stroke-width:2px
  style V3 fill:#fff3e0,stroke:#ff9800,stroke-width:2px
  style V4 fill:#f3e5f5,stroke:#9c27b0,stroke-width:2px
  style V5 fill:#fff8e1,stroke:#ffc107,stroke-width:2px

  classDef ui fill:#ffb6c1,stroke:#d87093,color:#000
  classDef nonui fill:#d3d3d3,stroke:#808080,color:#000
  classDef store fill:#e6e6fa,stroke:#9370db,color:#000

  class U1,U2,U3,U4,U5,U6,U7,U8,U9 ui
  class N1,N2,N3,N4,N5,N6,N7,N8,N9,N10,N11,N12,N13,N14,N15 nonui
  class S1,S2,S4 store
```

## V1: Target Config and Local Selection

Status: complete.

**Goal:** Targets are data, and the local target remains the default.

**Affordances:** U1, U3, N1, N2, N4, N5, N14

**Demo:** `oma target list` shows `local` as the default. `oma --target local list` returns the same apps as `oma list`. A missing target name produces a clear error.

**Acceptance**

- Config is loaded from `~/.config/oma/config.yaml` or `$XDG_CONFIG_HOME/oma/config.yaml`.
- A missing config creates an implicit `local` target without writing a file.
- `target add`, `target list`, and `target remove` work for named targets.
- `--target` selects a target; `OMA_TARGET` is the fallback; the config default is last.
- Unknown target names fail before any Docker call.
- The local target uses the existing Docker client path and does not start SSH.
- Target resolution returns a transport implementation; the local implementation satisfies the same contract the future SSH and agent transports use.
- Command construction depends on the transport contract, not on a concrete SSH process.
- Removing the default target is rejected or reassigns the default explicitly.
- Unit and CLI tests pass.

**Out of scope:** SSH tunnels, TUI switching, remote metrics.

## V2: SSH Tunnel and Remote Operations

Status: complete.

**Goal:** A remote target behaves like the local target through the existing Docker API.

**Affordances:** U2, U7, N3, N4, N11

**Demo:** Add a VPS target, run `oma target check`, deploy an app with `oma --target vps deploy ...`, then list, stop, start, read logs, exec, back up, restore, and remove it.

**Acceptance**

- Target creation stores an SSH destination, Docker socket, and namespace.
- The tunnel starts OpenSSH with a short local socket and waits for readiness.
- The SSH implementation satisfies the same transport contract as the local target; commands and TUI code are unchanged.
- Docker operations use the injected remote client; no local Docker state is read.
- Deploy, list, start, stop, update, logs, exec, backup, restore, remove, and settings work remotely.
- App and proxy state appears only on the remote daemon.
- SSH host-key/auth failures, missing Docker, stopped Docker, and socket permission denial produce distinct errors.
- Exiting or cancelling closes the SSH process and removes the local socket.
- A connection probe verifies SSH, Docker, and socket access; provisioning readiness is added in V5.
- Local target behavior is unchanged.
- Unit, CLI, and integration tests pass, including a real SSH target when available.

**Out of scope:** TUI switching, metrics, automation.

## V3: TUI Target Switching

**Goal:** The dashboard clearly belongs to one target and can switch targets.

**Affordances:** U4, U5, N6

**Demo:** Start Oma, switch from laptop to VPS, see the VPS apps and active-target badge, perform an action, then switch back.

**Acceptance**

- The dashboard header shows the active target name.
- The target switcher lists configured targets and the default.
- Switching targets closes the previous tunnel and rebuilds namespace, apps, metrics, and event subscriptions.
- An SSH failure during switching keeps the previous target active and shows an error.
- Local target switching has no tunnel overhead.
- TUI tests cover the header, switcher, and failed switch.
- The local target remains the default at startup.

**Out of scope:** Fleet dashboards, simultaneous multi-target views.

## V4: Remote Observability and Safety

**Goal:** Remote telemetry comes from the target or is visibly unavailable, and a failed check cannot destroy data.

**Affordances:** U6, U8, N7, N8, N9, N12

**Demo:** Open the remote dashboard and see proxy request metrics plus target CPU/memory/disk (or an explicit unavailable label). Run a deploy with `--no-verify` and confirm the app stays. Induce a verification failure and confirm the data volume remains.

**Acceptance**

- Proxy metrics come from the target proxy, not the controller.
- Host CPU/memory/disk come from the target through SSH or show "unavailable"; controller values are never displayed for a remote target.
- `--no-verify` skips verification for deploy and apply.
- A failed verification removes the new deployment but leaves the data volume intact.
- Backup paths are labeled as controller-side when the target is remote.
- Metrics and backup labels update immediately after a target switch.
- Tests cover remote metrics, unavailable fallback, skipped verification, and non-destructive failure.

**Out of scope:** Remote-side backup storage, remote agent telemetry.

## V5: Target Setup, Provisioning, Automation

**Goal:** An operator can bootstrap a target explicitly, and understand what background automation does when the controller is offline.

**Affordances:** U9, N10, N13, N15

**Demo:** `oma target setup vps --yes` installs Docker if needed, fixes docker-group access, reconnects, verifies the socket, and reports DNS/firewall readiness. `oma --target vps background run` updates and backs up remote apps while the controller runs; the UI/docs state that a sleeping controller stops automation.

**Acceptance**

- `target add` stores a target without mutating the remote host.
- `target check` is read-only and preserves the V2 SSH, Docker, and socket results.
- `target setup` is explicit, confirmed, idempotent, and requires no remote Oma installation.
- Setup installs Docker when missing, adds the SSH user to the docker group when permission is denied, and reconnects to verify.
- Setup without sudo/root fails with exact manual commands.
- Setup never stores passwords, private keys, or tokens.
- Setup reports DNS and ports 80/443 readiness without changing firewall or DNS automatically.
- `background run --target` uses the selected target and performs update/backup checks against it.
- Interactive and background operations acquire the shared operation lock before deploy, update, or backup.
- The background runner depends on the target transport, not on a concrete SSH process.
- The background service records the target name and the controller-side backup path.
- The UI/docs state that a stopped controller stops remote automation.
- A remote-agent path and a target-local runner are not implemented, but the transport and runner seams are documented and tested at the interface level.
- Tests cover setup outcomes, check outcomes, background target selection, operation-lock contention, and offline-controller messaging.

**Out of scope:** Remote Oma agent, multi-target scheduling, remote-side backup storage, automatic DNS/firewall changes.

## Slice Grid

|  |  |  |
| :-- | :-- | :-- |
| **V1: Target Config and Local Selection**<br>✅ COMPLETE<br><br>• Target config<br>• Resolver<br>• CLI target commands<br>• Local default<br><br>*Demo: `target list` and `--target local`* | **V2: SSH Tunnel and Remote Operations**<br>✅ COMPLETE<br><br>• OpenSSH forward<br>• Docker client injection<br>• Remote CRUD<br>• Connection probe<br><br>*Demo: Deploy and manage a VPS app* | **V3: TUI Target Switching**<br>⏳ PENDING<br><br>• Active-target badge<br>• Switcher<br>• State rebuild<br>• Failed-switch handling<br><br>*Demo: Switch laptop to VPS* |
| **V4: Remote Observability and Safety**<br>⏳ PENDING<br><br>• Target metrics<br>• Unavailable fallback<br>• `--no-verify`<br>• Non-destructive failure<br><br>*Demo: Remote dashboard and safe failure* | **V5: Target Setup and Automation**<br>⏳ PENDING<br><br>• Explicit setup<br>• Provisioning check<br>• Background target<br>• Offline-controller notes<br><br>*Demo: Bootstrap a VPS and run backups* | **V6: Deferred Extensions**<br>⏳ OUT OF SCOPE<br><br>• Remote agent transport<br>• Target-local runner<br>• Fleet scheduling<br>• Docker TCP/TLS<br>• Remote-side backups<br><br>*Demo: Not part of this phase* |
