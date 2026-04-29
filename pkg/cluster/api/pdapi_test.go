// Copyright 2026 PingCAP, Inc.
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

package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pingcap/kvproto/pkg/metapb"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestDelStorePhysicallyDestroyedQuery(t *testing.T) {
	tests := []struct {
		name                   string
		delStore               func(*PDClient, string, *utils.RetryOption) error
		expectedDeleteRawQuery string
	}{
		{
			name:                   "regular delete",
			delStore:               (*PDClient).DelStore,
			expectedDeleteRawQuery: "",
		},
		{
			name:                   "physically destroyed delete",
			delStore:               (*PDClient).DelStorePhysicallyDestroyed,
			expectedDeleteRawQuery: "force=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var deleted atomic.Bool
			var deleteRawQuery string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/pd/api/v1/stores":
					state := metapb.StoreState_Up
					if deleted.Load() {
						state = metapb.StoreState_Offline
					}
					fmt.Fprintf(w, `{"count":1,"stores":[{"store":{"id":42,"address":"store-1:20160","state":%d}}]}`, state)
				case r.Method == http.MethodDelete && r.URL.Path == "/pd/api/v1/store/42":
					deleteRawQuery = r.URL.RawQuery
					deleted.Store(true)
					fmt.Fprint(w, `"The store is set as Offline."`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			logger := logprinter.NewLogger("")
			logger.SetStdout(io.Discard)
			logger.SetStderr(io.Discard)
			ctx := context.WithValue(context.Background(), logprinter.ContextKeyLogger, logger)
			client := NewPDClient(ctx, []string{strings.TrimPrefix(server.URL, "http://")}, time.Second, nil)

			err := tt.delStore(client, "store-1:20160", &utils.RetryOption{
				Delay:   time.Millisecond,
				Timeout: time.Second,
			})

			require.NoError(t, err)
			require.Equal(t, tt.expectedDeleteRawQuery, deleteRawQuery)
		})
	}
}
