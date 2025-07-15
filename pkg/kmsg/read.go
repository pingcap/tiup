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

package kmsg

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"syscall"
)

// Read reads all available kernel messages
func Read() ([]*Msg, error) {
	fd, err := syscall.Open(kmsgFile, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(fd)

	result := make([]*Msg, 0)
	msgChan, sucChan, errChan := readKMsg(fd)

	for {
		select {
		case err := <-errChan:
			return nil, err
		case <-sucChan: // finished
			return result, nil
		case msg := <-msgChan:
			result = append(result, msg)
		}
	}
}

func readKMsg(fd int) (<-chan *Msg, <-chan bool, <-chan error) {
	msgChan := make(chan *Msg)
	sucChan := make(chan bool, 1)
	errChan := make(chan error, 1)

	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := syscall.Read(fd, buf)
			if err != nil {
				if err == io.EOF || err == syscall.EAGAIN {
					// complete
					sucChan <- true
					return
				}
				if err == syscall.EPIPE {
					// read failed, retry
					continue
				}
				errChan <- err
				return
			}

			msg, err := parseMsg(string(buf[:n]))
			if err != nil {
				errChan <- err
				return
			}
			msgChan <- msg
		}
	}()

	return msgChan, sucChan, errChan
}

func parseMsg(msg string) (*Msg, error) {
	result := &Msg{}
	fields := strings.Split(msg, ";")
	if len(fields) < 2 {
		return nil, fmt.Errorf("incorrect kernel log format")
	}
	result.Message = strings.TrimSuffix(fields[1], "\n")

	prefix := strings.Split(fields[0], ",")
	if len(prefix) < 3 {
		return nil, fmt.Errorf("incorrect kernel log prefix format")
	}

	priority, err := strconv.Atoi(prefix[0])
	if err != nil {
		return result, fmt.Errorf("incorrect kernel log priority %s", prefix[0])
	}
	result.Facility = decodeFacility(priority)
	result.Severity = decodeSeverity(priority)

	seq, err := strconv.Atoi(prefix[1])
	if err != nil {
		return result, fmt.Errorf("incorrect kernel log sequence %s", prefix[1])
	}
	result.Sequence = seq

	ts, err := strconv.Atoi(prefix[2])
	if err != nil {
		return result, fmt.Errorf("incorrect kernel log timestamp %s", prefix[2])
	}
	result.Timestamp = ts

	return result, nil
}
