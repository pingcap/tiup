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
	IntSlice     []int                `yaml:"ints,omitempty"`
	StrSlice     []string             `yaml:"strs,omitempty" validate:"strs:editable"`
	MapSlice     []map[string]any     `yaml:"maps,omitempty" validate:"maps:ignore"`
	StrElem      string               `yaml:"stre" validate:"editable"`
	StrElem2     string               `yaml:"str2,omitempty" validate:"str2:expandable"`
	StructSlice1 []sampleDataElem     `yaml:"slice1" validate:"slice1:editable"`
	StructSlice2 []sampleDataElem     `yaml:"slice2,omitempty"`
	StructSlice3 []sampleDataEditable `yaml:"slice3,omitempty" validate:"slice3:editable"`
}

type sampleDataElem struct {
	StrElem1       string         `yaml:"str1" validate:"str1:editable"`
	StrElem2       string         `yaml:"str2,omitempty" validate:"str2:editable"`
	IntElem        int            `yaml:"int"`
	InterfaceElem  any            `yaml:"interface,omitempty" validate:"interface:editable"`
	InterfaceSlice map[string]any `yaml:"mapslice,omitempty" validate:"mapslice:editable"`
}

type sampleDataEditable struct {
	StrElem1 string `yaml:"str1" validate:"str1:editable"`
	StrElem2 string `yaml:"str2,omitempty" validate:"str2:editable"`
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

	// add editable element (without specifying alias)
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
	c.Assert(err.Error(), Equals, "immutable field changed: added IntSlice.3 with value '14'")
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
	c.Assert(err.Error(), Equals, "immutable field changed: slice1.1.IntElem changed from '42' to '43'")

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
	c.Assert(err.Error(), Equals, "immutable field changed: added slice1.2.IntElem with value '42'")

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
	c.Assert(err.Error(), Equals, "immutable field changed: removed slice1.1.IntElem with value '42'")
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
	c.Assert(err.Error(), Equals, "immutable field changed: StructSlice2.1.IntElem changed from '42' to '43'")

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
	c.Assert(err.Error(), Equals, "immutable field changed: added StructSlice2.2.str1 with value 'strv31', added StructSlice2.2.str2 with value 'strv32'")

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
	c.Assert(err.Error(), Equals, "immutable field changed: removed StructSlice2.1.str1 with value 'strv12', removed StructSlice2.1.str2 with value 'strv22', removed StructSlice2.1.IntElem with value '42', removed StructSlice2.1.interface with value '12'")
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

func (d *diffSuite) TestValidateSpecDiffType(c *C) {
	var d1 sampleDataMeta
	var d2 sampleDataMeta
	var err error

	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice3:
  - key0: 0
`), &d1)
	c.Assert(err, IsNil)

	// Modify key in editable map, with the same type
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice3:
  - key0: 1
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)

	// Modify key in editable map, with value type changed
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice3:
  - key0: 2.0
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)

	// Modify key in editable map, with value type changed
	err = yaml.Unmarshal([]byte(`
ints: [11, 12, 13]
slice3:
  - key0: sss
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)
}

func (d *diffSuite) TestValidateSpecDiffExpandable(c *C) {
	var d1 sampleDataMeta
	var d2 sampleDataMeta
	var err error

	err = yaml.Unmarshal([]byte(`
str2: "/ssd0/tiflash,/ssd1/tiflash"
`), &d1)
	c.Assert(err, IsNil)

	// Expand path
	err = yaml.Unmarshal([]byte(`
str2: "/ssd0/tiflash,/ssd1/tiflash,/ssd2/tiflash"
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)

	// Expand path with non-sorted paths
	err = yaml.Unmarshal([]byte(`
str2: "/ssd0/tiflash,/ssd2/tiflash,/ssd1/tiflash"
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, IsNil)

	// Expand path with non-sorted paths. Changing the first path is not allowed.
	err = yaml.Unmarshal([]byte(`
str2: "/ssd1/tiflash,/ssd0/tiflash,/ssd2/tiflash"
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, NotNil)

	// Shirnking paths is not allowed
	err = yaml.Unmarshal([]byte(`
str2: "/ssd0/tiflash"
`), &d2)
	c.Assert(err, IsNil)
	err = ValidateSpecDiff(d1, d2)
	c.Assert(err, NotNil)
}
