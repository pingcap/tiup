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

package manager

import (
	"crypto/tls"

	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
)

var (
	operationInfo OperationInfo = OperationInfo{}
)

// OperationType represents the operation type
type OperationType string

const (
	operationDeploy   OperationType = "deploy"
	operationStart    OperationType = "start"
	operationStop     OperationType = "stop"
	operationScaleIn  OperationType = "scaleIn"
	operationScaleOut OperationType = "scaleOut"
	operationDestroy  OperationType = "destroy"
)

// OperationInfo records latest operation task and related info
type OperationInfo struct {
	operationType OperationType
	clusterName   string
	curTask       *task.Serial
	err           error
}

// OperationStatus represents the current deployment status
type OperationStatus struct {
	OperationType OperationType `json:"operation_type"`
	ClusterName   string        `json:"cluster_name"`
	TotalProgress int           `json:"total_progress"`
	Steps         []string      `json:"steps"`
	ErrMsg        string        `json:"err_msg"`
}

// GetOperationStatus returns the current operations status, including progress, steps, err message
func (m *Manager) GetOperationStatus() OperationStatus {
	operationStatus := OperationStatus{
		OperationType: operationInfo.operationType,
		ClusterName:   operationInfo.clusterName,
		Steps:         []string{},
	}
	if operationInfo.curTask != nil {
		if operationInfo.operationType == operationDeploy {
			steps, progress := operationInfo.curTask.ComputeProgress()
			operationStatus.TotalProgress = progress
			operationStatus.Steps = steps
		} else {
			operationStatus.TotalProgress = operationInfo.curTask.Progress
			operationStatus.Steps = []string{}
			operationStatus.Steps = append(operationStatus.Steps, operationInfo.curTask.Steps...)
			operationStatus.Steps = append(operationStatus.Steps, operationInfo.curTask.CurTaskSteps...)
		}
	}
	if operationInfo.err != nil {
		operationStatus.ErrMsg = operationInfo.err.Error()
	}
	return operationStatus
}

////////////////////////////////////////////////////////

// DoStartCluster start the cluster with specified name.
func (m *Manager) DoStartCluster(name string, options operator.Options, fn ...func(b *task.Builder, metadata spec.Metadata)) {
	operationInfo = OperationInfo{operationType: operationStart, clusterName: name}
	operationInfo.err = m.StartCluster(name, options, fn...)
}

// DoStopCluster stop the cluster.
func (m *Manager) DoStopCluster(clusterName string, options operator.Options) {
	operationInfo = OperationInfo{operationType: operationStop, clusterName: clusterName}
	operationInfo.err = m.StopCluster(clusterName, options)
}

// DoDeploy deploy the cluster
func (m *Manager) DoDeploy(
	clusterName string,
	clusterVersion string,
	topoFile string,
	opt DeployOptions,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	skipConfirm bool,
	gOpt operator.Options,
) {
	operationInfo = OperationInfo{operationType: operationDeploy, clusterName: clusterName}
	operationInfo.err = m.Deploy(
		clusterName,
		clusterVersion,
		topoFile,
		opt,
		afterDeploy,
		skipConfirm,
		gOpt,
	)
}

// DoDestroyCluster destroy the cluster.
func (m *Manager) DoDestroyCluster(clusterName string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) {
	operationInfo = OperationInfo{operationType: operationDestroy, clusterName: clusterName}
	operationInfo.err = m.DestroyCluster(
		clusterName,
		gOpt,
		destroyOpt,
		skipConfirm,
	)
}

// DoScaleIn the cluster.
func (m *Manager) DoScaleIn(
	clusterName string,
	skipConfirm bool,
	gOpt operator.Options,
	scale func(builer *task.Builder, metadata spec.Metadata, tlsCfg *tls.Config),
) {
	operationInfo = OperationInfo{operationType: operationScaleIn, clusterName: clusterName}
	operationInfo.err = m.ScaleIn(
		clusterName,
		skipConfirm,
		gOpt,
		scale,
	)
}

// DoScaleOut scale out the cluster.
func (m *Manager) DoScaleOut(
	clusterName string,
	topoFile string,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	final func(b *task.Builder, name string, meta spec.Metadata),
	opt ScaleOutOptions,
	skipConfirm bool,
	gOpt operator.Options,
) {
	operationInfo = OperationInfo{operationType: operationScaleOut, clusterName: clusterName}
	operationInfo.err = m.ScaleOut(
		clusterName,
		topoFile,
		afterDeploy,
		final,
		opt,
		skipConfirm,
		gOpt,
	)
}
