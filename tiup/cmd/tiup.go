package main

import (
	"fmt"
	"os"

	"github.com/AstroProfundis/tiup-demo/tiup/pkg/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
