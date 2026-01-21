# AGENTS.md

General Go development guidelines for this repository.

Feature-specific rules should live next to the code (via directory `AGENTS.md` files) and only include what is truly unique to that feature.

## Common Commands

- Lint: `make lint && make check-static`

After making changes, ensure build and lint pass before committing.

## Engineering Principles

### Semantics and Abstractions

- **Don’t treat “looks similar” as “equivalent”**: Different concepts should not be treated as equivalent even if their fields/strings are the same; abstractions must correspond to real semantics.
- **Abstractions must be meaningful**: Introduce an abstraction only when it reduces cognitive load/error probability/repetition and has clear semantics; otherwise, prefer straightforward, intuitive code.

### Single Source of Truth

- **Single source of truth**: If a value is derivable from a declaration/schema, do not maintain a second handwritten implementation elsewhere. Prefer one canonical representation and shared helpers so behavior cannot drift.
- **No parallel “ways of doing the same thing”**: Avoid having multiple allocation/normalization/construction paths for the same kind of data; choose one path and delete the others.
- **Normalize once, reuse everywhere**: If an input must be normalized/canonicalized (trim, sanitize, defaulting rules), compute the canonical value once at the boundary and use it consistently—never mix raw and normalized forms.
- **Centralize cross-module keys**: When data is exchanged via maps or loosely typed fields, define keys/constants once and reuse them everywhere; do not scatter magic strings.
- **Validate early, fail early**: Prefer validating spec/schema declarations at registration/init time instead of letting invalid state surface deep in execution.

### Naming and Literals

- **Avoid low-value constants**: Don’t extract constants just to wrap literals (even if used a few times). Only extract when it meaningfully improves readability, expresses semantics, or represents a shared contract.
- **Avoid no-value locals**: Don’t introduce local variables that simply mirror a field/map lookup and are used once; inline the expression unless the name adds essential semantics.
- **Keys should be semantic, not redundant**: When defining internal map keys, pick concise names that describe the concept; avoid redundant suffixes/prefixes implied by the container.
- **Only constantize stable contracts**: Use shared constants when a key becomes a cross-module contract or is used widely; otherwise, local string literals are often clearer than long, low-value constants.

### Testability

- **Avoid over-abstraction for tests**: Prefer simple policy knobs/configuration to stabilize tests over introducing heavy test-only injection layers. Unit-test OS-dependent helpers separately.
- **Use `testify/require` for assertions**: Prefer `require.Equal`/`require.NoError`/`require.Contains` etc over hand-written assertion blocks.

### Documentation and Rationale

- **Explain non-obvious complexity**: If a change introduces logic that looks “overly complex”, add a short but explicit comment near the code describing the constraint that forced it, why a simpler approach does not work (or what would break), and what would need to change to make simplification safe later.

### DRY and Structural Organization

- **DRY**: Don’t copy and paste large blocks of identical logic; wherever `for` loops/table-driven approaches/unified helpers can be used, duplication must be eliminated.
- **No subset helpers**: If the functionality of `A()` is a subset of `B()` (and has almost no extra semantics), delete `A()` and standardize on `B()`.
- **Maintainable ordering**: Avoid manually writing and maintaining things like traversal order/priority lists, which are easy to miss. Prefer deterministic ordering (e.g., sorting by key) or explicit dependencies.

### Avoid “Pointless Encapsulation”

- **Don’t write meaningless getters/setters**: Unless you need to maintain invariants/isolate concurrency/provide essential semantics, directly exposing fields or reading/writing them is clearer.
- **Don’t write one-line wrapper functions**: For example, a helper that merely wraps an assignment/construction (without expressing additional semantics) should be removed.
- **Method ownership**: If a function is essentially a behavior/property of a struct, make it a method on that struct rather than a “dangling” free function.

### Nil and Optional Parameters

- **Don’t treat `nil` as an “optional parameter/state”**: Unless the semantics truly require a third “missing” state, establish invariants at the call site and ensure parameters are non-`nil`, avoiding repeated checks at every layer.
- **No `nil` checks after `make(...)`**: The result of `make(chan/map/slice)` can never be `nil`; such checks must be removed.
