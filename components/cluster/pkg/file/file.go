package file

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/errors"
)

// SaveFileWithBackup will backup the file before save is.
// back meta.yaml as meta-2006-01-02T15:04:05Z07:00.yaml
// backup the files in the same dir of path if backupDir is empty.
func SaveFileWithBackup(path string, data []byte, backupDir string) error {
	timestr := time.Now().Format(time.RFC3339Nano)

	info, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return errors.AddStack(err)
	}

	if info != nil && info.IsDir() {
		return errors.Errorf("%s is directory", path)
	}

	// backup file
	if !os.IsNotExist(err) {
		base := filepath.Base(path)
		dir := filepath.Dir(path)

		var backupName string
		p := strings.Split(base, ".")
		if len(p) == 1 {
			backupName = base + "-" + timestr
		} else {
			backupName = strings.Join(p[0:len(p)-1], ".") + "-" + timestr + "." + p[len(p)-1]
		}

		backupData, err := ioutil.ReadFile(path)
		if err != nil {
			return errors.AddStack(err)
		}

		var backupPath string
		if backupDir != "" {
			backupPath = filepath.Join(backupDir, backupName)
		} else {
			backupPath = filepath.Join(dir, backupName)
		}
		err = ioutil.WriteFile(backupPath, backupData, 0644)
		if err != nil {
			return errors.AddStack(err)
		}
	}

	err = ioutil.WriteFile(path, data, 0644)
	if err != nil {
		return errors.AddStack(err)
	}

	return nil
}
