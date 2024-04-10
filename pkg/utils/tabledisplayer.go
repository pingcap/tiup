package utils

import (
	"fmt"
	"io"
	"strings"
)

// TableDisplayer is a simple table displayer
type TableDisplayer struct {
	Header []string
	Rows   [][]string
	Writer io.Writer
}

// NewTableDisplayer creates a new TableDisplayer
func NewTableDisplayer(w io.Writer, header []string) *TableDisplayer {
	return &TableDisplayer{
		Writer: w,
		Header: header,
	}
}

// AddRow adds a row to the table
func (t *TableDisplayer) AddRow(row ...string) {
	// cut items if row is longer than header
	if len(row) > len(t.Header) {
		row = row[:len(t.Header)]
	}
	t.Rows = append(t.Rows, row)
}

// Display the table
func (t *TableDisplayer) Display() {
	lens := make([]int, len(t.Header))
	for i, h := range t.Header {
		lens[i] = len(h)
	}
	for _, row := range t.Rows {
		for i, r := range row {
			if len(r) > lens[i] {
				lens[i] = len(r)
			}
		}
	}

	outputs := [][]string{t.Header, {}}
	for _, item := range t.Header {
		outputs[1] = append(outputs[1], strings.Repeat("-", len(item)))
	}
	outputs = append(outputs, t.Rows...)

	for _, row := range outputs {
		for i, r := range row[:len(row)-1] {
			fmt.Fprintf(t.Writer, "%-*s", lens[i]+2, r)
		}
		fmt.Fprintf(t.Writer, "%s\n", row[len(row)-1])
	}
}
