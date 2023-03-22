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

package operator

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"sync"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/checkpoint"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/telemetry"
	"github.com/pingcap/tiup/pkg/version"
	"golang.org/x/sync/errgroup"
)

// GetNodeInfo the node info in topology.
func GetNodeInfo(
	ctx context.Context,
	topo spec.Topology,
) (nodes []*telemetry.NodeInfo, err error) {
	ver := version.NewTiUPVersion().String()
	dir := "/tmp/_cluster"

	// Download cluster binary
	errg, _ := errgroup.WithContext(ctx)
	foundArchs := make(map[string]struct{})
	topo.IterInstance(func(inst spec.Instance) {
		arch := fmt.Sprintf("%s-%s", inst.OS(), inst.Arch())
		if _, ok := foundArchs[arch]; !ok {
			inst := inst
			errg.Go(func() error {
				return Download("cluster", inst.OS(), inst.Arch(), ver)
			})
		}
		foundArchs[arch] = struct{}{}
	})

	err = errg.Wait()
	if err != nil {
		return nil, err
	}

	// Copy and get info per host.
	errg, _ = errgroup.WithContext(ctx)
	var nodesMu sync.Mutex
	foundHosts := make(map[string]struct{})
	topo.IterInstance(func(inst spec.Instance) {
		host := inst.GetManageHost()

		if _, ok := foundHosts[host]; ok {
			return
		}
		foundHosts[host] = struct{}{}

		// the checkpoint part of context can't be shared between goroutines
		// since it's used to trace the stack, so we must create a new layer
		// of checkpoint context every time put it into a new goroutine.
		nctx := checkpoint.NewContext(ctx)
		errg.Go(func() error {
			exec := ctxt.GetInner(nctx).Get(host)

			// Copy component...
			_, _, err := exec.Execute(nctx, "mkdir -p "+filepath.Join(dir, "bin"), false)
			if err != nil {
				return err
			}

			resName := fmt.Sprintf("%s-%s", "cluster", ver)
			fileName := fmt.Sprintf("%s-%s-%s.tar.gz", resName, inst.OS(), inst.Arch())
			srcPath := spec.ProfilePath(spec.TiUPPackageCacheDir, fileName)

			dstDir := filepath.Join(dir, "bin")
			dstPath := filepath.Join(dstDir, path.Base(srcPath))
			err = exec.Transfer(nctx, srcPath, dstPath, false, 0, false)
			if err != nil {
				return err
			}

			// get node info by exec _telemetry node_info at remote instance.
			cmd := fmt.Sprintf(`tar --no-same-owner -zxf %s -C %s && rm %s`, dstPath, dstDir, dstPath)
			_, stderr, err := exec.Execute(nctx, cmd, false)
			if err != nil {
				return errors.Annotatef(err, "stderr: %s", string(stderr))
			}

			cmd = fmt.Sprintf("%s/cluster _telemetry node_info", dstDir)
			stdout, _, err := exec.Execute(nctx, cmd, false)
			if err == nil {
				nodeInfo, err := telemetry.NodeInfoFromText(string(stdout))
				if err == nil {
					nodeInfo.NodeId = telemetry.SaltedHash(host)
					nodesMu.Lock()
					nodes = append(nodes, nodeInfo)
					nodesMu.Unlock()
				}
			}
			return nil
		})
	})

	err = errg.Wait()
	if err != nil {
		return nil, err
	}

	return
}
