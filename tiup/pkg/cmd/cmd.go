package cmd

import (
	"fmt"

	"github.com/AstroProfundis/tiup-demo/tiup/pkg/version"
	"github.com/spf13/cobra"
)

var rootCmd *cobra.Command

func init() {
	var printVersion bool

	rootCmd = &cobra.Command{
		Use:   "tiup",
		Short: "Download and install TiDB components from command line",
		Long: `The tiup utility is a command line tool that can help downloading
and installing TiDB components to the local system.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if printVersion {
				fmt.Println(version.NewTiUPVersion())
			}
			return nil
		},
	}

	rootCmd.Flags().BoolVarP(&printVersion, "version", "V", false, "Show tiup version and quit")

	rootCmd.AddCommand(newShowCmd())
}

func Execute() error {
	return rootCmd.Execute()
}
