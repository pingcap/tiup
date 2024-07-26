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
	"crypto/tls"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/checkpoint"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
	"go.uber.org/zap"
)

var (
	// register checkpoint for upgrade operation
	upgradePoint       = checkpoint.Register(checkpoint.Field("instance", reflect.DeepEqual))
	increaseLimitPoint = checkpoint.Register()
)

// Upgrade the cluster. (actually, it's rolling restart)
func Upgrade(
	ctx context.Context,
	topo spec.Topology,
	options Options,
	tlsCfg *tls.Config,
	currentVersion string,
	targetVersion string,
) error {
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := topo.ComponentsByUpdateOrder(currentVersion)
	components = FilterComponent(components, roleFilter)
	logger := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)
	systemdMode := string(topo.BaseTopo().GlobalOptions.SystemdMode)

	noAgentHosts := set.NewStringSet()
	uniqueHosts := set.NewStringSet()

	var cdcOpenAPIClient *api.CDCOpenAPIClient // client for cdc openapi, only used when upgrade cdc

	for _, component := range components {
		instances := FilterInstance(component.Instances(), nodeFilter)
		if len(instances) < 1 {
			continue
		}
		logger.Infof("Upgrading component %s", component.Name())

		// perform pre-upgrade actions of component
		var origLeaderScheduleLimit int
		var origRegionScheduleLimit int
		var err error

		var tidbClient *api.TiDBClient
		var pdEndpoints []string
		forcePDEndpoints := os.Getenv(EnvNamePDEndpointOverwrite) // custom set PD endpoint list

		switch component.Name() {
		case spec.ComponentTiKV:
			if forcePDEndpoints != "" {
				pdEndpoints = strings.Split(forcePDEndpoints, ",")
				logger.Warnf("%s is set, using %s as PD endpoints", EnvNamePDEndpointOverwrite, pdEndpoints)
			} else {
				pdEndpoints = topo.(*spec.Specification).GetPDListWithManageHost()
			}
			pdClient := api.NewPDClient(ctx, pdEndpoints, 10*time.Second, tlsCfg)
			origLeaderScheduleLimit, origRegionScheduleLimit, err = increaseScheduleLimit(ctx, pdClient)
			if err != nil {
				// the config modifying error should be able to be safely ignored, as it will
				// be processed with current settings anyway.
				logger.Warnf("failed increasing schedule limit: %s, ignore", err)
			} else {
				defer func() {
					upgErr := decreaseScheduleLimit(pdClient, origLeaderScheduleLimit, origRegionScheduleLimit)
					if upgErr != nil {
						logger.Warnf(
							"failed decreasing schedule limit (original values should be: %s, %s), please check if their current values are reasonable: %s",
							fmt.Sprintf("leader-schedule-limit=%d", origLeaderScheduleLimit),
							fmt.Sprintf("region-schedule-limit=%d", origRegionScheduleLimit),
							upgErr,
						)
					}
				}()
			}
		case spec.ComponentTiDB:
			dbs := topo.(*spec.Specification).TiDBServers
			endpoints := []string{}
			for _, db := range dbs {
				endpoints = append(endpoints, utils.JoinHostPort(db.GetManageHost(), db.StatusPort))
			}

			if currentVersion != targetVersion && tidbver.TiDBSupportUpgradeAPI(currentVersion) && tidbver.TiDBSupportUpgradeAPI(targetVersion) {
				tidbClient = api.NewTiDBClient(ctx, endpoints, 10*time.Second, tlsCfg)
				err = tidbClient.StartUpgrade()
				if err != nil {
					return err
				}
			}

		default:
			// do nothing, kept for future usage with other components
		}

		// some instances are upgraded after others
		deferInstances := make([]spec.Instance, 0)

		for _, instance := range instances {
			// monitors
			uniqueHosts.Insert(instance.GetManageHost())
			if instance.IgnoreMonitorAgent() {
				noAgentHosts.Insert(instance.GetManageHost())
			}

			// Usage within the switch statement
			switch component.Name() {
			case spec.ComponentPD, spec.ComponentTSO, spec.ComponentScheduling:
				// defer PD related leader/primary to be upgraded after others
				isLeader, err := checkAndDeferPDLeader(ctx, topo, int(options.APITimeout), tlsCfg, instance)
				if err != nil {
					logger.Warnf("cannot found pd related leader/primary, ignore: %s, instance: %s", err, instance.ID())
					return err
				}
				if isLeader {
					deferInstances = append(deferInstances, instance)
					logger.Debugf("Upgrading deferred instance %s...", instance.ID())
					continue
				}
			case spec.ComponentCDC:
				ins := instance.(*spec.CDCInstance)
				address := ins.GetAddr()
				if !tidbver.TiCDCSupportRollingUpgrade(currentVersion) {
					logger.Debugf("rolling upgrade cdc not supported, upgrade by force, "+
						"addr: %s, version: %s", address, currentVersion)
					options.Force = true
					if err := upgradeInstance(ctx, topo, instance, options, tlsCfg); err != nil {
						options.Force = false
						return err
					}
					options.Force = false
					continue
				}

				// during the upgrade process, endpoint addresses should not change, so only new the client once.
				if cdcOpenAPIClient == nil {
					cdcOpenAPIClient = api.NewCDCOpenAPIClient(ctx, topo.(*spec.Specification).GetCDCListWithManageHost(), 5*time.Second, tlsCfg)
				}

				capture, err := cdcOpenAPIClient.GetCaptureByAddr(address)
				if err != nil {
					// After the previous status check, we know that the cdc instance should be `Up`, but know it cannot be found by address
					// perhaps since the specified version of cdc does not support open api, or the instance just crashed right away
					logger.Debugf("upgrade cdc, cannot found the capture by address: %s", address)
					if err := upgradeInstance(ctx, topo, instance, options, tlsCfg); err != nil {
						return err
					}
					continue
				}

				if capture.IsOwner {
					deferInstances = append(deferInstances, instance)
					logger.Debugf("Deferred upgrading of TiCDC owner %s, captureID: %s, addr: %s", instance.ID(), capture.ID, address)
					continue
				}
			default:
				// do nothing, kept for future usage with other components
			}

			if err := upgradeInstance(ctx, topo, instance, options, tlsCfg); err != nil {
				return err
			}
		}

		// process deferred instances
		for _, instance := range deferInstances {
			logger.Debugf("Upgrading deferred instance %s...", instance.ID())
			if err := upgradeInstance(ctx, topo, instance, options, tlsCfg); err != nil {
				return err
			}
		}

		switch component.Name() {
		case spec.ComponentTiDB:
			if currentVersion != targetVersion && tidbver.TiDBSupportUpgradeAPI(currentVersion) && tidbver.TiDBSupportUpgradeAPI(targetVersion) {
				err = tidbClient.FinishUpgrade()
				if err != nil {
					return err
				}
			}

		default:
			// do nothing, kept for future usage with other components
		}
	}

	if topo.GetMonitoredOptions() == nil {
		return nil
	}

	return RestartMonitored(ctx, uniqueHosts.Slice(), noAgentHosts, topo.GetMonitoredOptions(), options.OptTimeout, systemdMode)
}

// checkAndDeferPDLeader checks the PD related leader/primary instance's status and defers its upgrade if necessary.
func checkAndDeferPDLeader(ctx context.Context, topo spec.Topology, apiTimeout int, tlsCfg *tls.Config, instance spec.Instance) (isLeader bool, err error) {
	switch instance.ComponentName() {
	case spec.ComponentPD:
		isLeader, err = instance.(*spec.PDInstance).IsLeader(ctx, topo, apiTimeout, tlsCfg)
	case spec.ComponentScheduling:
		isLeader, err = instance.(*spec.SchedulingInstance).IsPrimary(ctx, topo, tlsCfg)
	case spec.ComponentTSO:
		isLeader, err = instance.(*spec.TSOInstance).IsPrimary(ctx, topo, tlsCfg)
	}
	if err != nil {
		return false, err
	}
	return isLeader, nil
}

func upgradeInstance(
	ctx context.Context,
	topo spec.Topology,
	instance spec.Instance,
	options Options,
	tlsCfg *tls.Config,
) (err error) {
	// insert checkpoint
	point := checkpoint.Acquire(ctx, upgradePoint, map[string]any{"instance": instance.ID()})
	defer func() {
		point.Release(err, zap.String("instance", instance.ID()))
	}()

	if point.Hit() != nil {
		return nil
	}

	var rollingInstance spec.RollingUpdateInstance
	var isRollingInstance bool

	if !options.Force {
		rollingInstance, isRollingInstance = instance.(spec.RollingUpdateInstance)
	}

	err = executeSSHCommand(ctx, "Executing pre-upgrade command", instance.GetManageHost(), options.SSHCustomScripts.BeforeRestartInstance.Command())
	if err != nil {
		return err
	}

	if isRollingInstance {
		err := rollingInstance.PreRestart(ctx, topo, int(options.APITimeout), tlsCfg)
		if err != nil && !options.Force {
			return err
		}
	}
	systemdMode := string(topo.BaseTopo().GlobalOptions.SystemdMode)
	if err := restartInstance(ctx, instance, options.OptTimeout, tlsCfg, systemdMode); err != nil && !options.Force {
		return err
	}

	if isRollingInstance {
		err := rollingInstance.PostRestart(ctx, topo, tlsCfg)
		if err != nil && !options.Force {
			return err
		}
	}

	err = executeSSHCommand(ctx, "Executing post-upgrade command", instance.GetManageHost(), options.SSHCustomScripts.AfterRestartInstance.Command())
	if err != nil {
		return err
	}

	return nil
}

// Addr returns the address of the instance.
func Addr(ins spec.Instance) string {
	if ins.GetPort() == 0 || ins.GetPort() == 80 {
		panic(ins)
	}

	return utils.JoinHostPort(ins.GetManageHost(), ins.GetPort())
}

var (
	leaderScheduleLimitOffset = 32
	regionScheduleLimitOffset = 512
	// storeLimitOffset             = 512
	leaderScheduleLimitThreshold = 64
	regionScheduleLimitThreshold = 1024
	// storeLimitThreshold          = 1024
)

// increaseScheduleLimit increases the schedule limit of leader and region for faster
// rebalancing during the rolling restart / upgrade process
func increaseScheduleLimit(ctx context.Context, pc *api.PDClient) (
	currLeaderScheduleLimit int,
	currRegionScheduleLimit int,
	err error) {
	// insert checkpoint
	point := checkpoint.Acquire(ctx, increaseLimitPoint, map[string]any{})
	defer func() {
		point.Release(err,
			zap.Int("currLeaderScheduleLimit", currLeaderScheduleLimit),
			zap.Int("currRegionScheduleLimit", currRegionScheduleLimit),
		)
	}()

	if data := point.Hit(); data != nil {
		currLeaderScheduleLimit = int(data["currLeaderScheduleLimit"].(float64))
		currRegionScheduleLimit = int(data["currRegionScheduleLimit"].(float64))
		return
	}

	// query current values
	cfg, err := pc.GetConfig()
	if err != nil {
		return
	}
	val, ok := cfg["schedule.leader-schedule-limit"].(float64)
	if !ok {
		return currLeaderScheduleLimit, currRegionScheduleLimit, perrs.New("cannot get current leader-schedule-limit")
	}
	currLeaderScheduleLimit = int(val)
	val, ok = cfg["schedule.region-schedule-limit"].(float64)
	if !ok {
		return currLeaderScheduleLimit, currRegionScheduleLimit, perrs.New("cannot get current region-schedule-limit")
	}
	currRegionScheduleLimit = int(val)

	// increase values
	if currLeaderScheduleLimit < leaderScheduleLimitThreshold {
		newLimit := currLeaderScheduleLimit + leaderScheduleLimitOffset
		if newLimit > leaderScheduleLimitThreshold {
			newLimit = leaderScheduleLimitThreshold
		}
		if err := pc.SetReplicationConfig("leader-schedule-limit", newLimit); err != nil {
			return currLeaderScheduleLimit, currRegionScheduleLimit, err
		}
	}
	if currRegionScheduleLimit < regionScheduleLimitThreshold {
		newLimit := currRegionScheduleLimit + regionScheduleLimitOffset
		if newLimit > regionScheduleLimitThreshold {
			newLimit = regionScheduleLimitThreshold
		}
		if err := pc.SetReplicationConfig("region-schedule-limit", newLimit); err != nil {
			// try to revert leader scheduler limit by our best effort, does not make sense
			// to handle this error again
			_ = pc.SetReplicationConfig("leader-schedule-limit", currLeaderScheduleLimit)
			return currLeaderScheduleLimit, currRegionScheduleLimit, err
		}
	}

	return
}

// decreaseScheduleLimit tries to set the schedule limit back to it's original with
// the same offset value as increaseScheduleLimit added, with some sanity checks
func decreaseScheduleLimit(pc *api.PDClient, origLeaderScheduleLimit, origRegionScheduleLimit int) error {
	if err := pc.SetReplicationConfig("leader-schedule-limit", origLeaderScheduleLimit); err != nil {
		return err
	}
	return pc.SetReplicationConfig("region-schedule-limit", origRegionScheduleLimit)
}
