# Phase 7 — Operational UI

Status: complete (V1–V2)

Phase: 7
Related: [`slices.md`](slices.md) · [`../phase-6/slices.md`](../phase-6/slices.md) · [`../oma-fork-plan.md`](../oma-fork-plan.md)

## Frame

Phase 6 added agent automation and object-storage backups through the CLI, but the TUI still treated backups as a controller-or-target checkbox and hid automation ownership. This phase closes that surface gap without changing storage semantics.

## Requirements

| ID | Requirement | Priority |
| --- | --- | --- |
| R1 | Backup destination is selected explicitly as controller, target, or object storage | Must-have |
| R2 | Object storage does not require an application backup path and explains that credentials live on the target | Must-have |
| R3 | Target switching shows whether automation is controller-owned or agent-owned, including the agent version | Must-have |
| R4 | Existing controller and target backup behavior is unchanged | Must-have |
| R5 | Secrets are never shown in the TUI | Must-have |

## Shapes

### S1-A: Three-state destination selector

Use a select field in the existing Backups form. Selecting object clears the app-level path and shows the target-agent requirement. Selecting controller or target preserves the existing path behavior.

### S1-B: Separate object-storage form

A separate form would duplicate endpoint, bucket, and credential handling already owned by `oma target storage configure`.

S1-A is selected: the TUI chooses the destination; target-local object configuration remains a CLI/agent concern.

## Selected parts

| Part | Mechanism |
| --- | --- |
| **P1** | Reusable select form field |
| **P2** | Backups destination selector with controller/target/object states |
| **P3** | Target switcher automation labels |

## Acceptance

- A user can select object storage in the Backups form without supplying a path.
- The Backups status line distinguishes controller, target, and object storage.
- Run-now stays controller-only; target and object destinations explain that the agent runs the backup.
- The target switcher distinguishes controller-owned and agent-owned targets and shows the agent version.
- Existing controller and target submit behavior remains intact.
