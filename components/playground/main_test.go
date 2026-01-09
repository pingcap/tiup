package main

import (
	"os"
	"strings"
	"testing"
)

// To build:
// see build_integration_test in Makefile
// To run:
// tiup-playground.test -test.coverprofile={file} __DEVEL--i-heard-you-like-tests
func TestMain(t *testing.T) {
	var (
		args []string
		run  bool
	)

	for _, arg := range os.Args {
		switch {
		case arg == "__DEVEL--i-heard-you-like-tests":
			run = true
		case strings.HasPrefix(arg, "-test"):
		case strings.HasPrefix(arg, "__DEVEL"):
		default:
			args = append(args, arg)
		}
	}
	os.Args = args

	if run {
		main()
	}
}
