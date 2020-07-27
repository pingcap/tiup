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
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/juju/errors"
	"github.com/pingcap/tiup/pkg/base52"
	"github.com/pingcap/tiup/pkg/cliutil"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
)

// ShowAuditList show the audit list.
func ShowAuditList(dir string) error {
	firstLine := func(fileName string) (string, error) {
		file, err := os.Open(filepath.Join(dir, fileName))
		if err != nil {
			return "", errors.Trace(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		if scanner.Scan() {
			return scanner.Text(), nil
		}
		return "", errors.New("unknown audit log format")
	}

	// Header
	clusterTable := [][]string{{"ID", "Time", "Command"}}
	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, fi := range fileInfos {
		if fi.IsDir() {
			continue
		}
		ts, err := base52.Decode(fi.Name())
		if err != nil {
			continue
		}
		t := time.Unix(ts, 0)
		cmd, err := firstLine(fi.Name())
		if err != nil {
			continue
		}
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
	fname := filepath.Join(dir, base52.Encode(time.Now().Unix()))
	return ioutil.WriteFile(fname, data, 0644)
}

// ShowAuditLog show the audit with the specified auditID
func ShowAuditLog(dir string, auditID string) error {
	path := filepath.Join(dir, auditID)
	if tiuputils.IsNotExist(path) {
		return errors.Errorf("cannot find the audit log '%s'", auditID)
	}

	ts, err := base52.Decode(auditID)
	if err != nil {
		return errors.Annotatef(err, "unrecognized audit id '%s'", auditID)
	}

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Trace(err)
	}

	t := time.Unix(ts, 0)
	hint := fmt.Sprintf("- OPERATION TIME: %s -", t.Format("2006-01-02T15:04:05"))
	line := strings.Repeat("-", len(hint))
	_, _ = os.Stdout.WriteString(color.MagentaString("%s\n%s\n%s\n", line, hint, line))
	_, _ = os.Stdout.Write(content)
	return nil
}
