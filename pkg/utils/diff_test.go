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
	. "github.com/pingcap/check"
	"gopkg.in/yaml.v2"
)

type diffSuite struct {
}

var _ = Suite(&diffSuite{})

type sampleDataMeta struct {
	IntSlice     []int                    `yaml:"ints,omitempty"`
	StrSlice     []string                 `yaml:"strs,omitempty" validate:"editable"`
	MapSlice     []map[string]interface{} `yaml:"maps,omitempty" validate:"ignore"`
	StrElem      string                   `yaml:"stre" validate:"editable"`
	StructSlice1 []sampleDataElem         `yaml:"slice1" validate:"editable"`
	StructSlice2 []sampleDataElem         `yaml:"slice2,omitempty"`
	StructSlice3 []sampleDataEditable     `yaml:"slice3,omitempty" validate:"editable"`
}

type sampleDataElem struct {
	StrElem1       string                   `yaml:"str1" validate:"editable"`
	StrElem2       string                   `yaml:"str2,omitempty" validate:"editable"`
	IntElem        int                      `yaml:"int"`
	InterfaceElem  interface{}              `yaml:"interface,omitempty" validate:"editable"`
	InterfaceSlice []map[string]interface{} `yaml:"mapslice,omitempty" validate:"editable"`
}

type sampleDataEditable struct {
	StrElem1 string `yaml:"str1" validate:"editable"`
	StrElem2 string `yaml:"str2,omitempty" validate:"editable"`
}

func (d *diffSuite) TestValidateSpecDiff1(c *C) {
	var d1 sampleDataMeta
	var d2 sampleDataMeta
	var err error

	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
strs:
  - str1
  - "str2"
`), &d1)
	c.Assert(err, IsNil)
	// unchanged
	err = ValidateSpecDiff(d1, d1)
	c.Assert(err, IsNil)

	// swap element order
	err = yaml.Unmarshal([]byte(`
ints: [11, 13, 12]
strs:
  - str2
  - "str1"
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)

	// add editable element
	err = yaml.Unmarshal([]byte(`
ints: [11, 13, 12]
strs:
  - "str1"
  - str2
stre: "test1.3"
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)

	// add item to immutable element
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13, 14]
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "immutable field changed: (create) IntSlice.3 '<nil>' -> '14'")
}

func (d *diffSuite) TestValidateSpecDiff2(c *C) {
	var d1 sampleDataMeta
	var d2 sampleDataMeta
	var err error

	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice1:
  - str1: strv11
    str2: strv21
    int: 42
    interface: 11
  - str1: strv12
    str2: strv22
    int: 42
    interface: "12"
`), &d1)
	c.Assert(err, IsNil)

	// change editable filed of item in editable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice1:
  - str1: strv11
    str2: strv233
    int: 42
    interface: 11
  - str1: strv12
    str2: strv22
    int: 42
    interface: "12"
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)

	// change immutable filed of item in editable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice1:
  - str1: strv11
    str2: strv21
    int: 42
    interface: 11
  - str1: strv12
    str2: strv22
    int: 43
    interface: "12"
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "immutable field changed: (update) editable.1.IntElem '42' -> '43'")

	// Add item with immutable field to editable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice1:
  - str1: strv11
    str2: strv21
    int: 42
    interface: 11
  - str1: strv12
    str2: strv22
    int: 42
    interface: "12"
  - str1: strv13
    str2: strv23
    int: 42
    interface: "13"
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "immutable field changed: (create) editable.2.IntElem '<nil>' -> '42'")

	// Delete item with immutable field from editable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice1:
  - str1: strv11
    str2: strv21
    int: 42
    interface: 11
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "immutable field changed: (delete) editable.1.IntElem '42' -> '<nil>'")
}

func (d *diffSuite) TestValidateSpecDiff3(c *C) {
	var d1 sampleDataMeta
	var d2 sampleDataMeta
	var err error

	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice2:
  - str1: strv11
    str2: strv21
    int: 42
    interface: 11
  - str1: strv12
    str2: strv22
    int: 42
    interface: "12"
`), &d1)
	c.Assert(err, IsNil)

	// change editable filed of item in immutable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice2:
  - str1: strv11
    str2: strv233
    int: 42
    interface: 11
  - str1: strv12
    str2: strv22
    int: 42
    interface: "12"
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)

	// change immutable filed of item in immutable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice2:
  - str1: strv11
    str2: strv21
    int: 42
    interface: 11
  - str1: strv12
    str2: strv22
    int: 43
    interface: "12"
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "immutable field changed: (update) StructSlice2.1.IntElem '42' -> '43'")

	// Add item to immutable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice2:
  - str1: strv11
    str2: strv21
    int: 42
    interface: 11
  - str1: strv12
    str2: strv22
    int: 42
    interface: "12"
  - str1: strv31
    str2: strv32
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "immutable field changed: (create) StructSlice2.2.editable '<nil>' -> 'strv31', (create) StructSlice2.2.editable '<nil>' -> 'strv32'")

	// Remove item from immutable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice2:
  - str1: strv11
    str2: strv21
    int: 42
    interface: 11
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "immutable field changed: (delete) StructSlice2.1.editable 'strv12' -> '<nil>', (delete) StructSlice2.1.editable 'strv22' -> '<nil>', (delete) StructSlice2.1.IntElem '42' -> '<nil>', (delete) StructSlice2.1.editable '12' -> '<nil>'")
}

func (d *diffSuite) TestValidateSpecDiff4(c *C) {
	var d1 sampleDataMeta
	var d2 sampleDataMeta
	var err error

	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice3:
  - str1: strv11
    str2: strv21
`), &d1)
	c.Assert(err, IsNil)

	// Add item with only editable fields to editable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice3:
  - str1: strv11
    str2: strv21
  - str1: strv21
    str2: strv22
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)

	// Remove item with only editable fields from editable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice3:
  - str1: strv21
    str2: strv22
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)
}

func (d *diffSuite) TestValidateSpecDiff5(c *C) {
	var d1 sampleDataMeta
	var d2 sampleDataMeta
	var err error

	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice1:
  - str1: strv11
    str2: strv21
    interslice:
      - key0: 0
  - str1: strv12
    str2: strv22
slice2:
  - str1: strv13
    str2: strv14
    interslice:
      - key0: 0
`), &d1)
	c.Assert(err, IsNil)

	// Modify item of editable slice in item of editable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice1:
  - str1: strv11
    str2: strv21
    interslice:
      - key0: 0.1
  - str1: strv12
    str2: strv22
    interslice:
      - key1: 1
      - key2: "v2"
slice2:
  - str1: strv13
    str2: strv14
    interslice:
      - key0: 0
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)

	// Modify item of editable slice in item of editable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice1:
  - str1: strv11
    str2: strv21
    interslice:
      - key0: 0
  - str1: strv12
    str2: strv22
    interslice:
      - key1: 1
      - key2: "v2"
slice2:
  - str1: strv13
    str2: strv14
    interslice:
      - key0: 0.2
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)

	// Add item to editable slice to item of editable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice1:
  - str1: strv11
    str2: strv21
    interslice:
      - key0: 0
  - str1: strv12
    str2: strv22
    interslice:
      - key1: 1
      - key2: "v2"
slice2:
  - str1: strv13
    str2: strv14
    interslice:
      - key0: 0
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)

	// Add item to editable slice to item of immutable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice1:
  - str1: strv11
    str2: strv21
    interslice:
      - key0: 0
  - str1: strv12
    str2: strv22
slice2:
  - str1: strv13
    str2: strv14
    interslice:
      - key0: 0
      - key3: 3.0
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)
}

func (d *diffSuite) TestValidateSpecDiff6(c *C) {
	var d1 sampleDataMeta
	var d2 sampleDataMeta
	var err error

	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
maps:
  - key0: 0
  - dot.key1: 1
  - dotkey.subkey.1: "1"
`), &d1)
	c.Assert(err, IsNil)

	// Modify key without dot in name, in ignorable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
maps:
  - key0: 1
  - dot.key1: 1
  - dotkey.subkey.1: "1"
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)

	// Modify key with one dot in name, in ignorable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
maps:
  - key0: 0
  - dot.key1: 11
  - dotkey.subkey.1: "1"
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)

	// Modify key with two dots and number in name, in ignorable slice
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
maps:
  - key0: 0
  - dot.key1: 1
  - dotkey.subkey.1: "12"
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)
}
