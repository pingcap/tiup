# AGENTS.md

TiUP Playground is used to start a single-node TiDB+TiKV+PD+... cluster locally, making it convenient for users (mainly developers) to try out features and run tests. It supports Linux and macOS.

Read doc/design/arch-playground.md to learn how playground is designed and implemented.

- **Keep AGENTS.md generic**: Put only general, reusable guidelines here; avoid rules that are specific to one feature or business workflow.

## Common Commands

Under TiUP directory:

- Build: make playground
- Lint: make lint && make check-static

## Principles to Follow when Developing TiUP Playground

This codebase will outlive you. Every shortcut you take becomes someone else's burden. Every hack compounds into technical debt that slows the whole team down.

You are not just writing code. You are shaping the future of this project. The patterns you establish will be copied. The corners you cut will be cut again.

Fight entropy. Leave the codebase better than you found it.

### Semantics and Abstractions

- **Concept constraints**: Playground is only allowed to have two core concepts: **component** and **service**. Do not introduce a third parallel concept (e.g., `namespace`). If it is truly unavoidable, you must clearly explain in the code what it is, what the boundary is between it and component/service, and why the existing concepts cannot express it.
- **Don’t treat “looks similar” as “equivalent”**: Different concepts should not be treated as equivalent even if their fields/strings are the same; abstractions must correspond to real semantics.
- **Abstractions must be meaningful**: Introduce an abstraction only when it reduces cognitive load/error probability/repetition and has clear semantics; otherwise, prefer straightforward, intuitive code.

### Testability

- **Avoid over-abstraction for tests**: Prefer simple policy knobs/configuration to stabilize tests over introducing heavy test-only injection layers. Unit-test OS-dependent helpers separately.
- **Use `testify/require` for assertions**: Prefer `require.Equal`/`require.NoError`/`require.Contains` etc over hand-written assertion blocks like `if !slices.Equal(...) { t.Fatalf(...) }`.

### DRY and Structural Organization

- **DRY**: Don’t copy and paste large blocks of identical logic; wherever `for` loops/table-driven approaches/unified helpers can be used, duplication must be eliminated.
- **No subset helpers**: If the functionality of `A()` is a subset of `B()` (and has almost no extra semantics), delete `A()` and standardize on `B()`.
- **Keep component code cohesive**: Try to keep `tiflash.go`/`tidb.go`/`tikv.go` to one file each (extra `*_config.go` files are allowed). Avoid a scattered structure like “5 files each mixing tiflash/tidb/tikv”.
- **Prefer a coarse-grained file layout**: In `components/playground` (top-level), `components/playground/proc` and `components/playground/service`, prefer a few semantically cohesive files over many tiny ones. Avoid < 100 LOC files unless they are required (e.g. OS/build-tag specific files); aim for ~200-700 LOC per file when it keeps readability. However, don't merge multiple unrelated components/services into one file.
- **Maintainable ordering**: Avoid manually writing and maintaining things like “service traversal order/priority lists”, which are easy to miss. Prefer deterministic ordering (e.g., sorting by key) or explicit dependencies (e.g., `StartAfter`).

### Concurrency Model (Actor)

- **Single owner**: Playground’s runtime state (e.g., `procs`/`expectedExit`/service count statistics, etc.) is exclusively owned by the controller goroutine. Other goroutines may only interact with it through the event channel (`EmitEvent`/`doCommand`).
- **Avoid adding mutexes**: Do not introduce new mutexes just to read/write runtime state. When cross-goroutine notifications/changes are needed, define a `controllerEvent` and let the controller handle it.
- **UI is the exception**: Progress UI/output-redirection state may use the existing `progressMu` for minimal protection, but do not expand it into a “global big lock”.

### Avoid “Pointless Encapsulation”

- **Don’t write meaningless getters/setters**: Unless you need to maintain invariants/isolate concurrency/provide essential semantics, directly exposing fields or reading/writing them is clearer.
- **Don’t write one-line wrapper functions**: For example, a helper that merely wraps an assignment/construction (without expressing additional semantics) should be removed; writing the real code directly is easier to understand.
- **Method ownership**: If a function is essentially a behavior/property of a struct (e.g., instance name, display name), make it a method on that struct (e.g., `info.Name()`), rather than a “dangling” free function.

### Nil and Optional Parameters (Strict Constraints)

- **Don’t treat `nil` as an “optional parameter/state”**: Unless the semantics truly require a third “missing” state, establish invariants at the call site and ensure parameters are non-`nil`, avoiding repeated `if x != nil` checks at every layer.
  - Rust analogy: If you don’t actually need the tri-state semantics of `Option<T>` (Some/None), you shouldn’t introduce `Option<T>`; the same applies in Go—reduce parameter uncertainty and prefer guaranteeing “never nil” at the call site.
- **No `nil` checks after `make(...)`**: The result of `make(chan/map/slice)` can never be `nil`; such checks must be removed.
