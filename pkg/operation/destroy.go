package operator

import (
	"bytes"
	"fmt"
	"io"

	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiops/pkg/module"
	"github.com/pingcap/errors"
)

// Destroy the cluster.
func Destroy(
	getter ExecutorGetter,
	w io.Writer,
	spec *meta.Specification,
) error {
	coms := spec.ComponentsByStopOrder()

	for _, com := range coms {
		err := DestroyComponent(getter, w, com.Instances())
		if err != nil {
			return errors.Annotatef(err, "failed to stop %s", com.Name())
		}
	}
	return nil
}

// DestroyComponent destroy the instances.
func DestroyComponent(getter ExecutorGetter, w io.Writer, instances []meta.Instance) error {
	if len(instances) <= 0 {
		return nil
	}

	name := instances[0].ComponentName()
	fmt.Fprintf(w, "Destroying component %s\n", name)

	for _, ins := range instances {
		e := getter.Get(ins.GetHost())
		fmt.Fprintf(w, "Destroying instance %s\n", ins.GetHost())

		// Stop by systemd.
		var command string
		switch name {
		case meta.ComponentTiKV, meta.ComponentPD, meta.ComponentPump, meta.ComponentDrainer, meta.ComponentPrometheus, meta.ComponentAlertManager:
			command = command + fmt.Sprintf("rm -rf %s;", ins.DataDir())
		default:
			command = ""
		}
		command = command + fmt.Sprintf("rm -rf %s;", ins.DeployDir()) + fmt.Sprintf("rm -rf /etc/systemd/system/%s;", ins.ServiceName())
		c := module.ShellModuleConfig{
			Command:  command,
			Sudo:     true, // the .service files are in a directory owned by root
			Chdir:    "",
			UseShell: false,
		}
		shell := module.NewShellModule(c)
		stdout, stderr, err := shell.Execute(e)

		io.Copy(w, bytes.NewReader(stdout))
		io.Copy(w, bytes.NewReader(stderr))

		if err != nil {
			return errors.Annotatef(err, "failed to destroy: %s", ins.GetHost())
		}

		err = ins.WaitForDown(e)
		if err != nil {
			str := fmt.Sprintf("%s failed to destroy: %s", ins.GetHost(), err)
			fmt.Fprintln(w, str)
			return errors.Annotatef(err, str)
		}

		fmt.Fprintf(w, "Destroy %s success\n", ins.GetHost())
	}

	return nil
}
