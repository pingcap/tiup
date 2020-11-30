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

package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/pingcap/errors"
)

var defaultURL = "https://telemetry.pingcap.com/api/v1/clusters/report"

// Telemetry control telemetry.
type Telemetry struct {
	url string
	cli *http.Client
}

// NewTelemetry return a new Telemetry instance.
func NewTelemetry() *Telemetry {
	cli := new(http.Client)

	return &Telemetry{
		url: defaultURL,
		cli: cli,
	}
}

// Report report the msg right away.
func (t *Telemetry) Report(ctx context.Context, msg *Report) error {
	dst, err := json.Marshal(msg)
	if err != nil {
		return errors.AddStack(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.url, bytes.NewReader(dst))
	if err != nil {
		return errors.AddStack(err)
	}

	req.Header.Add("Content-Type", "application/json")
	resp, err := t.cli.Do(req)
	if err != nil {
		return errors.AddStack(err)
	}
	defer resp.Body.Close()

	code := resp.StatusCode
	if code < 200 || code >= 300 {
		return errors.Errorf("StatusCode: %d, Status: %s", resp.StatusCode, resp.Status)
	}

	return nil
}
