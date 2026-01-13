# AGENTS.md

TiUP Playground NG is used to start a single-node TiDB+TiKV+PD+... cluster locally, making it convenient for users (mainly developers) to try out features and run tests. It supports Linux and macOS.

Read doc/design/arch-playground-ng.md to learn how playground is designed and implemented.

This file contains **Playground-NG-specific** guidelines.
For general Go development guidelines shared by the whole repository, see `AGENTS.md` at repo root.

- Build playground NG: `make playground`

Note: Playground NG binary is named `tiup-playground-ng` to distinguish it from the existing `tiup-playground`. NG is the new generation of playground.

## Principles to Follow when Developing TiUP Playground-NG

This codebase will outlive you. Every shortcut you take becomes someone else's burden. Every hack compounds into technical debt that slows the whole team down.

You are not just writing code. You are shaping the future of this project. The patterns you establish will be copied. The corners you cut will be cut again.

Fight entropy. Leave the codebase better than you found it.

### Semantics and Abstractions

- **Concept constraints**: Playground is only allowed to have two core concepts: **component** and **service**. Do not introduce a third parallel concept (e.g., `namespace`). If it is truly unavoidable, you must clearly explain in the code what it is, what the boundary is between it and component/service, and why the existing concepts cannot express it.

### Plan/Runtime Consistency

- **Plan is the contract**: Execution should consume the plan as the single contract; avoid re-deriving runtime behavior from config/spec once the plan exists.
- **Align boot and scale-out semantics**: Boot-time creation and runtime scale-out should follow the same rules. If scale-out cannot consume a pre-built plan, it should reuse the exact same planning helper(s) used for plans.

### DRY and Structural Organization

- **Keep component code cohesive**: Try to keep `tiflash.go`/`tidb.go`/`tikv.go` to one file each (extra `*_config.go` files are allowed). Avoid a scattered structure like “5 files each mixing tiflash/tidb/tikv”.
- **Prefer a coarse-grained file layout**: In `components/playground-ng` (top-level), `components/playground-ng/proc` and `components/playground-ng/service`, prefer a few semantically cohesive files over many tiny ones. Avoid < 100 LOC files unless they are required (e.g. OS/build-tag specific files); aim for ~200-700 LOC per file when it keeps readability. However, don't merge multiple unrelated components/services into one file.

### Concurrency Model (Actor)

- **Single owner**: Playground’s runtime state (e.g., `procs`/`expectedExit`/service count statistics, etc.) is exclusively owned by the controller goroutine. Other goroutines may only interact with it through the event channel (`EmitEvent`/`doCommand`).
- **Avoid adding mutexes**: Do not introduce new mutexes just to read/write runtime state. When cross-goroutine notifications/changes are needed, define a `controllerEvent` and let the controller handle it.
- **UI is the exception**: Progress UI/output-redirection state may use the existing `progressMu` for minimal protection, but do not expand it into a “global big lock”.
