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
	"errors"
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	// AnsibleImportedConfigPath is the sub path where all imported configs are stored
	AnsibleImportedConfigPath = "ansible-imported-configs"
	// TempConfigPath is the sub path where generated temporary configs are stored
	TempConfigPath = "config-cache"
	// migrateLockName is the directory name of migrating lock
	migrateLockName = "tiup-migrate.lck"
)

// ErrorCheckConfig represent error occurred in config check stage
var ErrorCheckConfig = errors.New("check config failed")

// strKeyMap tries to convert `map[interface{}]interface{}` to `map[string]interface{}`
func strKeyMap(val interface{}) interface{} {
	m, ok := val.(map[interface{}]interface{})
	if ok {
		ret := map[string]interface{}{}
		for k, v := range m {
			kk, ok := k.(string)
			if !ok {
				return val
			}
			ret[kk] = strKeyMap(v)
		}
		return ret
	}

	rv := reflect.ValueOf(val)
	if rv.Kind() == reflect.Slice {
		var ret []interface{}
		for i := 0; i < rv.Len(); i++ {
			ret = append(ret, strKeyMap(rv.Index(i).Interface()))
		}
		return ret
	}

	return val
}

func foldKey(key string, val interface{}) (string, interface{}) {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) == 1 {
		return key, strKeyMap(val)
	}
	subKey, subVal := foldKey(parts[1], val)
	return parts[0], map[string]interface{}{
		subKey: strKeyMap(subVal),
	}
}

func patch(origin map[string]interface{}, key string, val interface{}) {
	origVal, found := origin[key]
	if !found {
		origin[key] = strKeyMap(val)
		return
	}
	origMap, lhsOk := origVal.(map[string]interface{})
	valMap, rhsOk := val.(map[string]interface{})
	if lhsOk && rhsOk {
		for k, v := range valMap {
			patch(origMap, k, v)
		}
	} else {
		// overwrite
		origin[key] = strKeyMap(val)
	}
}

// FoldMap convert single layer map to multi-layer
func FoldMap(ms map[string]interface{}) map[string]interface{} {
	// we flatten map first to deal with the case like:
	// a.b:
	//   c.d: xxx
	ms = FlattenMap(ms)
	result := map[string]interface{}{}
	for k, v := range ms {
		key, val := foldKey(k, v)
		patch(result, key, val)
	}
	return result
}

// FlattenMap convert mutil-layer map to single layer
func FlattenMap(ms map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for k, v := range ms {
		var sub map[string]interface{}

		if m, ok := v.(map[string]interface{}); ok {
			sub = FlattenMap(m)
		} else if m, ok := v.(map[interface{}]interface{}); ok {
			fixM := map[string]interface{}{}
			for k, v := range m {
				if sk, ok := k.(string); ok {
					fixM[sk] = v
				}
			}
			sub = FlattenMap(fixM)
		} else {
			result[k] = v
			continue
		}

		for sk, sv := range sub {
			result[k+"."+sk] = sv
		}
	}
	return result
}

func merge(orig map[string]interface{}, overwrites ...map[string]interface{}) (map[string]interface{}, error) {
	lhs := FoldMap(orig)
	for _, overwrite := range overwrites {
		rhs := FoldMap(overwrite)
		for k, v := range rhs {
			patch(lhs, k, v)
		}
	}
	return lhs, nil
}

// GetValueFromPath try to find the value by path recursively
func GetValueFromPath(m map[string]interface{}, p string) interface{} {
	ss := strings.Split(p, ".")

	searchMap := make(map[interface{}]interface{})
	m = FoldMap(m)
	for k, v := range m {
		searchMap[k] = v
	}

	return searchValue(searchMap, ss)
}

func searchValue(m map[interface{}]interface{}, ss []string) interface{} {
	l := len(ss)
	switch l {
	case 0:
		return m
	case 1:
		return m[ss[0]]
	}

	key := ss[0]
	if pm, ok := m[key].(map[interface{}]interface{}); ok {
		return searchValue(pm, ss[1:])
	} else if pm, ok := m[key].(map[string]interface{}); ok {
		searchMap := make(map[interface{}]interface{})
		for k, v := range pm {
			searchMap[k] = v
		}
		return searchValue(searchMap, ss[1:])
	}

	return nil
}

// Merge2Toml merge the config of global.
func Merge2Toml(comp string, global, overwrite map[string]interface{}) ([]byte, error) {
	return merge2Toml(comp, global, overwrite)
}

func merge2Toml(comp string, global, overwrite map[string]interface{}) ([]byte, error) {
	lhs, err := merge(global, overwrite)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBufferString(fmt.Sprintf(`# WARNING: This file is auto-generated. Do not edit! All your modification will be overwritten!
# You can use 'tiup cluster edit-config' and 'tiup cluster reload' to update the configuration
# All configuration items you want to change can be added to:
# server_configs:
#   %s:
#     aa.b1.c3: value
#     aa.b2.c4: value
`, comp))

	enc := toml.NewEncoder(buf)
	enc.Indent = ""
	err = enc.Encode(lhs)
	if err != nil {
		return nil, perrs.Trace(err)
	}
	return buf.Bytes(), nil
}

func mergeImported(importConfig []byte, specConfigs ...map[string]interface{}) (map[string]interface{}, error) {
	var configData map[string]interface{}
	if err := toml.Unmarshal(importConfig, &configData); err != nil {
		return nil, perrs.Trace(err)
	}

	// overwrite topology specifieced configs upon the imported configs
	lhs, err := merge(configData, specConfigs...)
	if err != nil {
		return nil, perrs.Trace(err)
	}
	return lhs, nil
}

// BindVersion map the cluster version to the third components binding version.
type BindVersion func(comp string, version string) (bindVersion string)

func checkConfig(e executor.Executor, componentName, clusterVersion, nodeOS, arch, config string, paths meta.DirPaths, bindVersion BindVersion) error {
	repo, err := clusterutil.NewRepository(nodeOS, arch)
	if err != nil {
		return perrs.Annotate(ErrorCheckConfig, err.Error())
	}

	ver := clusterVersion
	if bindVersion != nil {
		ver = bindVersion(componentName, clusterVersion)
	}

	entry, err := repo.ComponentBinEntry(componentName, ver)
	if err != nil {
		return perrs.Annotate(ErrorCheckConfig, err.Error())
	}

	binPath := path.Join(paths.Deploy, "bin", entry)
	// Skip old versions
	if !hasConfigCheckFlag(e, binPath) {
		return nil
	}

	// Hack tikv --pd flag
	extra := ""
	if componentName == ComponentTiKV {
		extra = `--pd=""`
	}

	configPath := path.Join(paths.Deploy, "conf", config)
	_, _, err = e.Execute(fmt.Sprintf("%s --config-check --config=%s %s", binPath, configPath, extra), false)
	if err != nil {
		return perrs.Annotate(ErrorCheckConfig, err.Error())
	}
	return nil
}

func hasConfigCheckFlag(e executor.Executor, binPath string) bool {
	stdout, stderr, _ := e.Execute(fmt.Sprintf("%s --help", binPath), false)
	return strings.Contains(string(stdout), "config-check") || strings.Contains(string(stderr), "config-check")
}

// HandleImportPathMigration tries to rename old configs file directory for imported clusters to the new name
func HandleImportPathMigration(clsName string) error {
	dirPath := ClusterPath(clsName)
	targetPath := path.Join(dirPath, AnsibleImportedConfigPath)
	_, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		log.Warnf("renaming '%s/config' to '%s'", dirPath, targetPath)

		if lckErr := utils.Retry(func() error {
			_, lckErr := os.Stat(path.Join(dirPath, migrateLockName))
			if os.IsNotExist(lckErr) {
				return nil
			}
			return perrs.Errorf("config dir already lock by another task, %s", lckErr)
		}); lckErr != nil {
			return lckErr
		}
		if lckErr := os.Mkdir(path.Join(dirPath, migrateLockName), 0755); lckErr != nil {
			return perrs.Errorf("can not lock config dir, %s", lckErr)
		}
		defer func() {
			rmErr := os.Remove(path.Join(dirPath, migrateLockName))
			if rmErr != nil {
				log.Errorf("error unlocking config dir, %s", rmErr)
			}
		}()

		// ignore if the old config path does not exist
		if _, err := os.Stat(path.Join(dirPath, "config")); os.IsNotExist(err) {
			return nil
		}
		return os.Rename(path.Join(dirPath, "config"), targetPath)
	}
	return nil
}
