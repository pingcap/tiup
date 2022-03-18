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

package utils

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/otiai10/copy"
	"github.com/pingcap/errors"
)

var (
	fileLocks = make(map[string]*sync.Mutex)
	filesLock = sync.Mutex{}
)

// IsSymExist check whether a symbol link is exist
func IsSymExist(path string) bool {
	_, err := os.Lstat(path)
	return !os.IsNotExist(err)
}

// IsExist check whether a path is exist
func IsExist(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// IsNotExist check whether a path is not exist
func IsNotExist(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// IsEmptyDir check whether a path is an empty directory
func IsEmptyDir(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// IsExecBinary check whether a path is a valid executable
func IsExecBinary(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir() && info.Mode()&0111 == 0111
}

// IsSubDir returns if sub is a sub directory of parent
func IsSubDir(parent, sub string) bool {
	up := ".." + string(os.PathSeparator)

	rel, err := filepath.Rel(parent, sub)
	if err != nil {
		return false
	}
	if !strings.HasPrefix(rel, up) && rel != ".." {
		return true
	}
	return false
}

// Tar compresses the folder to tarball with gzip
func Tar(writer io.Writer, from string) error {
	compressW := gzip.NewWriter(writer)
	defer compressW.Close()
	tarW := tar.NewWriter(compressW)
	defer tarW.Close()

	return filepath.Walk(from, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, _ := tar.FileInfoHeader(info, "")
		header.Name, _ = filepath.Rel(from, path)
		// skip "."
		if header.Name == "." {
			return nil
		}

		err = tarW.WriteHeader(header)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fd, err := os.Open(path)
			if err != nil {
				return err
			}
			defer fd.Close()
			_, err = io.Copy(tarW, fd)
			return err
		}
		return nil
	})
}

// Untar decompresses the tarball
func Untar(reader io.Reader, to string) error {
	gr, err := gzip.NewReader(reader)
	if err != nil {
		return errors.Trace(err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	decFile := func(hdr *tar.Header) error {
		file := path.Join(to, hdr.Name)
		err := os.MkdirAll(filepath.Dir(file), 0755)
		if err != nil {
			return err
		}
		fw, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
		if err != nil {
			return errors.Trace(err)
		}
		defer fw.Close()

		_, err = io.Copy(fw, tr)
		return errors.Trace(err)
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Trace(err)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path.Join(to, hdr.Name), hdr.FileInfo().Mode()); err != nil {
				return errors.Trace(err)
			}
		case tar.TypeSymlink:
			if err = os.Symlink(hdr.Linkname, filepath.Join(to, hdr.Name)); err != nil {
				return errors.Trace(err)
			}
		default:
			if err := decFile(hdr); err != nil {
				return errors.Trace(err)
			}
		}
	}
	return nil
}

// Copy copies a file or directory from src to dst
func Copy(src, dst string) error {
	// check if src is a directory
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		// use copy.Copy to copy a directory
		return copy.Copy(src, dst)
	}

	// for regular files
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	err = out.Close()
	if err != nil {
		return err
	}

	err = os.Chmod(dst, fi.Mode())
	if err != nil {
		return err
	}

	// Make sure the created dst's modify time is newer (at least equal) than src
	// this is used to workaround github action virtual filesystem
	ofi, err := os.Stat(dst)
	if err != nil {
		return err
	}
	if fi.ModTime().After(ofi.ModTime()) {
		return os.Chtimes(dst, fi.ModTime(), fi.ModTime())
	}
	return nil
}

// Move moves a file from src to dst, this is done by copying the file and then
// delete the old one. Use os.Rename() to rename file within the same filesystem
// instead this, it's more lightweight but can not be used across devices.
func Move(src, dst string) error {
	if err := Copy(src, dst); err != nil {
		return errors.Trace(err)
	}
	return errors.Trace(os.RemoveAll(src))
}

// CreateDir creates the directory if it not exists.
func CreateDir(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(path, 0755)
		}
		return err
	}
	return nil
}

// Checksum returns the sha1 sum of target file
func Checksum(file string) (string, error) {
	tarball, err := os.OpenFile(file, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}
	defer tarball.Close()

	sha1Writter := sha1.New()
	if _, err := io.Copy(sha1Writter, tarball); err != nil {
		return "", err
	}

	checksum := hex.EncodeToString(sha1Writter.Sum(nil))
	return checksum, nil
}

// TailN try get the latest n line of the file.
func TailN(fname string, n int) (lines []string, err error) {
	file, err := os.Open(fname)
	if err != nil {
		return nil, errors.AddStack(err)
	}
	defer file.Close()

	estimateLineSize := 1024

	stat, err := os.Stat(fname)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	start := int(stat.Size()) - n*estimateLineSize
	if start < 0 {
		start = 0
	}

	_, err = file.Seek(int64(start), 0 /*means relative to the origin of the file*/)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return
}

func fileLock(path string) *sync.Mutex {
	filesLock.Lock()
	defer filesLock.Unlock()

	if _, ok := fileLocks[path]; !ok {
		fileLocks[path] = &sync.Mutex{}
	}

	return fileLocks[path]
}

// SaveFileWithBackup will backup the file before save it.
// e.g., backup meta.yaml as meta-2006-01-02T15:04:05Z07:00.yaml
// backup the files in the same dir of path if backupDir is empty.
func SaveFileWithBackup(path string, data []byte, backupDir string) error {
	fileLock(path).Lock()
	defer fileLock(path).Unlock()

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
		timestr := time.Now().Format(time.RFC3339Nano)
		p := strings.Split(base, ".")
		if len(p) == 1 {
			backupName = base + "-" + timestr
		} else {
			backupName = strings.Join(p[0:len(p)-1], ".") + "-" + timestr + "." + p[len(p)-1]
		}

		backupData, err := os.ReadFile(path)
		if err != nil {
			return errors.AddStack(err)
		}

		var backupPath string
		if backupDir != "" {
			backupPath = filepath.Join(backupDir, backupName)
		} else {
			backupPath = filepath.Join(dir, backupName)
		}
		err = os.WriteFile(backupPath, backupData, 0644)
		if err != nil {
			return errors.AddStack(err)
		}
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return errors.AddStack(err)
	}

	return nil
}
