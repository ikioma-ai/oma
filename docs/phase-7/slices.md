# Phase 7 — Slices

Status: Phase 7 complete; V1–V2 complete
Shape: [`shaping.md`](shaping.md) · Roadmap: [`../oma-fork-plan.md`](../oma-fork-plan.md)

| V1: Backup Destination UI | V2: Automation Visibility |
|---|---|
| ✅ COMPLETE | ✅ COMPLETE |
| • Three-state selector<br>• Object path exemption<br>• Agent requirement status | • Controller/agent labels<br>• Agent version<br>• Target switcher tests |
| *Demo: choose object storage without a path* | *Demo: identify an agent-owned target* |

## V1: Backup Destination UI

**Goal:** The Backups form exposes the same three destinations as the CLI.

### Acceptance criteria

- Controller, target, and object storage are selectable.
- Object storage submits an empty application backup path.
- Controller and target submissions keep their existing path rules.
- The status line explains where the backup runs and what is required.
- Run-now is unavailable for target and object destinations.

### Edge cases

- Switching from object back to controller with no path validates as before.
- Auto-backup with object storage needs no path.
- A target destination without a path still fails validation.

### Status

COMPLETE — the form uses a reusable select field; object storage clears the path and shows the target-agent requirement. Tests cover controller, target, and object submissions.

## V2: Automation Visibility

**Goal:** Operators can see which targets run automation locally.

### Acceptance criteria

- Non-local controller-owned targets are labeled `controller`.
- Agent-owned targets are labeled `agent` with the installed version.
- The local target keeps its existing label.
- Target selection behavior is unchanged.

### Status

COMPLETE — the target switcher renders ownership suffixes and the agent version; tests cover both ownership states.
