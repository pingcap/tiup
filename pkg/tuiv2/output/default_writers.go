package output

import "os"

// Stdout is the shared, mutable writer for user-facing standard output.
//
// It defaults to os.Stdout. Higher-level UIs (e.g. progress UI) may redirect it
// to a UI-managed writer to keep terminal rendering stable.
var Stdout = NewAtomicWriter(os.Stdout)

// Stderr is the shared, mutable writer for user-facing standard error output.
//
// It defaults to os.Stderr. Higher-level UIs (e.g. progress UI) may redirect it
// to a UI-managed writer to keep terminal rendering stable.
var Stderr = NewAtomicWriter(os.Stderr)
