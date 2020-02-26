package main

import (
	"fmt"
	"os"

	"github.com/c4pt0r/tiup/cmd"
)

const DEBUG = true

func main() {
	if err := cmd.Execute(); err != nil {
		if DEBUG {
			fmt.Printf("%+v\n", err)
		}
		os.Exit(1)
	}
}
