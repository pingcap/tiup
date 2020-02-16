package utils

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
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
	if err := CreateDir(filepath.Dir(filePath)); err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, data, perm)
}

// ProfileDir returns profile directory, create it if
// the directory is not already exist.
// Fatal when the directory is not exist and fail to create.
func ProfileDir() string {
	dir, err := GetOrCreateProfileDir()
	if err != nil {
		log.Fatal(err)
		return ""
	}
	return dir
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

	filePath := path
	if !strings.HasPrefix(filePath, profilePath) {
		filePath = filepath.Join(profilePath, path)
	}
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
	if err := CreateDir(profilePath); err != nil {
		return "", err
	}
	return profilePath, nil
}

// CreateDir creates the directory if it not alerady exist.
func CreateDir(path string) error {
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

// MustDir makes sure dir exists and return the path name
func MustDir(path string) string {
	if err := CreateDir(path); err != nil {
		log.Fatal(err)
		return ""
	}
	return path
}

// Untar decompresses the tarball
func Untar(file, to string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		if hdr.FileInfo().IsDir() {
			os.MkdirAll(path.Join(to, hdr.Name),
				hdr.FileInfo().Mode())
		} else {
			fw, err := os.OpenFile(path.Join(to, hdr.Name),
				os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
				hdr.FileInfo().Mode())
			if err != nil {
				return err
			}
			defer fw.Close()
			_, err = io.Copy(fw, tr)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
