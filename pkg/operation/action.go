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
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/pingcap-incubator/tiops/pkg/executor"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiops/pkg/module"
	"github.com/pingcap/errors"
)

// Start the cluster.
func Start(
	getter ExecutorGetter,
	w io.Writer,
	spec *meta.Specification,
	component string,
	node string,
) error {
	coms := spec.ComponentsByStartOrder()
	coms = filterComponent(coms, component)

	for _, com := range coms {
		err := StartComponent(getter, w, filterInstance(com.Instances(), node))
		if err != nil {
			return errors.Annotatef(err, "failed to start %s", com.Name())
		}
	}

	return nil
}

// Stop the cluster.
func Stop(
	getter ExecutorGetter,
	w io.Writer,
	spec *meta.Specification,
	component string,
	node string,
) error {
	coms := spec.ComponentsByStopOrder()
	coms = filterComponent(coms, component)

	for _, com := range coms {
		err := StopComponent(getter, w, filterInstance(com.Instances(), node))
		if err != nil {
			return errors.Annotatef(err, "failed to stop %s", com.Name())
		}
	}
	return nil
}

// Restart the cluster.
func Restart(
	getter ExecutorGetter,
	w io.Writer,
	spec *meta.Specification,
	component string,
	node string,
) error {
	coms := spec.ComponentsByStartOrder()
	coms = filterComponent(coms, component)

	for _, com := range coms {
		err := StopComponent(getter, w, filterInstance(com.Instances(), node))
		if err != nil {
			return errors.Annotatef(err, "failed to stop %s", com.Name())
		}

		err = StartComponent(getter, w, filterInstance(com.Instances(), node))
		if err != nil {
			return errors.Annotatef(err, "failed to start %s", com.Name())
		}
	}

	return nil
}

// Destroy the cluster.
func Destroy(
	getter ExecutorGetter,
	w io.Writer,
	spec *meta.Specification,
	component string,
	node string,
) error {

	return nil
}

// StartComponent start the instances.
func StartComponent(getter ExecutorGetter, w io.Writer, instances []meta.Instance) error {
	if len(instances) <= 0 {
		return nil
	}

	name := instances[0].ComponentName()
	fmt.Fprintf(w, "Starting component %s\n", name)

	for _, ins := range instances {
		e := getter.Get(ins.GetHost())
		fmt.Fprintf(w, "Starting instance %s\n", ins.GetHost())

		// Start by systemd.
		c := module.SystemdModuleConfig{
			Unit:   ins.ServiceName(),
			Action: "start",
			// Scope: "",
		}
		systemd := module.NewSystemdModule(c)
		stdout, stderr, err := systemd.Execute(e)

		io.Copy(w, bytes.NewReader(stdout))
		io.Copy(w, bytes.NewReader(stderr))

		if err != nil {
			return errors.Annotatef(err, "failed to start: %s", ins.GetHost())
		}

		// Check ready.
		err = ins.Ready(e)
		if err != nil {
			str := fmt.Sprintf("%s failed to start: %s", ins.GetHost(), err)
			fmt.Fprintln(w, str)
			return errors.Annotatef(err, str)
		}

		fmt.Fprintf(w, "Start %s success\n", ins.GetHost())
	}

	return nil
}

// StopComponent stop the instances.
func StopComponent(getter ExecutorGetter, w io.Writer, instances []meta.Instance) error {
	if len(instances) <= 0 {
		return nil
	}

	name := instances[0].ComponentName()
	fmt.Fprintf(w, "Stopping component %s\n", name)

	for _, ins := range instances {
		e := getter.Get(ins.GetHost())
		fmt.Fprintf(w, "Stopping instance %s\n", ins.GetHost())

		// Stop by systemd.
		c := module.SystemdModuleConfig{
			Unit:   ins.ServiceName(),
			Action: "stop",
			// Scope: "",
		}
		systemd := module.NewSystemdModule(c)
		stdout, stderr, err := systemd.Execute(e)

		io.Copy(w, bytes.NewReader(stdout))
		io.Copy(w, bytes.NewReader(stderr))

		if err != nil {
			return errors.Annotatef(err, "failed to stop: %s", ins.GetHost())
		}

		err = ins.WaitForDown(e)
		if err != nil {
			str := fmt.Sprintf("%s failed to stop: %s", ins.GetHost(), err)
			fmt.Fprintln(w, str)
			return errors.Annotatef(err, str)
		}

		fmt.Fprintf(w, "Stop %s success\n", ins.GetHost())
	}

	return nil
}

func DestroyComponent(getter ExecutorGetter, w io.Writer, instances []meta.Instance) error {
	return errors.New("not implement")
}

/*
[tidb@ip-172-16-5-70 deploy]$ sudo systemctl status drainer-8249.service
● drainer-8249.service - drainer-8249 service
   Loaded: loaded (/etc/systemd/system/drainer-8249.service; disabled; vendor preset: disabled)
   Active: active (running) since Mon 2020-03-09 13:56:19 CST; 1 weeks 3 days ago
 Main PID: 36718 (drainer)
   CGroup: /system.slice/drainer-8249.service
           └─36718 bin/drainer --addr=172.16.5.70:8249 --pd-urls=http://172.16.5.70:2379 --data-dir=/data1/deploy/data.drainer --log-file=/data1/deploy/log/drainer.log --config=conf/drainer.toml --initial-commit-ts=408375872006389761

Mar 09 13:56:19 ip-172-16-5-70 systemd[1]: Started drainer-8249 service.
*/
// getServiceStatus return the Acitive line of status.
func getServiceStatus(e executor.TiOpsExecutor, name string) (active string, err error) {
	c := module.SystemdModuleConfig{
		Unit:   name,
		Action: "status",
	}
	systemd := module.NewSystemdModule(c)
	stdout, _, err := systemd.Execute(e)
	if err != nil {
		return
	}

	lines := strings.Split(string(stdout), "\n")
	if len(lines) < 3 {
		return "", errors.Errorf("unexpected output: %s", string(stdout))
	}

	return lines[2], nil
}

// PrintClusterStatus print cluster status into the io.Writer.
func PrintClusterStatus(getter ExecutorGetter, w io.Writer, spec *meta.Specification) (health bool) {
	health = true

	for _, com := range spec.ComponentsByStartOrder() {
		if len(com.Instances()) == 0 {
			continue
		}

		fmt.Fprintln(w, com.Name())
		for _, ins := range com.Instances() {
			fmt.Fprintf(w, "\t%s\n", ins.GetHost())
			e := getter.Get(ins.GetHost())
			active, err := getServiceStatus(e, ins.ServiceName())
			if err != nil {
				health = false
				fmt.Fprintf(w, "\t\t%s\n", err.Error())
			} else {
				fmt.Fprintf(w, "\t\t%s\n", active)
			}
		}

	}

	return
}
