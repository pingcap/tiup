package meta

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/pingcap-incubator/tiops/pkg/executor"
	"github.com/pingcap-incubator/tiops/pkg/template/config"
	"github.com/pingcap-incubator/tiops/pkg/template/scripts"
)

// PumpComponent represents Pump component.
type PumpComponent struct{ *Specification }

// Name implements Component interface.
func (c *PumpComponent) Name() string {
	return ComponentPump
}

// Instances implements Component interface.
func (c *PumpComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.PumpServers))
	for _, s := range c.PumpServers {
		ins = append(ins, &PumpInstance{instance{
			InstanceSpec: s,
			name:         c.Name(),
			host:         s.Host,
			port:         s.Port,
			sshp:         s.SSHPort,
			topo:         c.Specification,

			usedPorts: []int{
				s.Port,
			},
			usedDirs: []string{
				s.DeployDir,
				s.DataDir,
			},
			statusFn: func(_ ...string) string {
				url := fmt.Sprintf("http://%s:%d/status", s.Host, s.Port)
				return statusByURL(url)
			},
		}})
	}
	return ins
}

// PumpInstance represent the Pump instance.
type PumpInstance struct {
	instance
}

// ScaleConfig deploy temporary config on scaling
func (i *PumpInstance) ScaleConfig(e executor.TiOpsExecutor, b *Specification, user string, paths DirPaths) error {
	s := i.instance.topo
	defer func() {
		i.instance.topo = s
	}()
	i.instance.topo = b

	return i.InitConfig(e, user, paths)
}

// InitConfig implements Instance interface.
func (i *PumpInstance) InitConfig(e executor.TiOpsExecutor, user string, paths DirPaths) error {
	if err := i.instance.InitConfig(e, user, paths); err != nil {
		return err
	}

	// transfer run script
	ends := []*scripts.PDScript{}
	for _, spec := range i.instance.topo.PDServers {
		ends = append(ends, scripts.NewPDScript(
			spec.Name,
			spec.Host,
			spec.DeployDir,
			spec.DataDir,
			spec.LogDir,
		))
	}

	cfg := scripts.NewPumpScript(
		i.GetHost()+":"+strconv.Itoa(i.GetPort()),
		i.GetHost(),
		paths.Deploy,
		paths.Data,
		paths.Log,
	).AppendEndpoints(ends...)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_pump_%s_%d.sh", i.GetHost(), i.GetPort()))

	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_pump.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	// transfer config
	fp = filepath.Join(paths.Cache, fmt.Sprintf("pump_%s.toml", i.GetHost()))
	if err := config.NewPumpConfig().ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "conf", "pump.toml")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	return nil
}
