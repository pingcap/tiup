package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	showAll bool
)

func newShowCmd() *cobra.Command {
	cmdShow := &cobra.Command{
		Use:   "show [options]",
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
	}

	cmdShow.Flags().BoolVar(&showAll, "all", false, "Show all available components and versions (refresh online).")

	return cmdShow
}
