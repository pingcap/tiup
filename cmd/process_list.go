package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/c4pt0r/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

type procListCmd struct {
	*baseCmd
}

func newProcListCmd() *procListCmd {
	cmdProcList := &procListCmd{
		newBaseCmd(&cobra.Command{
			Use:   "list",
			Short: "Show process list",
			Long: `Show current process list, note that this is the list saved when
the process launched, the actual process might already exited and no longer running.`,
			RunE: showProcessList,
		}),
	}

	return cmdProcList
}

func showProcessList(cmd *cobra.Command, args []string) error {
	procList, err := getProcessList()
	if err != nil {
		return err
	}

	fmt.Println("Launched processes:")
	var procTable [][]string
	procTable = append(procTable, []string{"Process", "PID", "Working Dir", "Argument"})
	for _, proc := range procList {
		procTable = append(procTable, []string{
			filepath.Base(proc.Exec),
			fmt.Sprint(proc.Pid),
			proc.Dir,
			strings.Join(proc.Args, " "),
		})
	}

	utils.PrintTable(procTable, true)
	return nil
}
