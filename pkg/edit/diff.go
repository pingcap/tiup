package edit

import (
	"fmt"
	"io"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// ShowDiff write diff result into the Writer.
// return false if there's no diff.
func ShowDiff(t1 string, t2 string, w io.Writer) {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(t1, t2, false)

	fmt.Fprint(w, dmp.DiffPrettyText(diffs))
}
