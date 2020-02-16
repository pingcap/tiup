package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
)

var (
	profileDirName = ".tiup"
)

// SaveToProfile saves file to the profile directory, path is relative to the
// profile directory of current user
func SaveToProfile(path string, data []byte, perm os.FileMode) error {
	profilePath, err := GetOrCreateProfileDir()
	if err != nil {
		return err
	}
	filePath := filepath.Join(profilePath, path)
	// create sub directory if needed
	if err := createDir(filepath.Dir(filePath)); err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, data, perm)
}

// WriteJSON writes struct to a file (in the profile directory) in JSON format
func WriteJSON(path string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return SaveToProfile(path, jsonData, 0644)
}

// ReadFile reads data from a file in the profile directory
func ReadFile(path string) ([]byte, error) {
	profilePath, err := GetOrCreateProfileDir()
	if err != nil {
		return nil, err
	}
	filePath := filepath.Join(profilePath, path)
	return ioutil.ReadFile(filePath)
}

// getHomeDir get the home directory of current user (if they have one).
// The result path might be empty.
func getHomeDir() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.HomeDir, nil
}

// GetOrCreateProfileDir check for profile directory for the current user, and create
// the directory if not already exist.
// Error may be returned if the directory is not exist and fail to create.
func GetOrCreateProfileDir() (string, error) {
	homeDir, err := getHomeDir()
	if err != nil {
		return "", err
	}
	if len(homeDir) < 1 {
		return "", fmt.Errorf("unable to get home directoy of current user")
	}

	profilePath := path.Join(homeDir, profileDirName)
	if err := createDir(profilePath); err != nil {
		return "", err
	}
	return profilePath, nil
}

// createDir creates the directory if it not alerady exist.
func createDir(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(path, 0755)
			if err != nil {
				return err
			}
		}
		return err
	}
	return nil
}
