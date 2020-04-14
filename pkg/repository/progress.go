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

package repository

import (
	"github.com/cheggaaa/pb"
)

// DisableProgress implement the DownloadProgress interface and disable download progress
type DisableProgress struct{}

// SetTotal implement the DownloadProgress interface
func (d DisableProgress) SetTotal(size int64) {}

// SetCurrent implement the DownloadProgress interface
func (d DisableProgress) SetCurrent(size int64) {}

// Finish implement the DownloadProgress interface
func (d DisableProgress) Finish() {}

// ProgressBar implement the DownloadProgress interface with download progress
type ProgressBar struct {
	bar *pb.ProgressBar
}

// SetTotal implement the DownloadProgress interface
func (p *ProgressBar) SetTotal(size int64) {
	p.bar = pb.Start64(size)
	p.bar.Set(pb.Bytes, true)
	p.bar.SetTemplateString(`{{counters . }} {{percent . }} {{speed . }}`)
}

// SetCurrent implement the DownloadProgress interface
func (p *ProgressBar) SetCurrent(size int64) {
	p.bar.SetCurrent(size)
}

// Finish implement the DownloadProgress interface
func (p *ProgressBar) Finish() {
	p.bar.Finish()
}
