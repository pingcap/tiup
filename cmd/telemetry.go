package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/telemetry"
	"github.com/spf13/cobra"
)

const telemetryFname = "meta.yaml"

func getTelemetryMeta(env *environment.Environment) (meta *telemetry.Meta, fname string, err error) {
	dir := env.Profile().Path(localdata.TelemetryDir)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return
	}

	fname = filepath.Join(dir, telemetryFname)
	meta, err = telemetry.LoadFrom(fname)
	return
}

func newTelemetryCmd(env *environment.Environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "telemetry",
		Hidden: true, // Make false once it's ready to release.
		Short:  "Controls things about telemetry",
	}

	cmd.AddCommand(&cobra.Command{
		Use:    "reset",
		Hidden: false,
		Short:  "Reset the uuid used for telemetry",
		RunE: func(cmd *cobra.Command, args []string) error {
			teleMeta, fname, err := getTelemetryMeta(env)
			if err != nil {
				return err
			}

			teleMeta.UUID = telemetry.NewUUID()
			err = teleMeta.SaveTo(fname)
			if err != nil {
				return err
			}

			fmt.Printf("Reset uuid as: %s success\n", teleMeta.UUID)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "enable",
		Short: "Enable telemetry of tiup",
		RunE: func(cmd *cobra.Command, args []string) error {
			teleMeta, fname, err := getTelemetryMeta(env)
			if err != nil {
				return err
			}

			teleMeta.Status = telemetry.EnableStatus
			err = teleMeta.SaveTo(fname)
			if err != nil {
				return err
			}

			fmt.Printf("Enable telemetry success\n")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "disable",
		Short: "Disable telemetry of tiup",
		RunE: func(cmd *cobra.Command, args []string) error {
			teleMeta, fname, err := getTelemetryMeta(env)
			if err != nil {
				return err
			}

			teleMeta.Status = telemetry.DisableStatus
			err = teleMeta.SaveTo(fname)
			if err != nil {
				return err
			}

			fmt.Printf("Disable telemetry success\n")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Display the current status of tiup telemetry",
		RunE: func(cmd *cobra.Command, args []string) error {
			teleMeta, _, err := getTelemetryMeta(env)
			if err != nil {
				return err
			}

			fmt.Printf("status: %s\n", teleMeta.Status)
			fmt.Printf("uuid: %s\n", teleMeta.UUID)
			return nil
		},
	})

	return cmd
}
