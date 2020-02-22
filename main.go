package main

import (
	"fmt"
	"os"

	"github.com/c4pt0r/tiup/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
