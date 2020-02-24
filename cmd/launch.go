package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/c4pt0r/tiup/pkg/meta"
	"github.com/c4pt0r/tiup/pkg/utils"
	"github.com/phayes/freeport"
	"github.com/spf13/cobra"
)

type launchCmd struct {
	*baseCmd
}

const (
	compTypeMeta    = "pd"
	compTypeStorage = "tikv"
	compTypeCompute = "tidb"
)

func newLaunchCmd() *launchCmd {
	var (
		version   string
		component string
	)

	cmdLaunch := &launchCmd{
		newBaseCmd(&cobra.Command{
			Use:   "launch <component1> [version]",
			Short: "Launch a TiDB component of specific version",
			Long: `Launch a TiDB component process of specific version.
There are 3 types of component in "tidb-core":
  meta:     Metadata nodes of the cluster, the PD server
  storage:  Storage nodes, the TiKV server
  compute:  SQL layer and compute nodes, the TiDB server`,
			Example: "tiup launch meta v3.0.8",
			Args: func(cmd *cobra.Command, args []string) error {
				var err error
				switch len(args) {
				case 0:
					cmd.Help()
					return nil
				case 1: // version unspecified, use stable latest as default
					currChan, err := meta.ReadVersionFile()
					if os.IsNotExist(err) {
						fmt.Println("default version not set, using latest stable.")
						compMeta, err := meta.ReadComponentList()
						if os.IsNotExist(err) {
							fmt.Println("no available component list, try `tiup component list --refresh` to get latest online list.")
							return nil
						} else if err != nil {
							return err
						}
						version = compMeta.Stable
					} else if err != nil {
						return err
					}
					version = currChan.Ver
				default:
					version, err = utils.FmtVer(args[1])
					if err != nil {
						return err
					}
				}
				component = strings.ToLower(args[0])
				return nil
			},
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Printf("Launching process of %s %s\n", component, version)
				pid, err := launchComponentProcess(version, component)
				if err != nil {
					return err
				}
				fmt.Printf("Process %d started for %s %s\n", pid, component, version)
				return nil
			},
		}),
	}

	return cmdLaunch
}

func launchComponentProcess(ver, compType string) (int, error) {
	binPath, err := getServerBinPath(ver, compType)
	if err != nil {
		return 0, err
	}

	args, err := getServerArguments(compType)
	if err != nil {
		return 0, err
	}

	fmt.Printf("%s %s\n", binPath, args)
	pid, err := utils.Exec(nil, nil, binPath, args...)
	if err != nil {
		return pid, err
	}

	return pid, saveProcessToList(&compProcess{
		Pid:  pid,
		Exec: binPath,
		Args: strings.Join(args, " "),
	})
}

func getServerBinPath(ver, compType string) (string, error) {
	instComp, err := getInstalledList()
	if err != nil {
		return "", err
	}
	if len(instComp) < 1 {
		return "", fmt.Errorf("no component installed")
	}

	for _, comp := range instComp {
		if comp.Version != ver {
			continue
		}
		switch compType {
		case "compute":
			return filepath.Join(comp.Path,
				fmt.Sprintf("%s-server", compTypeCompute)), nil
		case "meta":
			return filepath.Join(comp.Path,
				fmt.Sprintf("%s-server", compTypeMeta)), nil
		case "storage":
			return filepath.Join(comp.Path,
				fmt.Sprintf("%s-server", compTypeStorage)), nil
		default:
			continue
		}
	}
	return "", fmt.Errorf("can not find binary for %s %s", compType, ver)
}

func getServerArguments(compType string) ([]string, error) {
	// get unused ports
	ports, err := freeport.GetFreePorts(2)
	if err != nil {
		return nil, err
	}

	var args []string
	switch compType {
	case "compute":
		args = []string{
			"-P", fmt.Sprint(ports[0]),
			"-status", fmt.Sprint(ports[1]),
		}
	case "meta":
		fmt.Println("TODO: Support custom ports for PD, now use the default port number")
	case "storage":
		args = []string{
			"--addr", fmt.Sprintf("0.0.0.0:%d", ports[0]),
			"--status-addr", fmt.Sprintf("0.0.0.0:%d", ports[1]),
		}
	}

	return args, nil
}
