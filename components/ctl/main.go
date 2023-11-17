package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/fatih/color"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func main() {
	if err := execute(); err != nil {
		os.Exit(1)
	}
}

func execute() error {
	ignoreVersion := false
	home := os.Getenv(localdata.EnvNameComponentInstallDir)
	if home == "" {
		return errors.New("component `ctl` cannot run in standalone mode")
	}
	rootCmd := &cobra.Command{
		Use:                "tiup ctl {tidb/pd/tikv/binlog/etcd/cdc/tidb-lightning}",
		Short:              "TiDB controllers",
		SilenceUsage:       true,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			var transparentParams []string
			componentSpec := args[0]
			for i, arg := range os.Args {
				if arg == componentSpec {
					transparentParams = os.Args[i+1:]
					break
				}
			}

			if ignoreVersion {
				for i, arg := range transparentParams {
					if arg == "--ignore-version" {
						transparentParams = append(transparentParams[:i], transparentParams[i+1:]...)
					}
				}
			} else if os.Getenv(localdata.EnvNameUserInputVersion) == "" {
				// if user not set component version explicitly
				return errors.New(
					"ctl needs an explicit version, please run with `tiup ctl:<cluster-version>`, if you continue seeing this error, please upgrade your TiUP with `tiup update --self`",
				)
			}

			bin, err := binaryPath(home, componentSpec)
			if err != nil {
				return err
			}
			return run(bin, transparentParams...)
		},
	}

	originHelpFunc := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if len(args) < 2 {
			originHelpFunc(cmd, args)
			return
		}
		args = utils.RebuildArgs(args)
		bin, err := binaryPath(home, args[0])
		if err != nil {
			fmt.Println(color.RedString("Error: %v", err))
			return
		}
		if err := run(bin, args[1:]...); err != nil {
			fmt.Println(color.RedString("Error: %v", err))
		}
	})
	rootCmd.Flags().BoolVar(&ignoreVersion, "ignore-version", false, "Skip explicit version check")

	return rootCmd.Execute()
}

func binaryPath(home, cmd string) (string, error) {
	switch cmd {
	case "tidb", "tikv", "pd", "tidb-lightning":
		return path.Join(home, cmd+"-ctl"), nil
	case "binlog", "etcd":
		return path.Join(home, cmd+"ctl"), nil
	case "cdc":
		return path.Join(home, cmd+" cli"), nil
	default:
		return "", errors.New("ctl only supports tidb, tikv, pd, binlog, tidb-lightning, etcd and cdc currently")
	}
}

func run(name string, args ...string) error {
	os.Setenv("ETCDCTL_API", "3")
	// Handle `cdc cli`
	if strings.Contains(name, " ") {
		xs := strings.Split(name, " ")
		name = xs[0]
		args = append(xs[1:], args...)
	}
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
