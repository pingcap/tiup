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

package verbose

import (
	"fmt"
	"os"
	"strings"
)

var verbose bool

func init() {
	v := strings.ToLower(os.Getenv("TIUP_VERBOSE"))
	verbose = v == "1" || v == "enable"
}

// Log logs verbose messages
func Log(format string, args ...interface{}) {
	if !verbose {
		return
	}
	fmt.Println("Verbose:", fmt.Sprintf(format, args...))
}
