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

package spec

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"github.com/prometheus/common/expfmt"
	"go.etcd.io/etcd/client/pkg/v3/transport"
)

var tidbSpec *SpecManager

// GetSpecManager return the spec manager of tidb cluster.
func GetSpecManager() *SpecManager {
	if !initialized {
		panic("must Initialize profile first")
	}
	return tidbSpec
}

// ClusterMeta is the specification of generic cluster metadata
type ClusterMeta struct {
	User    string `yaml:"user"`         // the user to run and manage cluster on remote
	Version string `yaml:"tidb_version"` // the version of TiDB cluster
	// EnableFirewall bool   `yaml:"firewall"`
	OpsVer string `yaml:"last_ops_ver,omitempty"` // the version of ourself that updated the meta last time

	Topology *Specification `yaml:"topology"`
}

var _ UpgradableMetadata = &ClusterMeta{}

// SetVersion implement UpgradableMetadata interface.
func (m *ClusterMeta) SetVersion(s string) {
	m.Version = s
}

// SetUser implement UpgradableMetadata interface.
func (m *ClusterMeta) SetUser(s string) {
	m.User = s
}

// GetTopology implement Metadata interface.
func (m *ClusterMeta) GetTopology() Topology {
	return m.Topology
}

// SetTopology implement Metadata interface.
func (m *ClusterMeta) SetTopology(topo Topology) {
	tidbTopo, ok := topo.(*Specification)
	if !ok {
		panic(fmt.Sprintln("wrong type: ", reflect.TypeOf(topo)))
	}

	m.Topology = tidbTopo
}

// GetBaseMeta implements Metadata interface.
func (m *ClusterMeta) GetBaseMeta() *BaseMeta {
	return &BaseMeta{
		Version: m.Version,
		User:    m.User,
		OpsVer:  &m.OpsVer,
	}
}

// AuditDir return the directory for saving audit log.
func AuditDir() string {
	return filepath.Join(profileDir, TiUPAuditDir)
}

// SaveClusterMeta saves the cluster meta information to profile directory
func SaveClusterMeta(clusterName string, cmeta *ClusterMeta) error {
	// set the cmd version
	cmeta.OpsVer = version.NewTiUPVersion().String()
	return GetSpecManager().SaveMeta(clusterName, cmeta)
}

// ClusterMetadata tries to read the metadata of a cluster from file
func ClusterMetadata(clusterName string) (*ClusterMeta, error) {
	var cm ClusterMeta
	err := GetSpecManager().Metadata(clusterName, &cm)
	if err != nil {
		// Return the value of cm even on error, to make sure the caller can get the data
		// we read, if there's any.
		// This is necessary when, either by manual editing of meta.yaml file, by not fully
		// validated `edit-config`, or by some unexpected operations from a broken legacy
		// release, we could provide max possibility that operations like `display`, `scale`
		// and `destroy` are still (more or less) working, by ignoring certain errors.
		return &cm, err
	}

	return &cm, nil
}

// LoadClientCert read and load the client cert key pair and CA cert
func LoadClientCert(dir string) (*tls.Config, error) {
	return transport.TLSInfo{
		TrustedCAFile: filepath.Join(dir, TLSCACert),
		CertFile:      filepath.Join(dir, TLSClientCert),
		KeyFile:       filepath.Join(dir, TLSClientKey),
	}.ClientConfig()
}

// statusByHost queries current status of the instance by http status api.
func statusByHost(host string, port int, path string, timeout time.Duration, tlsCfg *tls.Config) string {
	if timeout < time.Second {
		timeout = statusQueryTimeout
	}

	client := utils.NewHTTPClient(timeout, tlsCfg)

	scheme := "http"
	if tlsCfg != nil {
		scheme = "https"
	}
	if path == "" {
		path = "/"
	}
	url := fmt.Sprintf("%s://%s%s", scheme, utils.JoinHostPort(host, port), path)

	// body doesn't have any status section needed
	body, err := client.Get(context.TODO(), url)
	if err != nil || body == nil {
		return "Down"
	}
	return "Up"
}

// UptimeByHost queries current uptime of the instance by http Prometheus metric api.
func UptimeByHost(host string, port int, timeout time.Duration, tlsCfg *tls.Config) time.Duration {
	if timeout < time.Second {
		timeout = statusQueryTimeout
	}

	scheme := "http"
	if tlsCfg != nil {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/metrics", scheme, utils.JoinHostPort(host, port))

	client := utils.NewHTTPClient(timeout, tlsCfg)

	body, err := client.Get(context.TODO(), url)
	if err != nil || body == nil {
		return 0
	}

	var parser expfmt.TextParser
	reader := bytes.NewReader(body)
	mf, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		return 0
	}

	now := time.Now()
	for k, v := range mf {
		if k == promMetricStartTimeSeconds {
			ms := v.GetMetric()
			if len(ms) >= 1 {
				startTime := ms[0].Gauge.GetValue()
				return now.Sub(time.Unix(int64(startTime), 0))
			}
			return 0
		}
	}

	return 0
}

// Abs returns the absolute path
func Abs(user, path string) string {
	// trim whitespaces before joining
	user = strings.TrimSpace(user)
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		path = filepath.Join("/home", user, path)
	}
	return filepath.Clean(path)
}

// MultiDirAbs returns the absolute path for multi-dir separated by comma
func MultiDirAbs(user, paths string) []string {
	var dirs []string
	for _, path := range strings.Split(paths, ",") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		dirs = append(dirs, Abs(user, path))
	}
	return dirs
}

// PackagePath return the tar bar path
func PackagePath(comp string, version string, os string, arch string) string {
	fileName := fmt.Sprintf("%s-%s-%s-%s.tar.gz", comp, version, os, arch)
	return ProfilePath(TiUPPackageCacheDir, fileName)
}

// GetDMMasterPackageName return package name of the first DMMaster instance
func GetDMMasterPackageName(topo Topology) string {
	for _, c := range topo.ComponentsByStartOrder() {
		if c.Name() == ComponentDMMaster {
			instances := c.Instances()
			if len(instances) > 0 {
				return instances[0].ComponentSource()
			}
		}
	}
	return ComponentDMMaster
}
