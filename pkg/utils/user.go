package utils

import (
	log2 "github.com/pingcap-incubator/tiup/pkg/logger/log"
	"os/user"
)

// CurrentUser returns current login user
func CurrentUser() string {
	user, err := user.Current()
	if err != nil {
		log2.Errorf("Get current user: %s", err)
		return "root"
	}
	return user.Username
}

// UserHome returns home directory of current user
func UserHome() string {
	user, err := user.Current()
	if err != nil {
		log2.Errorf("Get current user home: %s", err)
		return "root"
	}
	return user.HomeDir
}
