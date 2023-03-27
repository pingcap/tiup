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
// Do not support pattern, src and dst must be dir or file path
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

	remoteArgs := make([]string, 0)
	if compress {
		remoteArgs = append(remoteArgs, "-C")
	}
	if limit > 0 {
		remoteArgs = append(remoteArgs, fmt.Sprintf("-l %d", limit))
	}
	remoteCmd := fmt.Sprintf("scp -r %s -f %s", strings.Join(remoteArgs, " "), src)

	err = session.Start(remoteCmd)
	if err != nil {
		return err
	}
	if err := ack(w); err != nil { // send an empty byte to start transfer
		return err
	}

	wd := dst
	for firstCommand := true; ; firstCommand = false {
		// parse scp command
		line, err := bufr.ReadString('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		switch line[0] {
		// ignore ACK from server
		case '\x00':
			line = line[1:]
		case '\x01':
			return fmt.Errorf("scp warning: %s", line[1:])
		case '\x02':
			return fmt.Errorf("scp error: %s", line[1:])
		}

		switch line[0] {
		case 'C':
			mode, size, name, err := parseLine(line)
			if err != nil {
				return err
			}

			fp := filepath.Join(wd, name)
			// fisrt scp command is 'C' means src is a single file
			if firstCommand {
				fp = dst
			}

			targetFile, err := os.OpenFile(fp, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode|0700)
			if err != nil {
				return err
			}
			defer targetFile.Close()
			if err := ack(w); err != nil {
				return err
			}

			// transferring data
			n, err := io.CopyN(targetFile, bufr, size)
			if err != nil {
				return err
			}
			if n < size {
				return fmt.Errorf("error downloading via scp, file size mismatch")
			}
			if err := targetFile.Sync(); err != nil {
				return err
			}
		case 'D':
			mode, _, name, err := parseLine(line)
			if err != nil {
				return err
			}

			// normally, workdir is like this
			wd = filepath.Join(wd, name)

			// fisrt scp command is 'D' means src is a dir
			if firstCommand {
				fi, err := os.Stat(dst)
				if err != nil && !os.IsNotExist(err) {
					return err
				} else if err == nil && !fi.IsDir() {
					return fmt.Errorf("%s cannot be an exist file", wd)
				} else if os.IsNotExist(err) {
					// dst is not exist, so dst is the target dir
					wd = dst
				} else {
					// dst is exist, dst/name is the target dir
					break
				}
			}

			err = utils.MkdirAll(wd, mode)
			if err != nil {
				return err
			}
		case 'E':
			wd = filepath.Dir(wd)
		default:
			return fmt.Errorf("incorrect scp command '%b', should be 'C', 'D' or 'E'", line[0])
		}

		err = ack(w)
		if err != nil {
			return err
		}
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

func parseLine(line string) (mode fs.FileMode, size int64, name string, err error) {
	words := strings.Fields(strings.TrimSuffix(line, "\n"))
	if len(words) < 3 {
		return 0, 0, "", fmt.Errorf("incorrect scp command param number: %d", len(words))
	}

	modeN, err := strconv.ParseUint(words[0][1:], 0, 32)
	if err != nil {
		return 0, 0, "", fmt.Errorf("error parsing file mode; %s", err)
	}
	mode = fs.FileMode(modeN)

	size, err = strconv.ParseInt(words[1], 10, 64)
	if err != nil {
		return 0, 0, "", err
	}

	name = words[2]

	return
}
