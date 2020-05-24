package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	addr := "127.0.0.1:8989"
	indexKey := ""
	snapshotKey := ""
	timestampKey := ""

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s <root-dir>", os.Args[0]),
		Short: "bootstrap a mirror server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			s, err := newServer(args[0], indexKey, snapshotKey, timestampKey)
			if err != nil {
				return err
			}

			return s.run(addr)
		},
	}
	cmd.Flags().StringVarP(&addr, "addr", "", addr, "addr to listen")
	cmd.Flags().StringVarP(&indexKey, "index", "", "", "specific the private key for index")
	cmd.Flags().StringVarP(&snapshotKey, "snapshot", "", "", "specific the private key for snapshot")
	cmd.Flags().StringVarP(&timestampKey, "timestamp", "", "", "specific the private key for timestamp")

	cmd.MarkFlagRequired("index")
	cmd.MarkFlagRequired("snapshot")
	cmd.MarkFlagRequired("timestamp")

	cmd.Execute()
}
