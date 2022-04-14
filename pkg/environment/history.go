// Copyright 2022 PingCAP, Inc.
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

package environment

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pkg/errors"
)

const (
	// HistoryDir history save path
	HistoryDir          = "history"
	historyPrefix       = "tiup-history-"
	historySize   int64 = 1024 * 64 //  history file default size is 64k
)

// commandRow type of command history row
type historyRow struct {
	Date    time.Time `json:"time"`
	Command string    `json:"command"`
	Code    int       `json:"exit_code"`
}

// historyItem  record history row file item
type historyItem struct {
	path  string
	info  fs.FileInfo
	index int
}

// HistoryRecord record tiup exec cmd
func HistoryRecord(env *Environment, command []string, date time.Time, code int) error {
	if env == nil {
		return nil
	}

	historyPath := env.LocalPath(HistoryDir)
	if utils.IsNotExist(historyPath) {
		err := os.MkdirAll(historyPath, 0755)
		if err != nil {
			return err
		}
	}

	h := &historyRow{
		Command: strings.Join(command, " "),
		Date:    date,
		Code:    code,
	}

	return h.save(historyPath)
}

// save save commandRow to file
func (r *historyRow) save(dir string) error {
	rBytes, err := json.Marshal(r)
	if err != nil {
		return err
	}

	historyFile := getLatestHistoryFile(dir)

	f, err := os.OpenFile(historyFile.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(rBytes, []byte("\n")...))
	return err
}

// GetHistory get tiup history
func (env *Environment) GetHistory(count int, all bool) ([]*historyRow, error) {
	fList, err := getHistoryFileList(env.LocalPath(HistoryDir))
	if err != nil {
		return nil, err
	}
	rows := []*historyRow{}
	for _, f := range fList {
		rs, err := f.getHistory()
		if err != nil {
			return rows, err
		}
		if (len(rows)+len(rs)) > count && !all {
			i := len(rows) + len(rs) - count
			rows = append(rs[i:], rows...)
			break
		}

		rows = append(rs, rows...)
	}
	return rows, nil
}

// DeleteHistory delete history file
func (env *Environment) DeleteHistory(retainDays int, skipConfirm bool) error {
	if retainDays < 0 {
		return errors.Errorf("retainDays cannot be less than 0")
	}

	// history file before `DelBeforeTime` will be deleted
	oneDayDuration, _ := time.ParseDuration("-24h")
	delBeforeTime := time.Now().Add(oneDayDuration * time.Duration(retainDays))

	if !skipConfirm {
		fmt.Printf("History logs before %s will be %s!\n",
			color.HiYellowString(delBeforeTime.Format("2006-01-02T15:04:05")),
			color.HiYellowString("deleted"),
		)
		if err := tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]:"); err != nil {
			return err
		}
	}

	fList, err := getHistoryFileList(env.LocalPath(HistoryDir))
	if err != nil {
		return err
	}

	if len(fList) == 0 {
		return nil
	}

	for _, f := range fList {
		if f.info.ModTime().Before(delBeforeTime) {
			err := os.Remove(f.path)
			if err != nil {
				return err
			}
			continue
		}
	}
	return nil
}

// getHistory get tiup history execution row
func (i *historyItem) getHistory() ([]*historyRow, error) {
	rows := []*historyRow{}

	fi, err := os.Open(i.path)
	if err != nil {
		return rows, err
	}
	defer fi.Close()

	br := bufio.NewReader(fi)
	for {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		r := &historyRow{}
		// ignore
		err := json.Unmarshal(a, r)
		if err != nil {
			continue
		}
		rows = append(rows, r)
	}

	return rows, nil
}

// getHistoryFileList get the history file list
func getHistoryFileList(dir string) ([]historyItem, error) {
	fileInfos, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	hfileList := []historyItem{}
	for _, fi := range fileInfos {
		if fi.IsDir() {
			continue
		}

		// another suffix
		// ex: tiup-history-0.bak
		i, err := strconv.Atoi((strings.TrimPrefix(fi.Name(), historyPrefix)))
		if err != nil {
			continue
		}

		fInfo, _ := fi.Info()
		hfileList = append(hfileList, historyItem{
			path:  filepath.Join(dir, fi.Name()),
			index: i,
			info:  fInfo,
		})
	}

	sort.Slice(hfileList, func(i, j int) bool {
		return hfileList[i].index > hfileList[j].index
	})

	return hfileList, nil
}

// getLatestHistoryFile get the latest history file, use index 0 if it doesn't exist
func getLatestHistoryFile(dir string) (item historyItem) {
	fileList, err := getHistoryFileList(dir)
	// start from 0
	if len(fileList) == 0 || err != nil {
		item.index = 0
		item.path = filepath.Join(dir, fmt.Sprintf("%s%s", historyPrefix, strconv.Itoa(item.index)))
		return
	}

	latestItem := fileList[0]

	if latestItem.info.Size() >= historySize {
		item.index = latestItem.index + 1
		item.path = filepath.Join(dir, fmt.Sprintf("%s%s", historyPrefix, strconv.Itoa(item.index)))
	} else {
		item = latestItem
	}

	return
}
