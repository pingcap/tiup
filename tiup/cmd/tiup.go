package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/AstroProfundis/tiup-demo/tiup/pkg/version"
)

var (
	printVersion bool
)

func init() {
	flag.BoolVar(&printVersion, "V", false, "show version and quit")
	flag.BoolVar(&printVersion, "version", false, "show version and quit")
	flag.Parse()
}

func main() {
	if printVersion {
		fmt.Println(version.NewTiUPVersion())
		os.Exit(0)
	}
}
