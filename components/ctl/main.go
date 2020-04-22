package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/fatih/color"
	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func main() {
	if err := execute(); err != nil {
		os.Exit(1)
	}
}

func execute() error {
	home := os.Getenv(localdata.EnvNameComponentInstallDir)
	if home == "" {
		return errors.New("component `ctl` cannot run in standalone mode")
	}
	rootCmd := &cobra.Command{
		Use:          "tiup ctl {tidb/pd/tikv/binlog/etcd}",
		Short:        "TiDB controllers",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			bin, err := binaryPath(home, args[0])
			if err != nil {
				return err
			}
			if err := run(bin, args[1:]...); err != nil {
				return err
			}

			return nil
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

	return rootCmd.Execute()
}

func binaryPath(home, cmd string) (string, error) {
	switch cmd {
	case "tidb", "tikv", "pd":
		return path.Join(home, cmd+"-ctl"), nil
	case "binlog", "etcd":
		return path.Join(home, cmd+"ctl"), nil
	default:
		return "", errors.New("ctl only supports tidb, tikv, pd, binlog and etcd currently")
	}
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
