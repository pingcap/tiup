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

package audit

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/base52"
	"github.com/pingcap/tiup/pkg/cliutil"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/utils/rand"
)

// CommandArgs returns the original commands from the first line of a file
func CommandArgs(fp string) ([]string, error) {
	file, err := os.Open(fp)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return nil, errors.New("unknown audit log format")
	}

	args := strings.Split(scanner.Text(), " ")
	return DecodeCommandArgs(args)
}

// EncodeCommandArgs encode args with url.QueryEscape
func EncodeCommandArgs(args []string) []string {
	encoded := []string{}

	for _, arg := range args {
		encoded = append(encoded, url.QueryEscape(arg))
	}

	return encoded
}

// DecodeCommandArgs decode args with url.QueryUnescape
func DecodeCommandArgs(args []string) ([]string, error) {
	decoded := []string{}

	for _, arg := range args {
		a, err := url.QueryUnescape(arg)
		if err != nil {
			return nil, errors.Annotate(err, "failed on decode the command line of audit log")
		}
		decoded = append(decoded, a)
	}

	return decoded, nil
}

// ShowAuditList show the audit list.
func ShowAuditList(dir string) error {
	// Header
	clusterTable := [][]string{{"ID", "Time", "Command"}}
	fileInfos, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, fi := range fileInfos {
		if fi.IsDir() {
			continue
		}
		t, err := decodeAuditID(fi.Name())
		if err != nil {
			continue
		}
		args, err := CommandArgs(filepath.Join(dir, fi.Name()))
		if err != nil {
			continue
		}
		cmd := strings.Join(args, " ")
		clusterTable = append(clusterTable, []string{
			fi.Name(),
			t.Format(time.RFC3339),
			cmd,
		})
	}

	sort.Slice(clusterTable[1:], func(i, j int) bool {
		return clusterTable[i+1][1] > clusterTable[j+1][1]
	})

	cliutil.PrintTable(clusterTable, true)
	return nil
}

// OutputAuditLog outputs audit log.
func OutputAuditLog(dir string, data []byte) error {
	fname := filepath.Join(dir, base52.Encode(time.Now().UnixNano()+rand.Int63n(1000)))
	return os.WriteFile(fname, data, 0644)
}

// ShowAuditLog show the audit with the specified auditID
func ShowAuditLog(dir string, auditID string) error {
	path := filepath.Join(dir, auditID)
	if tiuputils.IsNotExist(path) {
		return errors.Errorf("cannot find the audit log '%s'", auditID)
	}

	t, err := decodeAuditID(auditID)
	if err != nil {
		return errors.Annotatef(err, "unrecognized audit id '%s'", auditID)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return errors.Trace(err)
	}

	hint := fmt.Sprintf("- OPERATION TIME: %s -", t.Format("2006-01-02T15:04:05"))
	line := strings.Repeat("-", len(hint))
	_, _ = os.Stdout.WriteString(color.MagentaString("%s\n%s\n%s\n", line, hint, line))
	_, _ = os.Stdout.Write(content)
	return nil
}

//decodeAuditID decodes the auditID to unix timestamp
func decodeAuditID(auditID string) (time.Time, error) {
	ts, err := base52.Decode(auditID)
	if err != nil {
		return time.Time{}, err
	}
	// compatible with old second based ts
	if ts>>32 > 0 {
		ts /= 1e9
	}
	t := time.Unix(ts, 0)
	return t, nil
}
