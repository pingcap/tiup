package main

import (
	"os"

	"github.com/c4pt0r/tiup/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
