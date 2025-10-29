package utils

import (
	"os"
	"os/user"

	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
)

// CurrentUser returns current login user
func CurrentUser() string {
	user, err := user.Current()
	if err != nil {
		logprinter.Errorf("Get current user: %s", err)
		return "root"
	}
	return user.Username
}

// UserHome returns home directory of current user
func UserHome() string {
	homedir, err := os.UserHomeDir()
	if err != nil {
		logprinter.Errorf("Get current user home: %s", err)
		return "root"
	}
	return homedir
}
