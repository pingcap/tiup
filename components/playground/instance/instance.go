// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package instance

import (
	"context"
	"fmt"

	"github.com/pingcap/tiup/pkg/repository/v0manifest"
)

// Config of the instance.
type Config struct {
	ConfigPath string
	BinPath    string
	Num        int
	Host       string
}

type instance struct {
	ID         int
	Dir        string
	Host       string
	Port       int
	StatusPort int // client port for PD
	ConfigPath string
	BinPath    string
}

// Instance represent running component
type Instance interface {
	Pid() int
	// Start the instance process.
	// Will kill the process once the context is done.
	Start(ctx context.Context, version v0manifest.Version) error
	// Component Return the component name.
	Component() string
	// LogFile return the log file name
	LogFile() string
	// Uptime show uptime.
	Uptime() string
	// StatusAddrs return the address to pull metrics.
	StatusAddrs() []string
	// Wait Should only call this if the instance is started successfully.
	// The implementation should be safe to call Wait multi times.
	Wait() error
}

func (inst *instance) StatusAddrs() (addrs []string) {
	if inst.Host != "" && inst.StatusPort != 0 {
		addrs = append(addrs, fmt.Sprintf("%s:%d", inst.Host, inst.StatusPort))
	}
	return
}

// CompVersion return the format to run specified version of a component.
func CompVersion(comp string, version v0manifest.Version) string {
	if version.IsEmpty() {
		return comp
	}
	return fmt.Sprintf("%v:%v", comp, version)
}

func advertiseHost(listen string) string {
	if listen == "0.0.0.0" {
		return "localhost"
	}

	return listen
}

func logIfErr(err error) {
	if err != nil {
		fmt.Println(err)
	}
}
