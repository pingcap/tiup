package scripts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithRetention(t *testing.T) {
	var val string
	ps := NewPrometheusScript("127.0.0.1", "/tidb", "/tidb/data", "/tidb/log")
	val = ps.WithRetention("-1d").Retention
	assert.EqualValues(t, "30d", val)

	val = ps.WithRetention("0d").Retention
	assert.EqualValues(t, "30d", val)

	val = ps.WithRetention("01d").Retention
	assert.EqualValues(t, "30d", val)

	val = ps.WithRetention("1dd").Retention
	assert.EqualValues(t, "30d", val)

	val = ps.WithRetention("*1d").Retention
	assert.EqualValues(t, "30d", val)

	val = ps.WithRetention("1d ").Retention
	assert.EqualValues(t, "30d", val)

	val = ps.WithRetention("ddd").Retention
	assert.EqualValues(t, "30d", val)

	val = ps.WithRetention("60d").Retention
	assert.EqualValues(t, "60d", val)

	val = ps.WithRetention("999d").Retention
	assert.EqualValues(t, "999d", val)
}
