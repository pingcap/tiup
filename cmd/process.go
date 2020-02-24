package cmd

import (
	"encoding/json"
	"os"

	"github.com/c4pt0r/tiup/pkg/utils"
)

const (
	processListFilename = "processes.json"
)

type compProcess struct {
	Pid  int    `json:"pid,omitempty"`
	Exec string `json:"exec,omitempty"`
	Port int    `json:"port,omitempty"`
}

type compProcessList []compProcess

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

	// TODO: check for duplication
	newList := append(currList, *p)
	return saveProcessList(&newList)
}
