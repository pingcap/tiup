package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetFreePort(t *testing.T) {
	expected := 22334
	port, err := getFreePort("127.0.0.1", expected)
	require.NoError(t, err)
	require.Equal(t, expected, port, "expect port %d", expected)

	port, err = getFreePort("127.0.0.1", expected)
	require.NoError(t, err)
	require.NotEqual(t, expected, port, "should not return same port twice")
}
