// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package edit

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/r3labs/diff"
	"github.com/sergi/go-diff/diffmatchpatch"
)

const (
	validateTagName     = "validate"
	validateTagEditable = "editable"
	validateTagIgnore   = "ignore"
)

// ShowDiff write diff result into the Writer.
// return false if there's no diff.
func ShowDiff(t1 string, t2 string, w io.Writer) {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(t1, t2, false)
	diffs = dmp.DiffCleanupSemantic(diffs)

	fmt.Fprint(w, dmp.DiffPrettyText(diffs))
}

// ValidateSpecDiff checks and validates the new spec to see if the modified
// keys are all marked as editable
func ValidateSpecDiff(s1, s2 interface{}) error {
	differ, err := diff.NewDiffer(
		diff.TagName(validateTagName),
	)
	if err != nil {
		return err
	}
	changelog, err := differ.Diff(s1, s2)
	if err != nil {
		return err
	}

	if len(changelog) == 0 {
		return nil
	}

	msg := make([]string, 0)
	for _, c := range changelog {
		if len(c.Path) > 0 {
			// c.Path will be the tag value if TagName matched on the field
			if c.Type == diff.UPDATE && c.Path[len(c.Path)-1] == validateTagEditable {
				// If the field is marked as editable, it is allowed to be modified no matter
				// its parent level element is marked as editable or not
				continue
			}

			pathEditable := true
			pathIgnore := false
			for _, p := range c.Path {
				if _, err := strconv.Atoi(p); err == nil {
					// ignore slice offset counts
					continue
				}
				if p == validateTagIgnore {
					pathIgnore = true
					continue
				}
				if p != validateTagEditable {
					pathEditable = false
				}
			}
			// if the path has any ignorable item, just ignore it
			if pathIgnore || pathEditable && (c.Type == diff.CREATE || c.Type == diff.DELETE) {
				// If *every* parent elements on the path are all marked as editable,
				// AND the field itself is marked as editable, it is allowed to add or delete
				continue
			}
		}
		msg = append(msg, fmt.Sprintf("(%s) %s '%v' -> '%v'", c.Type, strings.Join(c.Path, "."), c.From, c.To))
	}

	if len(msg) > 0 {
		return fmt.Errorf("immutable field changed: %s", strings.Join(msg, ", "))
	}
	return nil
}
