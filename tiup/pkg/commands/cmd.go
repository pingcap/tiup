package cmd

import (
	"flag"
)

var (
	printVersion bool
)

type RootFlags struct {
	Version bool
}

func initFlags() {
	flag.BoolVar(&printVersion, "V", false, "show version and quit")
	flag.BoolVar(&printVersion, "version", false, "show version and quit")
	flag.Parse()
}

func Init() *RootFlags {
	initFlags()
	return &RootFlags{
		Version: printVersion,
	}
}
