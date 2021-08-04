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
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/utils"
)

var defaultURL = "https://telemetry.pingcap.com/api/v1/clusters/report"

// Telemetry control telemetry.
type Telemetry struct {
	url string
	cli *utils.HTTPClient
}

// NewTelemetry return a new Telemetry instance.
func NewTelemetry() *Telemetry {
	cli := utils.NewHTTPClient(time.Second*10, nil)

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

	if _, err = t.cli.Post(ctx, t.url, bytes.NewReader(dst)); err != nil {
		return errors.AddStack(err)
	}

	return nil
}
