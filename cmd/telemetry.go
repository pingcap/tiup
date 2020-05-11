package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/telemetry"
	"github.com/spf13/cobra"
)

var telemetryCmd *cobra.Command

const telemetryFname = "meta.yaml"

func getTelemetryFname(env *meta.Environment) (fname string, err error) {
	dir := env.Profile().Path(localdata.TelemetryDir)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return
	}
	fname = filepath.Join(dir, telemetryFname)

	return
}

func newTelemetryCmd(env *meta.Environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telemetry",
		Short: "Controls things about telemetry",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Reset the uuid used for telemetry",
		RunE: func(cmd *cobra.Command, args []string) error {
			fname, err := getTelemetryFname(env)
			if err != nil {
				return err
			}

			teleMeta, err := telemetry.LoadFrom(fname)
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
			fname, err := getTelemetryFname(env)
			if err != nil {
				return err
			}

			teleMeta, err := telemetry.LoadFrom(fname)
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
			fname, err := getTelemetryFname(env)
			if err != nil {
				return err
			}

			teleMeta, err := telemetry.LoadFrom(fname)
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
			fname, err := getTelemetryFname(env)
			if err != nil {
				return err
			}

			teleMeta, err := telemetry.LoadFrom(fname)
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
