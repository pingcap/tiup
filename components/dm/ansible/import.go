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

package ansible

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/dm/spec"
	"github.com/pingcap/tiup/pkg/cluster/ansible"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/relex/aini"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v2"
)

// ref https://docs.ansible.com/ansible/latest/reference_appendices/config.html#the-configuration-file
// Changes can be made and used in a configuration file which will be searched for in the following order:
//
// ANSIBLE_CONFIG (environment variable if set)
// ansible.cfg (in the current directory)
// ~/.ansible.cfg (in the home directory)
// /etc/ansible/ansible.cfg
func searchConfigFile(dir string) (fname string, err error) {
	// ANSIBLE_CONFIG (environment variable if set)
	if v := os.Getenv("ANSIBLE_CONFIG"); len(v) > 0 {
		return v, nil
	}

	// ansible.cfg (in the current directory)
	f := filepath.Join(dir, "ansible.cfg")
	if utils.IsExist(f) {
		return f, nil
	}

	// ~/.ansible.cfg (in the home directory)
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.AddStack(err)
	}
	f = filepath.Join(home, ".ansible.cfg")
	if utils.IsExist(f) {
		return f, nil
	}

	// /etc/ansible/ansible.cfg
	f = "/etc/ansible/ansible.cfg"
	if utils.IsExist(f) {
		return f, nil
	}

	return "", errors.Errorf("can not found ansible.cfg, dir: %s", dir)
}

func readConfigFile(dir string) (file *ini.File, err error) {
	fname, err := searchConfigFile(dir)
	if err != nil {
		return nil, err
	}

	file, err = ini.Load(fname)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to load ini: %s", fname)
	}

	return
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}

	return ""
}

func getAbsPath(dir string, path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	path = filepath.Join(dir, path)
	return path
}

// ExecutorGetter get the executor by host.
type ExecutorGetter interface {
	Get(host string) (e ctxt.Executor)
}

// Importer used for import from ansible.
// ref DM docs: https://docs.pingcap.com/zh/tidb-data-migration/dev/deploy-a-dm-cluster-using-ansible
type Importer struct {
	dir               string // ansible directory.
	inventoryFileName string
	sshType           executor.SSHType
	sshTimeout        uint64

	// following vars parse from ansbile
	user    string
	sources map[string]*SourceConfig // addr(ip:port) -> SourceConfig

	// only use for test.
	// when setted, we use this executor instead of getting a truly one.
	testExecutorGetter ExecutorGetter
}

// NewImporter create an Importer.
// @sshTimeout: set 0 to use a default value
func NewImporter(ansibleDir, inventoryFileName string, sshType executor.SSHType, sshTimeout uint64) (*Importer, error) {
	dir, err := filepath.Abs(ansibleDir)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	return &Importer{
		dir:               dir,
		inventoryFileName: inventoryFileName,
		sources:           make(map[string]*SourceConfig),
		sshType:           sshType,
		sshTimeout:        sshTimeout,
	}, nil
}

func (im *Importer) getExecutor(host string, port int) (e ctxt.Executor, err error) {
	if im.testExecutorGetter != nil {
		return im.testExecutorGetter.Get(host), nil
	}

	keypath := ansible.SSHKeyPath()

	cfg := executor.SSHConfig{
		Host:    host,
		Port:    port,
		User:    im.user,
		KeyFile: keypath,
		Timeout: time.Second * time.Duration(im.sshTimeout),
	}

	e, err = executor.New(im.sshType, false, cfg)

	return
}

func (im *Importer) fetchFile(ctx context.Context, host string, port int, fname string) (data []byte, err error) {
	e, err := im.getExecutor(host, port)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to get executor, target: %s", utils.JoinHostPort(host, port))
	}

	tmp, err := os.MkdirTemp("", "tiup")
	if err != nil {
		return nil, errors.AddStack(err)
	}
	defer os.RemoveAll(tmp)

	tmp = filepath.Join(tmp, filepath.Base(fname))

	err = e.Transfer(ctx, fname, tmp, true /*download*/, 0, false)
	if err != nil {
		return nil, errors.Annotatef(err, "transfer %s from %s", fname, utils.JoinHostPort(host, port))
	}

	data, err = os.ReadFile(tmp)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	return
}

func setConfig(config *map[string]any, k string, v any) {
	if *config == nil {
		*config = make(map[string]any)
	}

	(*config)[k] = v
}

// handleWorkerConfig fetch the config file of worker and generate the source
// which we need for the master.
func (im *Importer) handleWorkerConfig(ctx context.Context, srv *spec.WorkerSpec, fname string) error {
	data, err := im.fetchFile(ctx, srv.Host, srv.SSHPort, fname)
	if err != nil {
		return err
	}

	config := new(Config)
	err = toml.Unmarshal(data, config)
	if err != nil {
		return errors.AddStack(err)
	}

	source := config.ToSource()
	im.sources[srv.Host+":"+strconv.Itoa(srv.Port)] = source

	return nil
}

// ScpSourceToMaster scp the source files to master,
// and set V1SourcePath of the master spec.
func (im *Importer) ScpSourceToMaster(ctx context.Context, topo *spec.Specification) (err error) {
	for i := 0; i < len(topo.Masters); i++ {
		master := topo.Masters[i]
		target := filepath.Join(firstNonEmpty(master.DeployDir, topo.GlobalOptions.DeployDir), "v1source")
		master.V1SourcePath = target

		e, err := im.getExecutor(master.Host, master.SSHPort)
		if err != nil {
			return errors.Annotatef(err, "failed to get executor, target: %s", utils.JoinHostPort(master.Host, master.SSHPort))
		}
		_, stderr, err := e.Execute(ctx, "mkdir -p "+target, false)
		if err != nil {
			return errors.Annotatef(err, "failed to execute: %s", string(stderr))
		}

		for addr, source := range im.sources {
			f, err := os.CreateTemp("", "tiup-dm-*")
			if err != nil {
				return errors.AddStack(err)
			}

			data, err := yaml.Marshal(source)
			if err != nil {
				return errors.AddStack(err)
			}

			_, err = f.Write(data)
			if err != nil {
				return errors.AddStack(err)
			}

			err = e.Transfer(ctx, f.Name(), filepath.Join(target, addr+".yml"), false, 0, false)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func instancDeployDir(comp string, port int, hostDir string, globalDir string) string {
	if hostDir != globalDir {
		return filepath.Join(hostDir, fmt.Sprintf("%s-%d", comp, port))
	}

	return ""
}

// ImportFromAnsibleDir generate the metadata from ansible deployed cluster.
//
//revive:disable
func (im *Importer) ImportFromAnsibleDir(ctx context.Context) (clusterName string, meta *spec.Metadata, err error) {
	dir := im.dir
	inventoryFileName := im.inventoryFileName

	cfg, err := readConfigFile(dir)
	if err != nil {
		return "", nil, err
	}

	fname := filepath.Join(dir, inventoryFileName)
	file, err := os.Open(fname)
	if err != nil {
		return "", nil, errors.AddStack(err)
	}

	inventory, err := aini.Parse(file)
	if err != nil {
		return "", nil, errors.AddStack(err)
	}

	meta = &spec.Metadata{
		Topology: new(spec.Specification),
	}
	topo := meta.Topology

	// Grafana admin username and password
	var grafanaUser string
	var grafanaPass string
	if group, ok := inventory.Groups["all"]; ok {
		for k, v := range group.Vars {
			switch k {
			case "ansible_user":
				meta.User = v
				im.user = v
			case "dm_version":
				meta.Version = v
			case "cluster_name":
				clusterName = v
			case "deploy_dir":
				topo.GlobalOptions.DeployDir = v
				// ansible convention directory for log
				topo.GlobalOptions.LogDir = filepath.Join(v, "log")
			case "grafana_admin_user":
				grafanaUser = strings.Trim(v, "\"")
			case "grafana_admin_password":
				grafanaPass = strings.Trim(v, "\"")
			default:
				fmt.Println("ignore unknown global var ", k, v)
			}
		}
	}

	for gname, group := range inventory.Groups {
		switch gname {
		case "dm_master_servers":
			for _, host := range group.Hosts {
				srv := &spec.MasterSpec{
					Host:     host.Vars["ansible_host"],
					SSHPort:  ansible.GetHostPort(host, cfg),
					Imported: true,
				}

				runFileName := filepath.Join(host.Vars["deploy_dir"], "scripts", "run_dm-master.sh")
				data, err := im.fetchFile(ctx, srv.Host, srv.SSHPort, runFileName)
				if err != nil {
					return "", nil, err
				}
				deployDir, flags, err := parseRunScript(data)
				if err != nil {
					return "", nil, err
				}

				if deployDir == "" {
					return "", nil, errors.Errorf("unexpected run script %s, can get deploy dir", runFileName)
				}

				for k, v := range flags {
					switch k {
					case "master-addr":
						ar := strings.Split(v, ":")
						port, err := strconv.Atoi(ar[len(ar)-1])
						if err != nil {
							return "", nil, errors.AddStack(err)
						}
						srv.Port = port
						// srv.PeerPort use default value
					case "L":
						// in tiup, must set in Config.
						setConfig(&srv.Config, "log-level", v)
					case "config":
						// Ignore the config file, nothing we care.
					case "log-file":
						srv.LogDir = filepath.Dir(getAbsPath(deployDir, v))
					default:
						fmt.Printf("ignore unknown arg %s=%s in run script %s\n", k, v, runFileName)
					}
				}

				srv.DeployDir = instancDeployDir(spec.ComponentDMMaster, srv.Port, host.Vars["deploy_dir"], topo.GlobalOptions.DeployDir)

				topo.Masters = append(topo.Masters, srv)
			}
		case "dm_worker_servers":
			for _, host := range group.Hosts {
				srv := &spec.WorkerSpec{
					Host:      host.Vars["ansible_host"],
					SSHPort:   ansible.GetHostPort(host, cfg),
					DeployDir: firstNonEmpty(host.Vars["deploy_dir"], topo.GlobalOptions.DeployDir),
					Imported:  true,
				}

				runFileName := filepath.Join(host.Vars["deploy_dir"], "scripts", "run_dm-worker.sh")
				data, err := im.fetchFile(ctx, srv.Host, srv.SSHPort, runFileName)
				if err != nil {
					return "", nil, err
				}
				deployDir, flags, err := parseRunScript(data)
				if err != nil {
					return "", nil, err
				}

				if deployDir == "" {
					return "", nil, errors.Errorf("unexpected run script %s, can not get deploy directory", runFileName)
				}

				var configFileName string
				for k, v := range flags {
					switch k {
					case "worker-addr":
						ar := strings.Split(v, ":")
						port, err := strconv.Atoi(ar[len(ar)-1])
						if err != nil {
							return "", nil, errors.AddStack(err)
						}
						srv.Port = port
					case "L":
						// in tiup, must set in Config.
						setConfig(&srv.Config, "log-level", v)
					case "config":
						configFileName = getAbsPath(deployDir, v)
					case "log-file":
						srv.LogDir = filepath.Dir(getAbsPath(deployDir, v))
					case "relay-dir":
						// Safe to ignore this
					default:
						fmt.Printf("ignore unknown arg %s=%s in run script %s\n", k, v, runFileName)
					}
				}

				// Deploy dir MUST always keep the same and CAN NOT change.
				// dm-worker will save the data in the wording directory and there's no configuration
				// to specific the directory.
				// We will always set the wd as DeployDir.
				srv.DeployDir = deployDir

				err = im.handleWorkerConfig(ctx, srv, configFileName)
				if err != nil {
					return "", nil, err
				}

				topo.Workers = append(topo.Workers, srv)
			}
		case "dm_portal_servers":
			fmt.Println("ignore deprecated dm_portal_servers")
		case "prometheus_servers":
			for _, host := range group.Hosts {
				srv := &spec.PrometheusSpec{
					Host:      host.Vars["ansible_host"],
					SSHPort:   ansible.GetHostPort(host, cfg),
					DeployDir: firstNonEmpty(host.Vars["deploy_dir"], topo.GlobalOptions.DeployDir),
					Imported:  true,
				}

				runFileName := filepath.Join(host.Vars["deploy_dir"], "scripts", "run_prometheus.sh")
				data, err := im.fetchFile(ctx, srv.Host, srv.SSHPort, runFileName)
				if err != nil {
					return "", nil, err
				}

				deployDir, flags, err := parseRunScript(data)
				if err != nil {
					return "", nil, err
				}

				if deployDir == "" {
					return "", nil, errors.Errorf("unexpected run script %s, can get deploy dir", runFileName)
				}

				for k, v := range flags {
					// just get data directory and port, ignore all other flags.
					switch k {
					case "storage.tsdb.path":
						srv.DataDir = getAbsPath(deployDir, v)
					case "web.listen-address":
						ar := strings.Split(v, ":")
						port, err := strconv.Atoi(ar[len(ar)-1])
						if err != nil {
							return "", nil, errors.AddStack(err)
						}
						srv.Port = port
					case "STDOUT":
						srv.LogDir = filepath.Dir(getAbsPath(deployDir, v))
					case "config.file", "web.external-url", "log.level", "storage.tsdb.retention":
						// ignore intent
					default:
						fmt.Printf("ignore unknown arg %s=%s in run script %s\n", k, v, runFileName)
					}
				}

				srv.DeployDir = instancDeployDir(spec.ComponentPrometheus, srv.Port, host.Vars["deploy_dir"], topo.GlobalOptions.DeployDir)

				topo.Monitors = append(topo.Monitors, srv)
			}
		case "alertmanager_servers":
			for _, host := range group.Hosts {
				srv := &spec.AlertmanagerSpec{
					Host:      host.Vars["ansible_host"],
					SSHPort:   ansible.GetHostPort(host, cfg),
					DeployDir: firstNonEmpty(host.Vars["deploy_dir"], topo.GlobalOptions.DeployDir),
					Imported:  true,
				}

				runFileName := filepath.Join(host.Vars["deploy_dir"], "scripts", "run_alertmanager.sh")
				data, err := im.fetchFile(ctx, srv.Host, srv.SSHPort, runFileName)
				if err != nil {
					return "", nil, err
				}

				deployDir, flags, err := parseRunScript(data)
				if err != nil {
					return "", nil, err
				}

				if deployDir == "" {
					return "", nil, errors.Errorf("unexpected run script %s, can get deploy dir", runFileName)
				}

				for k, v := range flags {
					switch k {
					case "storage.path":
						srv.DataDir = getAbsPath(deployDir, v)
					case "web.listen-address":
						ar := strings.Split(v, ":")
						port, err := strconv.Atoi(ar[len(ar)-1])
						if err != nil {
							return "", nil, errors.AddStack(err)
						}
						srv.WebPort = port
					case "STDOUT":
						srv.LogDir = filepath.Dir(getAbsPath(deployDir, v))
					case "config.file", "data.retention", "log.level":
						// ignore
					default:
						fmt.Printf("ignore unknown arg %s=%s in run script %s\n", k, v, runFileName)
					}
				}

				srv.DeployDir = instancDeployDir(spec.ComponentAlertmanager, srv.WebPort, host.Vars["deploy_dir"], topo.GlobalOptions.DeployDir)

				topo.Alertmanagers = append(topo.Alertmanagers, srv)
			}
		case "grafana_servers":
			for _, host := range group.Hosts {
				// Do not fetch the truly used config file of Grafana,
				// get port directly from ansible ini files.
				port := 3000
				if v, ok := host.Vars["grafana_port"]; ok {
					if iv, err := strconv.Atoi(v); err == nil {
						port = iv
					}
				}
				srv := &spec.GrafanaSpec{
					Host:     host.Vars["ansible_host"],
					SSHPort:  ansible.GetHostPort(host, cfg),
					Port:     port,
					Username: grafanaUser,
					Password: grafanaPass,
					Imported: true,
				}

				runFileName := filepath.Join(host.Vars["deploy_dir"], "scripts", "run_grafana.sh")
				data, err := im.fetchFile(ctx, srv.Host, srv.SSHPort, runFileName)
				if err != nil {
					return "", nil, err
				}
				_, _, err = parseRunScript(data)
				if err != nil {
					return "", nil, err
				}

				srv.DeployDir = instancDeployDir(spec.ComponentGrafana, srv.Port, host.Vars["deploy_dir"], topo.GlobalOptions.DeployDir)
				topo.Grafanas = append(topo.Grafanas, srv)
			}
		case "all", "ungrouped":
			// ignore intent
		default:
			fmt.Println("ignore unknown group ", gname)
		}
	}

	return
}

// parseRunScript parse the run script generate by dm-ansible
// flags contains the flags of command line, adding a key "STDOUT"
// if it redirect the stdout to a file.
func parseRunScript(data []byte) (deployDir string, flags map[string]string, err error) {
	scanner := bufio.NewScanner(bytes.NewBuffer(data))

	flags = make(map[string]string)

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// parse "DEPLOY_DIR=/home/tidb/deploy"
		prefix := "DEPLOY_DIR="
		if strings.HasPrefix(line, prefix) {
			deployDir = line[len(prefix):]
			deployDir = strings.TrimSpace(deployDir)
			continue
		}

		// parse such line:
		// exec > >(tee -i -a "/home/tidb/deploy/log/alertmanager.log")
		//
		// get the file path, as a "STDOUT" flag.
		if strings.Contains(line, "tee -i -a") {
			left := strings.Index(line, "\"")
			right := strings.LastIndex(line, "\"")
			if left < right {
				v := line[left+1 : right]
				flags["STDOUT"] = v
			}
		}

		// trim the ">> /path/to/file ..." part
		if index := strings.Index(line, ">>"); index != -1 {
			line = line[:index]
		}

		line = strings.TrimSuffix(line, "\\")
		line = strings.TrimSpace(line)

		// parse flag
		if strings.HasPrefix(line, "-") {
			seps := strings.Split(line, "=")
			if len(seps) != 2 {
				continue
			}

			k := strings.TrimLeft(seps[0], "-")
			v := strings.Trim(seps[1], "\"")
			flags[k] = v
		}
	}

	return
}
