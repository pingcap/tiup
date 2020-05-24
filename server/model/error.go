package model

import (
	"errors"
)

var (
	// ErrorConflict indicats manifest conflict
	ErrorConflict = errors.New("manifest conflict")
)
