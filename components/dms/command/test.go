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

package command

/*
import (
	"context"
	"fmt"
	"time"

	meta2 "github.com/pingcap/tiup/pkg/dms/meta"

	dmpb "github.com/pingcap/dm/dm/pb"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/meta"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
)

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "_test <cluster-name>",
		Short:  "test toolkit",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}

			clusterName := args[0]
			if utils.IsNotExist(meta.ClusterPath(clusterName, meta.MetaFileName)) {
				return errors.Errorf("cannot start non-exists cluster %s", clusterName)
			}

			metadata, err := meta2.DMMetadata(clusterName)
			if err != nil {
				return err
			}

			switch args[1] {
			case "readable":
				return readable(metadata.Topology)
			default:
				fmt.Println("unknown command: ", args[1])
				return cmd.Help()
			}

		},
	}

	return cmd
}

func checkMasterOnline(addr string) error {
	bc := backoff.DefaultConfig
	bc.MaxDelay = 2 * time.Second
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithConnectParams(grpc.ConnectParams{Backoff: bc}))
	if err != nil {
		return err
	}
	cli := dmpb.NewMasterClient(conn)
	req := &dmpb.ShowDDLLocksRequest{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := cli.ShowDDLLocks(ctx, req)
	if err == nil && !resp.Result {
		return errors.Errorf("check master ddl locks failed: %s", resp.Msg)
	}
	return err
}

func checkWorkerOnline(addr string) error {
	bc := backoff.DefaultConfig
	bc.MaxDelay = 2 * time.Second
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithConnectParams(grpc.ConnectParams{Backoff: bc}))
	if err != nil {
		return err
	}
	cli := dmpb.NewWorkerClient(conn)
	req := &dmpb.QueryStatusRequest{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := cli.QueryStatus(ctx, req)
	if err == nil && !resp.Result {
		return errors.Errorf("check worker status failed: %s", resp.Msg)
	}
	return err
}

func readable(topo *meta2.DMSTopologySpecification) error {
	errg, _ := errgroup.WithContext(context.Background())

	for _, spec := range topo.Masters {
		spec := spec
		errg.Go(func() error {
			if err := checkMasterOnline(fmt.Sprintf("%s:%d", spec.Host, spec.Port)); err != nil {
				return err
			}

			fmt.Printf("read dm-master %s:%d success\n", spec.Host, spec.Port)
			return nil
		})
	}

	for _, spec := range topo.Workers {
		spec := spec
		errg.Go(func() error {
			if err := checkWorkerOnline(fmt.Sprintf("%s:%d", spec.Host, spec.Port)); err != nil {
				return err
			}

			fmt.Printf("read dm-master %s:%d success\n", spec.Host, spec.Port)
			return nil
		})
	}

	return errg.Wait()
}
*/
