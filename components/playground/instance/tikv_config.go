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

package instance

import (
	"io"
)

const config = `
# log-level = "info"
# log-file = ""
# log-rotation-timespan = "24h"
# log-rotation-size = "300MB"
# refresh-config-interval = "30s"

[readpool]
# unify-read-pool = true

[readpool.unified]
# min-thread-count = 1
# max-thread-count = 8
# stack-size = "10MB"
# max-tasks-per-worker = 2000

[readpool.storage]
# high-concurrency = 4
# normal-concurrency = 4
# low-concurrency = 4
# max-tasks-per-worker-high = 2000
# max-tasks-per-worker-normal = 2000
# max-tasks-per-worker-low = 2000
# stack-size = "10MB"

[readpool.coprocessor]
# high-concurrency = 8
# normal-concurrency = 8
# low-concurrency = 8
# max-tasks-per-worker-high = 2000
# max-tasks-per-worker-normal = 2000
# max-tasks-per-worker-low = 2000
# stack-size = "10MB"

[server]
# addr = "127.0.0.1:20160"
# advertise-addr = ""
# status-addr = "127.0.0.1:20180"
# status-thread-pool-size = 1
# grpc-compression-type = "none"
# grpc-concurrency = 4
# grpc-concurrent-stream = 1024
# grpc-memory-pool-quota = "32G"
# grpc-raft-conn-num = 1
# grpc-stream-initial-window-size = "2MB"
# grpc-keepalive-time = "10s"
# grpc-keepalive-timeout = "3s"
# concurrent-send-snap-limit = 32
# concurrent-recv-snap-limit = 32
# end-point-recursion-limit = 1000
# end-point-request-max-handle-duration = "60s"
# snap-max-write-bytes-per-sec = "100MB"
# enable-request-batch = true
# request-batch-enable-cross-command = true
# request-batch-wait-duration = "1ms"
# labels = {}

[storage]
# data-dir = "/tmp/tikv/store"
# scheduler-concurrency = 2048000
# scheduler-worker-pool-size = 4
# scheduler-pending-write-threshold = "100MB"

[storage.block-cache]
# shared = true
# capacity = "1GB"

[pd]
# endpoints = []
# retry-interval = "300ms"
# retry-log-every = 10
# retry-max-count = -1

[raftstore]
# sync-log = true
# prevote = true
# raftdb-path = ""
# capacity = 0
# notify-capacity = 40960
# messages-per-tick = 4096
# pd-heartbeat-tick-interval = "60s"
# pd-store-heartbeat-tick-interval = "10s"
# region-split-check-diff = "6MB"
# split-region-check-tick-interval = "10s"
# raft-entry-max-size = "8MB"
# raft-log-gc-tick-interval = "10s"
# raft-log-gc-threshold = 50
# raft-log-gc-count-limit = 72000
# raft-log-gc-size-limit = "72MB"
# max-peer-down-duration = "5m"
# region-compact-check-interval = "5m"
# region-compact-check-step = 100
# region-compact-min-tombstones = 10000
# region-compact-tombstones-percent = 30
# lock-cf-compact-interval = "10m"
# lock-cf-compact-bytes-threshold = "256MB"
# consistency-check-interval = 0
# clean-stale-peer-delay = "10m"
# cleanup-import-sst-interval = "10m"
# apply-pool-size = 2
# store-pool-size = 2

[coprocessor]
# split-region-on-table = false
# batch-split-limit = 10
# region-max-size = "144MB"
# region-split-size = "96MB"
# region-max-keys = 1440000
# region-split-keys = 960000

[rocksdb]
# max-background-jobs = 8
# max-sub-compactions = 3
max-open-files = 256
# max-manifest-file-size = "128MB"
# create-if-missing = true
# wal-recovery-mode = 2
# wal-dir = "/tmp/tikv/store"
# wal-ttl-seconds = 0
# wal-size-limit = 0
# max-total-wal-size = "4GB"
# enable-statistics = true
# stats-dump-period = "10m"
# compaction-readahead-size = 0
# writable-file-max-buffer-size = "1MB"
# use-direct-io-for-flush-and-compaction = false
# rate-bytes-per-sec = 0
# rate-limiter-mode = 2
# auto-tuned = false
# enable-pipelined-write = true
# bytes-per-sync = "1MB"
# wal-bytes-per-sync = "512KB"
# info-log-max-size = "1GB"
# info-log-roll-time = "0"
# info-log-keep-log-file-num = 10
# info-log-dir = ""

[rocksdb.titan]
# enabled = false
# default: 4
# max-background-gc = 4

[rocksdb.defaultcf]
# compression-per-level = ["no", "no", "lz4", "lz4", "lz4", "zstd", "zstd"]
# block-size = "64KB"
# bloom-filter-bits-per-key = 10
# block-based-bloom-filter = false
# level0-file-num-compaction-trigger = 4
# level0-slowdown-writes-trigger = 20
# level0-stop-writes-trigger = 36
# write-buffer-size = "128MB"
# max-write-buffer-number = 5
# min-write-buffer-number-to-merge = 1
# max-bytes-for-level-base = "512MB"
# target-file-size-base = "8MB"
# max-compaction-bytes = "2GB"
# compaction-pri = 3
# cache-index-and-filter-blocks = true
# pin-l0-filter-and-index-blocks = true
# read-amp-bytes-per-bit = 0
# dynamic-level-bytes = true
# optimize-filters-for-hits = true

[rocksdb.defaultcf.titan]
# min-blob-size = "1KB"
# default: lz4
# blob-file-compression = "lz4"
# default: 0
# blob-cache-size = "0GB"
# default: 0.5
# discardable-ratio = 0.5
# default: normal
# blob-run-mode = "normal"

[rocksdb.writecf]
# compression-per-level = ["no", "no", "lz4", "lz4", "lz4", "zstd", "zstd"]
# block-size = "64KB"
# write-buffer-size = "128MB"
# max-write-buffer-number = 5
# min-write-buffer-number-to-merge = 1
# max-bytes-for-level-base = "512MB"
# target-file-size-base = "8MB"
# level0-file-num-compaction-trigger = 4
# level0-slowdown-writes-trigger = 20
# level0-stop-writes-trigger = 36
# cache-index-and-filter-blocks = true
# pin-l0-filter-and-index-blocks = true
# compaction-pri = 3
# read-amp-bytes-per-bit = 0
# dynamic-level-bytes = true
# optimize-filters-for-hits = false

[rocksdb.lockcf]
# compression-per-level = ["no", "no", "no", "no", "no", "no", "no"]
# block-size = "16KB"
# write-buffer-size = "32MB"
# max-write-buffer-number = 5
# min-write-buffer-number-to-merge = 1
# max-bytes-for-level-base = "128MB"
# target-file-size-base = "8MB"
# level0-file-num-compaction-trigger = 1
# level0-slowdown-writes-trigger = 20
# level0-stop-writes-trigger = 36
# cache-index-and-filter-blocks = true
# pin-l0-filter-and-index-blocks = true
# compaction-pri = 0
# read-amp-bytes-per-bit = 0
# dynamic-level-bytes = true
# optimize-filters-for-hits = false

[raftdb]
# max-background-jobs = 4
# max-sub-compactions = 2
max-open-files = 256
# max-manifest-file-size = "20MB"
# create-if-missing = true
# enable-statistics = true
# stats-dump-period = "10m"
# compaction-readahead-size = 0
# writable-file-max-buffer-size = "1MB"
# use-direct-io-for-flush-and-compaction = false
# enable-pipelined-write = true
# allow-concurrent-memtable-write = false
# bytes-per-sync = "1MB"
# wal-bytes-per-sync = "512KB"
# info-log-max-size = "1GB"
# info-log-roll-time = "0"
# info-log-keep-log-file-num = 10
# info-log-dir = ""
# optimize-filters-for-hits = true

[raftdb.defaultcf]
# compression-per-level = ["no", "no", "lz4", "lz4", "lz4", "zstd", "zstd"]
# block-size = "64KB"
# write-buffer-size = "128MB"
# max-write-buffer-number = 5
# min-write-buffer-number-to-merge = 1
# max-bytes-for-level-base = "512MB"
# target-file-size-base = "8MB"
# level0-file-num-compaction-trigger = 4
# level0-slowdown-writes-trigger = 20
# level0-stop-writes-trigger = 36
# cache-index-and-filter-blocks = true
# pin-l0-filter-and-index-blocks = true
# compaction-pri = 0
# read-amp-bytes-per-bit = 0
# dynamic-level-bytes = true
# optimize-filters-for-hits = true

[security]
# ca-path = ""
# cert-path = ""
# key-path = ""

[import]
# num-threads = 8
# stream-channel-window = 128

[pessimistic-txn]
# enabled = true
# wait-for-lock-timeout = 1000
# wake-up-delay-duration = 20

[gc]
# batch-keys = 512
# max-write-bytes-per-sec = "0"
`

func writeConfig(w io.Writer) error {
	_, err := w.Write([]byte(config))
	return err
}
