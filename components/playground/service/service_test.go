package service

import (
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/stretchr/testify/require"
)

func TestValidatePortSpecs(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		require.NoError(t, validatePortSpecs([]PortSpec{
			{Name: proc.PortNamePort, Base: 4000, FromConfigPort: true},
			{Name: proc.PortNameStatusPort, AliasOf: proc.PortNamePort},
		}))
	})

	t.Run("duplicate name", func(t *testing.T) {
		err := validatePortSpecs([]PortSpec{
			{Name: proc.PortNamePort, Base: 1},
			{Name: proc.PortNamePort, Base: 2},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate")
	})

	t.Run("alias unknown", func(t *testing.T) {
		err := validatePortSpecs([]PortSpec{
			{Name: proc.PortNameStatusPort, AliasOf: proc.PortNamePort},
			{Name: proc.PortNamePort, Base: 1},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "aliases unknown")
	})

	t.Run("alias with base", func(t *testing.T) {
		err := validatePortSpecs([]PortSpec{
			{Name: proc.PortNamePort, Base: 1},
			{Name: proc.PortNameStatusPort, AliasOf: proc.PortNamePort, Base: 2},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "base must be 0")
	})

	t.Run("alias with host", func(t *testing.T) {
		err := validatePortSpecs([]PortSpec{
			{Name: proc.PortNamePort, Base: 1},
			{Name: proc.PortNameStatusPort, AliasOf: proc.PortNamePort, Host: "127.0.0.1"},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "host must be empty")
	})

	t.Run("alias with FromConfigPort", func(t *testing.T) {
		err := validatePortSpecs([]PortSpec{
			{Name: proc.PortNamePort, Base: 1},
			{Name: proc.PortNameStatusPort, AliasOf: proc.PortNamePort, FromConfigPort: true},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "FromConfigPort")
	})

	t.Run("host with spaces", func(t *testing.T) {
		err := validatePortSpecs([]PortSpec{
			{Name: proc.PortNamePort, Base: 1, Host: " 0.0.0.0"},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "leading/trailing spaces")
	})

	t.Run("base invalid", func(t *testing.T) {
		err := validatePortSpecs([]PortSpec{{Name: proc.PortNamePort, Base: 0}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "base")
	})
}
