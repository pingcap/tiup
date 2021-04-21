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

package telemetry

import (
	fmt "fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pingcap/check"
)

type scrubSuite struct{}

var _ = check.Suite(&scrubSuite{})

func (s *scrubSuite) testScrubYaml(c *check.C, generate bool) {
	files, err := os.ReadDir("./testdata")
	c.Assert(err, check.IsNil)

	for _, f := range files {
		if !strings.HasSuffix(f.Name(), "yaml") {
			continue
		}

		c.Log("file: ", f.Name())

		data, err := os.ReadFile(filepath.Join("./testdata", f.Name()))
		c.Assert(err, check.IsNil)

		hashs := make(map[string]struct{})
		hashs["host"] = struct{}{}

		scrubed, err := ScrubYaml(data, hashs, "dummy-salt-string")
		c.Assert(err, check.IsNil)

		outName := filepath.Join("./testdata", f.Name()+".out")
		if generate {
			err = os.WriteFile(outName, scrubed, 0644)
			c.Assert(err, check.IsNil)
		} else {
			out, err := os.ReadFile(outName)
			c.Assert(err, check.IsNil)
			fmt.Printf("%s\n%s\n", outName, scrubed)
			c.Assert(scrubed, check.BytesEquals, out)
		}
	}
}

func (s *scrubSuite) TestScrubYaml(c *check.C) {
	s.testScrubYaml(c, false)
}

// alertmanager_servers will contains a nil value in the yaml.
func (s *scrubSuite) TestNilValueNotPanic(c *check.C) {
	data, err := os.ReadFile(filepath.Join("./testdata", "single/nilvalue.yaml"))
	c.Assert(err, check.IsNil)

	hashs := make(map[string]struct{})
	hashs["host"] = struct{}{}

	scrubed, err := ScrubYaml(data, hashs, "dummy-salt-string")
	c.Assert(err, check.IsNil)

	var _ = scrubed
}
