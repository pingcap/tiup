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
	"encoding/json"
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
	"github.com/pingcap/tiup/pkg/crypto/rand"
	"github.com/pingcap/tiup/pkg/tui"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
)

const (
	// EnvNameAuditID is the alternative ID appended to time based audit ID
	EnvNameAuditID = "TIUP_AUDIT_ID"
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
	return decodeCommandArgs(args)
}

// encodeCommandArgs encode args with url.QueryEscape
func encodeCommandArgs(args []string) []string {
	encoded := []string{}

	for _, arg := range args {
		encoded = append(encoded, url.QueryEscape(arg))
	}

	return encoded
}

// decodeCommandArgs decode args with url.QueryUnescape
func decodeCommandArgs(args []string) ([]string, error) {
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

	auditList, err := GetAuditList(dir)
	if err != nil {
		return err
	}

	for _, item := range auditList {
		clusterTable = append(clusterTable, []string{
			item.ID,
			item.Time,
			item.Command,
		})
	}

	tui.PrintTable(clusterTable, true)
	return nil
}

// Item represents a single audit item
type Item struct {
	ID      string `json:"id"`
	Time    string `json:"time"`
	Command string `json:"command"`
}

// GetAuditList get the audit item list
func GetAuditList(dir string) ([]Item, error) {
	fileInfos, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	auditList := []Item{}
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
		auditList = append(auditList, Item{
			ID:      fi.Name(),
			Time:    t.Format(time.RFC3339),
			Command: cmd,
		})
	}

	sort.Slice(auditList, func(i, j int) bool {
		return auditList[i].Time < auditList[j].Time
	})

	return auditList, nil
}

// OutputAuditLog outputs audit log.
func OutputAuditLog(dir, fileSuffix string, data []byte) error {
	auditID := base52.Encode(time.Now().UnixNano() + rand.Int63n(1000))
	if customID := os.Getenv(EnvNameAuditID); customID != "" {
		auditID = fmt.Sprintf("%s_%s", auditID, customID)
	}
	if fileSuffix != "" {
		auditID = fmt.Sprintf("%s_%s", auditID, fileSuffix)
	}

	fname := filepath.Join(dir, auditID)
	f, err := os.Create(fname)
	if err != nil {
		return errors.Annotate(err, "create audit log")
	}
	defer f.Close()

	args := encodeCommandArgs(os.Args)
	if _, err := f.Write([]byte(strings.Join(args, " ") + "\n")); err != nil {
		return errors.Annotate(err, "write audit log")
	}
	if _, err := f.Write(data); err != nil {
		return errors.Annotate(err, "write audit log")
	}
	return nil
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

// decodeAuditID decodes the auditID to unix timestamp
func decodeAuditID(auditID string) (time.Time, error) {
	tsID := auditID
	if strings.Contains(auditID, "_") {
		tsID = strings.Split(auditID, "_")[0]
	}
	ts, err := base52.Decode(tsID)
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

type deleteAuditLog struct {
	Files         []string  `json:"files"`
	Size          int64     `json:"size"`
	Count         int       `json:"count"`
	DelBeforeTime time.Time `json:"delete_before_time"` // audit logs before `DelBeforeTime` will be deleted
}

// DeleteAuditLog  cleanup audit log
func DeleteAuditLog(dir string, retainDays int, skipConfirm bool, displayMode string) error {
	if retainDays < 0 {
		return errors.Errorf("retainDays cannot be less than 0")
	}

	deleteLog := &deleteAuditLog{
		Files: []string{},
		Size:  0,
		Count: 0,
	}

	//  audit logs before `DelBeforeTime` will be deleted
	oneDayDuration, _ := time.ParseDuration("-24h")
	deleteLog.DelBeforeTime = time.Now().Add(oneDayDuration * time.Duration(retainDays))

	fileInfos, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, f := range fileInfos {
		if f.IsDir() {
			continue
		}
		t, err := decodeAuditID(f.Name())
		if err != nil {
			continue
		}
		if t.Before(deleteLog.DelBeforeTime) {
			info, err := f.Info()
			if err != nil {
				continue
			}
			deleteLog.Size += info.Size()
			deleteLog.Count++
			deleteLog.Files = append(deleteLog.Files, filepath.Join(dir, f.Name()))
		}
	}

	// output format json
	if displayMode == "json" {
		data, err := json.Marshal(struct {
			*deleteAuditLog `json:"deleted_logs"`
		}{deleteLog})

		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else {
		// print table
		fmt.Printf("Audit logs before %s will be deleted!\nFiles to be %s are:\n %s\nTotal count: %d \nTotal size: %s\n",
			color.HiYellowString(deleteLog.DelBeforeTime.Format("2006-01-02T15:04:05")),
			color.HiYellowString("deleted"),
			strings.Join(deleteLog.Files, "\n "),
			deleteLog.Count,
			readableSize(deleteLog.Size),
		)

		if !skipConfirm {
			if err := tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]:"); err != nil {
				return err
			}
		}
	}

	for _, f := range deleteLog.Files {
		if err := os.Remove(f); err != nil {
			return err
		}
	}

	if displayMode != "json" {
		fmt.Println("clean audit log successfully")
	}

	return nil
}

func readableSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}
