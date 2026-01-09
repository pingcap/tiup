package output

import (
	"io"
	"sync/atomic"
)

type atomicWriterValue struct {
	w io.Writer
}

// AtomicWriter is a goroutine-safe, mutable reference to an io.Writer.
//
// It is useful for packages that need to emit user-facing output but also want
// to cooperate with a higher-level UI that may redirect output to a managed
// writer (e.g. a progress UI that has to keep terminal rendering stable).
//
// AtomicWriter stores a wrapper type in sync/atomic.Value to avoid the common
// panic caused by storing interface values of different concrete types (e.g.
// *os.File vs *bytes.Buffer).
type AtomicWriter struct {
	fallback io.Writer
	v        atomic.Value // atomicWriterValue
}

// NewAtomicWriter creates an AtomicWriter with a fallback writer.
//
// If fallback is nil, it defaults to io.Discard.
func NewAtomicWriter(fallback io.Writer) *AtomicWriter {
	if fallback == nil {
		fallback = io.Discard
	}
	aw := &AtomicWriter{fallback: fallback}
	aw.v.Store(atomicWriterValue{w: fallback})
	return aw
}

// Set sets the current writer.
//
// If w is nil, it resets back to the fallback writer.
func (aw *AtomicWriter) Set(w io.Writer) {
	if aw == nil {
		return
	}
	if w == nil {
		w = aw.fallback
	}
	aw.v.Store(atomicWriterValue{w: w})
}

// Get returns the current writer (or the fallback writer if none is set).
func (aw *AtomicWriter) Get() io.Writer {
	if aw == nil {
		return io.Discard
	}

	if v := aw.v.Load(); v != nil {
		if sv, ok := v.(atomicWriterValue); ok && sv.w != nil {
			return sv.w
		}
	}
	if aw.fallback != nil {
		return aw.fallback
	}
	return io.Discard
}
