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
)

// Ref: https://www.kernel.org/doc/Documentation/ABI/testing/dev-kmsg
// The kmsg lines have prefix of the following format:
// | priority | sequence | monotonic timestamp | flag | message
//          6 ,      339 ,             5140900 ,    - ; NET: Registered protocol family 10
//         30 ,      340 ,             5690716 ,    - ; udevd[80]: starting version 181
// where the flag is not necessary for us, so we only parse the prefix
// for the first 3 fields: priority, sequence and timestamp

// the device to read kernel log from
const kmsgFile = "/dev/kmsg"

const severityMask = 0x07
const facilityMask = 0xf8

// Severity is part of the log priority
type Severity int

const (
	// From /usr/include/sys/syslog.h.
	// These are the same on Linux, BSD, and OS X.
	LOG_EMERG Severity = iota
	LOG_ALERT
	LOG_CRIT
	LOG_ERR
	LOG_WARNING
	LOG_NOTICE
	LOG_INFO
	LOG_DEBUG
)

// String implements the string interface
func (p Severity) String() string {
	return []string{
		"emerg", "alert", "crit", "err",
		"warning", "notice", "info", "debug",
	}[p]
}

// Facility is part of the log priority
type Facility int

const (
	// From /usr/include/sys/syslog.h.
	// These are the same up to LOG_FTP on Linux, BSD, and OS X.
	LOG_KERN Facility = iota << 3
	LOG_USER
	LOG_MAIL
	LOG_DAEMON
	LOG_AUTH
	LOG_SYSLOG
	LOG_LPR
	LOG_NEWS
	LOG_UUCP
	LOG_CRON
	LOG_AUTHPRIV
	LOG_FTP
	_ // unused
	_ // unused
	_ // unused
	_ // unused
	LOG_LOCAL0
	LOG_LOCAL1
	LOG_LOCAL2
	LOG_LOCAL3
	LOG_LOCAL4
	LOG_LOCAL5
	LOG_LOCAL6
	LOG_LOCAL7
)

// String implements the string interface
func (p Facility) String() string {
	return []string{
		"kern", "user", "mail", "daemon",
		"auth", "syslog", "lpr", "news",
		"uucp", "cron", "authpriv", "ftp",
		"", "", "", "",
		"local0", "local1", "local2", "local3",
		"local4", "local5", "local6", "local7",
	}[p]
}

func decodeSeverity(p int) Severity {
	return Severity(p) & severityMask
}

func decodeFacility(p int) Facility {
	return Facility(p) & facilityMask
}

// Msg is the type of kernel message
type Msg struct {
	Severity  Severity
	Facility  Facility
	Sequence  int // Sequence is the 64 bit message sequence number
	Timestamp int // Timestamp is the monotonic timestamp in microseconds
	Message   string
}

// String implements the string interface
func (m *Msg) String() string {
	return fmt.Sprintf("%s:%s: [%.6f] %s", m.Facility, m.Severity, float64(m.Timestamp)/1e6, m.Message)
}
