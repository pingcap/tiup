package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/instance"
)

// CommandType send to playground.
type CommandType string

// types of CommandType
const (
	ScaleInCommandType  CommandType = "scale-in"
	ScaleOutCommandType CommandType = "scale-out"
	DisplayCommandType  CommandType = "display"
)

// Command send to Playground.
type Command struct {
	CommandType CommandType
	ComponentID string
	instance.Config
}

func buildCommands(tp CommandType, opt *bootOptions) (cmds []Command) {
	for i := 0; i < opt.pd.Num; i++ {
		c := Command{
			CommandType: tp,
			ComponentID: "pd",
			Config:      opt.pd,
		}

		cmds = append(cmds, c)
	}
	for i := 0; i < opt.tikv.Num; i++ {
		c := Command{
			CommandType: tp,
			ComponentID: "tikv",
			Config:      opt.tikv,
		}

		cmds = append(cmds, c)
	}
	for i := 0; i < opt.tiflash.Num; i++ {
		c := Command{
			CommandType: tp,
			ComponentID: "tiflash",
			Config:      opt.tiflash,
		}

		cmds = append(cmds, c)
	}
	for i := 0; i < opt.tidb.Num; i++ {
		c := Command{
			CommandType: tp,
			ComponentID: "tidb",
			Config:      opt.tidb,
		}

		cmds = append(cmds, c)
	}
	return
}

func scaleIn(args []string, opt *bootOptions) error {
	port, err := targetTag()
	if err != nil {
		return errors.AddStack(err)
	}

	cmds := buildCommands(ScaleInCommandType, opt)

	addr := "127.0.0.1:" + strconv.Itoa(port)
	return sendCommandsAndPrintResult(cmds, addr)
}

func scaleOut(args []string, opt *bootOptions) error {
	port, err := targetTag()
	if err != nil {
		return errors.AddStack(err)
	}

	cmds := buildCommands(ScaleOutCommandType, opt)
	addr := "127.0.0.1:" + strconv.Itoa(port)
	return sendCommandsAndPrintResult(cmds, addr)
}

func display(args []string) error {
	port, err := targetTag()
	if err != nil {
		return errors.AddStack(err)
	}
	c := Command{
		CommandType: DisplayCommandType,
	}

	addr := "127.0.0.1:" + strconv.Itoa(port)
	return sendCommandsAndPrintResult([]Command{c}, addr)
}

func sendCommandsAndPrintResult(cmds []Command, addr string) error {
	for _, cmd := range cmds {
		rc, err := requestCommand(cmd, addr)
		if err != nil {
			return errors.AddStack(err)
		}

		_, err = io.Copy(os.Stdout, rc)
		rc.Close()
		if err != nil {
			return errors.AddStack(err)
		}
	}

	return nil
}

func requestCommand(cmd Command, addr string) (r io.ReadCloser, err error) {
	data, err := json.Marshal(&cmd)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	url := fmt.Sprintf("http://%s/command", addr)

	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, errors.AddStack(err)
	}

	return resp.Body, nil
}
