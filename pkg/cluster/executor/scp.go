// Copyright 2021 PingCAP, Inc.
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

package executor

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/crypto/ssh"
)

// ScpDownload downloads a file from remote with SCP
// The implementation is partially inspired by github.com/dtylman/scp
func ScpDownload(session *ssh.Session, client *ssh.Client, src, dst string, limit int, compress bool) error {
	r, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	bufr := bufio.NewReader(r)

	w, err := session.StdinPipe()
	if err != nil {
		return err
	}

	copyF := func() error {
		// parse SCP command
		line, _, err := bufr.ReadLine()
		if err != nil {
			return err
		}
		if line[0] != byte('C') {
			return fmt.Errorf("incorrect scp command '%b', should be 'C'", line[0])
		}

		mode, err := strconv.ParseUint(string(line[1:5]), 0, 32)
		if err != nil {
			return fmt.Errorf("error parsing file mode; %s", err)
		}

		// prepare dst file
		targetPath := filepath.Dir(dst)
		if err := utils.CreateDir(targetPath); err != nil {
			return err
		}
		targetFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, fs.FileMode(mode))
		if err != nil {
			return err
		}
		defer targetFile.Close()

		size, err := strconv.Atoi(strings.Fields(string(line))[1])
		if err != nil {
			return err
		}

		if err := ack(w); err != nil {
			return err
		}

		// transferring data
		n, err := io.CopyN(targetFile, bufr, int64(size))
		if err != nil {
			return err
		}
		if n < int64(size) {
			return fmt.Errorf("error downloading via scp, file size mismatch")
		}
		if err := targetFile.Sync(); err != nil {
			return err
		}

		return ack(w)
	}

	copyErrC := make(chan error, 1)
	go func() {
		defer w.Close()
		copyErrC <- copyF()
	}()

	remoteArgs := make([]string, 0)
	if compress {
		remoteArgs = append(remoteArgs, "-C")
	}
	if limit > 0 {
		remoteArgs = append(remoteArgs, fmt.Sprintf("-l %d", limit))
	}
	remoteCmd := fmt.Sprintf("scp %s -f %s", strings.Join(remoteArgs, " "), src)

	err = session.Start(remoteCmd)
	if err != nil {
		return err
	}
	if err := ack(w); err != nil { // send an empty byte to start transfer
		return err
	}

	err = <-copyErrC
	if err != nil {
		return err
	}
	return session.Wait()
}

func ack(w io.Writer) error {
	msg := []byte("\x00")
	n, err := w.Write(msg)
	if err != nil {
		return fmt.Errorf("fail to send response to remote: %s", err)
	}
	if n < len(msg) {
		return fmt.Errorf("fail to send response to remote, size mismatch")
	}
	return nil
}
