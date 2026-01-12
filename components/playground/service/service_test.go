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

	t.Run("base invalid", func(t *testing.T) {
		err := validatePortSpecs([]PortSpec{{Name: proc.PortNamePort, Base: 0}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "base")
	})
}
