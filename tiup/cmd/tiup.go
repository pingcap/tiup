package main

import (
	"fmt"
	"os"

	cmd "github.com/AstroProfundis/tiup-demo/tiup/pkg/commands"
	"github.com/AstroProfundis/tiup-demo/tiup/pkg/version"
)

func main() {
	args := cmd.Init()
	if args.Version {
		fmt.Println(version.NewTiUPVersion())
		os.Exit(0)
	}
}
