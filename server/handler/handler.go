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

package handler

import (
	"context"

	"github.com/pingcap/fn"
)

type errorMessage struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func init() {
	fn.SetErrorEncoder(func(ctx context.Context, err error) interface{} {
		if e, ok := err.(statusError); ok {
			return &errorMessage{
				Status:  e.Status(),
				Message: e.Error(),
			}
		}
		return &errorMessage{
			Status:  "UNKNOWN_ERROR",
			Message: "make sure your request is valid",
		}
	})
}
