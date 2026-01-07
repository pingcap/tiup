package main

import (
	"context"
	"os/exec"

	"github.com/pingcap/tiup/components/playground/proc"
)

type stubOSProcess struct {
	pid    int
	cmd    *exec.Cmd
	uptime string
}

func (p *stubOSProcess) Start() error                     { return nil }
func (p *stubOSProcess) Wait() error                      { return nil }
func (p *stubOSProcess) Pid() int                         { return p.pid }
func (p *stubOSProcess) Uptime() string                   { return p.uptime }
func (p *stubOSProcess) SetOutputFile(fname string) error { return nil }
func (p *stubOSProcess) Cmd() *exec.Cmd                   { return p.cmd }

type stubProcess struct {
	info    *proc.ProcessInfo
	logFile string
}

func (p *stubProcess) Info() *proc.ProcessInfo           { return p.info }
func (p *stubProcess) Prepare(ctx context.Context) error { return nil }
func (p *stubProcess) LogFile() string                   { return p.logFile }

type stubAddrProcess struct {
	*stubProcess
	addr string
}

func (p *stubAddrProcess) Addr() string { return p.addr }
