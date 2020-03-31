package operator

import (
	"fmt"
	"strings"

	"github.com/pingcap-incubator/tiops/pkg/log"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiops/pkg/module"
	"github.com/pingcap/errors"
)

// Destroy the cluster.
func Destroy(
	getter ExecutorGetter,
	spec *meta.Specification,
) error {
	coms := spec.ComponentsByStopOrder()

	for _, com := range coms {
		err := DestroyComponent(getter, com.Instances())
		if err != nil {
			return errors.Annotatef(err, "failed to destroy %s", com.Name())
		}
	}
	return nil
}

// DestroyComponent destroy the instances.
func DestroyComponent(getter ExecutorGetter, instances []meta.Instance) error {
	if len(instances) <= 0 {
		return nil
	}

	name := instances[0].ComponentName()
	log.Infof("Destroying component %s", name)

	for _, ins := range instances {
		e := getter.Get(ins.GetHost())
		log.Infof("Destroying instance %s", ins.GetHost())

		// Stop by systemd.
		delPaths := make([]string, 0)
		switch name {
		case meta.ComponentTiKV, meta.ComponentPD, meta.ComponentPump, meta.ComponentDrainer, meta.ComponentPrometheus, meta.ComponentAlertManager:
			delPaths = append(delPaths, ins.DataDir())
			fallthrough
		default:
			delPaths = append(delPaths, ins.LogDir())
		}

		// In TiDB-Ansible, deploy dir are shared by all components on the same
		// host, so not deleting it.
		// TODO: this may leave undeleted files when destroying the cluster, fix
		// that later.
		if !ins.IsImported() {
			delPaths = append(delPaths, ins.DeployDir())
		} else {
			log.Warnf("Deploy dir %s not deleted for TiDB-Ansible imported instance %s.",
				ins.DeployDir(), ins.InstanceName())
		}
		delPaths = append(delPaths, fmt.Sprintf("/etc/systemd/system/%s", ins.ServiceName()))
		c := module.ShellModuleConfig{
			Command:  fmt.Sprintf("rm -rf %s;", strings.Join(delPaths, " ")),
			Sudo:     true, // the .service files are in a directory owned by root
			Chdir:    "",
			UseShell: false,
		}
		shell := module.NewShellModule(c)
		stdout, stderr, err := shell.Execute(e)

		if len(stdout) > 0 {
			log.Output(string(stdout))
		}
		if len(stderr) > 0 {
			log.Errorf(string(stderr))
		}

		if err != nil {
			return errors.Annotatef(err, "failed to destroy: %s", ins.GetHost())
		}

		err = ins.WaitForDown(e)
		if err != nil {
			str := fmt.Sprintf("%s failed to destroy: %s", ins.GetHost(), err)
			log.Errorf(str)
			return errors.Annotatef(err, str)
		}

		log.Infof("Destroy %s success", ins.GetHost())
	}

	return nil
}
