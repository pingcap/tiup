package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

type showCmd struct {
	*baseCmd
}

func newShowCmd() *showCmd {
	var (
		showAll bool
	)

	cmdShow := &showCmd{
		newBaseCmd(&cobra.Command{
			Use:   "show",
			Short: "Show the available TiDB components",
			Long:  `Show available and installed TiDB components and their versions.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				if showAll {
					fmt.Println("TODO: grap online list...")
				} else {
					fmt.Println("TODO: show installed list...")
				}
				return nil
			},
		}),
	}

	cmdShow.cmd.Flags().BoolVar(&showAll, "all", false, "Show all available components and versions (refresh online).")

	return cmdShow
}
