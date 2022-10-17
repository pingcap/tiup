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

package utils

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/pingcap/tiup/pkg/set"
	"github.com/r3labs/diff/v2"
	"github.com/sergi/go-diff/diffmatchpatch"
)

const (
	validateTagName       = "validate"
	validateTagEditable   = "editable"
	validateTagIgnore     = "ignore"
	validateTagExpandable = "expandable"
	// r3labs/diff drops everything after the first ',' in the tag value, so we use a different
	// separator for the tag value and its options
	validateTagSeperator = ":"
)

// ShowDiff write diff result into the Writer.
// return false if there's no diff.
func ShowDiff(t1 string, t2 string, w io.Writer) {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(t1, t2, false)
	diffs = dmp.DiffCleanupSemantic(diffs)

	fmt.Fprint(w, dmp.DiffPrettyText(diffs))
}

func validateExpandable(fromField, toField any) bool {
	fromStr, ok := fromField.(string)
	if !ok {
		return false
	}
	toStr, ok := toField.(string)
	if !ok {
		return false
	}
	tidyPaths := func(arr []string) []string {
		for i := 0; i < len(arr); i++ {
			arr[i] = strings.TrimSuffix(strings.TrimSpace(arr[i]), "/")
		}
		return arr
	}
	fromPaths := tidyPaths(strings.Split(fromStr, ","))
	toPaths := tidyPaths(strings.Split(toStr, ","))
	// The first path must be the same
	if len(fromPaths) > 0 && len(toPaths) > 0 && fromPaths[0] != toPaths[0] {
		return false
	}
	// The intersection size must be the same with from size
	fromSet := set.NewStringSet(fromPaths...)
	toSet := set.NewStringSet(toPaths...)
	inter := fromSet.Intersection(toSet)
	return len(inter) == len(fromSet)
}

// ValidateSpecDiff checks and validates the new spec to see if the modified
// keys are all marked as editable
func ValidateSpecDiff(s1, s2 any) error {
	differ, err := diff.NewDiffer(
		diff.TagName(validateTagName),
		diff.AllowTypeMismatch(true),
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
			_, leafCtl := parseValidateTagValue(c.Path[len(c.Path)-1])
			// c.Path will be the tag value if TagName matched on the field
			if c.Type == diff.UPDATE && leafCtl == validateTagEditable {
				// If the field is marked as editable, it is allowed to be modified no matter
				// its parent level element is marked as editable or not
				continue
			}

			pathEditable := true
			pathIgnore := false
			pathExpandable := false
			for _, p := range c.Path {
				key, ctl := parseValidateTagValue(p)
				if _, err := strconv.Atoi(key); err == nil {
					// ignore slice offset counts
					continue
				}
				if ctl == validateTagIgnore {
					pathIgnore = true
					continue
				}
				if ctl == validateTagExpandable {
					pathExpandable = validateExpandable(c.From, c.To)
				}
				if ctl != validateTagEditable {
					pathEditable = false
				}
			}
			// if the path has any ignorable item, just ignore it
			if pathIgnore || (pathEditable && (c.Type == diff.CREATE || c.Type == diff.DELETE)) || pathExpandable {
				// If *every* parent elements on the path are all marked as editable,
				// AND the field itself is marked as editable, it is allowed to add or delete
				continue
			}
		}

		// build error messages
		switch c.Type {
		case diff.CREATE:
			msg = append(msg, fmt.Sprintf("added %s with value '%v'", buildFieldPath(c.Path), c.To))
		case diff.DELETE:
			msg = append(msg, fmt.Sprintf("removed %s with value '%v'", buildFieldPath(c.Path), c.From))
		case diff.UPDATE:
			msg = append(msg, fmt.Sprintf("%s changed from '%v' to '%v'", buildFieldPath(c.Path), c.From, c.To))
		}
	}

	if len(msg) > 0 {
		return fmt.Errorf("immutable field changed: %s", strings.Join(msg, ", "))
	}
	return nil
}

func parseValidateTagValue(v string) (key, ctl string) {
	pvs := strings.Split(v, validateTagSeperator)
	switch len(pvs) {
	case 1:
		// if only one field is set in tag value
		// use it as both the field name and control command
		key = pvs[0]
		ctl = pvs[0]
	case 2:
		key = pvs[0]
		ctl = pvs[1]
	default:
		panic(fmt.Sprintf("invalid tag value %s for %s, only one or two fields allowed", v, validateTagName))
	}

	return key, ctl
}

func buildFieldPath(rawPath []string) string {
	namedPath := make([]string, 0)
	for _, p := range rawPath {
		pvs := strings.Split(p, validateTagSeperator)
		if len(pvs) >= 1 {
			namedPath = append(namedPath, pvs[0])
		}
	}
	return strings.Join(namedPath, ".")
}
