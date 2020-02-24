package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/c4pt0r/tiup/pkg/utils"
)

const (
	processListFilename = "processes.json"
)

type compProcess struct {
	Pid  int      `json:"pid,omitempty"`  // PID of the process
	Exec string   `json:"exec,omitempty"` // Path to the binary
	Args []string `json:"args,omitempty"` // Command line arguments
	Dir  string   `json:"dir,omitempty"`  // Working directory
}

type compProcessList []compProcess

// Launch executes the process
func (p *compProcess) Launch() error {
	var err error

	dir := utils.MustDir(p.Dir)
	p.Pid, err = utils.Exec(nil, nil, dir, p.Exec, p.Args...)
	if err != nil {
		return err
	}
	return nil
}

func getProcessList() (compProcessList, error) {
	var list compProcessList
	var err error

	data, err := utils.ReadFile(processListFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return list, nil
		}
		return nil, err
	}
	if err = json.Unmarshal(data, &list); err != nil {
		return nil, err
	}

	return list, err
}

func saveProcessList(pl *compProcessList) error {
	return utils.WriteJSON(processListFilename, pl)
}

func saveProcessToList(p *compProcess) error {
	currList, err := getProcessList()
	if err != nil {
		return err
	}

	for _, currProc := range currList {
		if currProc.Pid == p.Pid {
			return fmt.Errorf("process %d already exist", p.Pid)
		}
	}

	newList := append(currList, *p)
	return saveProcessList(&newList)
}
