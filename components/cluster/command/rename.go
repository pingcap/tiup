package command

import (
	"github.com/spf13/cobra"
)

func newRenameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename <old-cluster-name> <new-cluster-name>",
		Short: "Rename the cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			if err := validRoles(gOpt.Roles); err != nil {
				return err
			}

			oldClusterName := args[0]
			newClusterName := args[1]
			teleCommand = append(teleCommand, scrubClusterName(oldClusterName))

			return manager.Rename(oldClusterName, gOpt, newClusterName)
		},
	}

	return cmd
}
