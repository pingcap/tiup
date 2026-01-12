package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePortSpecs(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		require.NoError(t, validatePortSpecs([]PortSpec{
			{Name: "port", Base: 4000, FromConfigPort: true},
			{Name: "statusPort", AliasOf: "port"},
		}))
	})

	t.Run("duplicate name", func(t *testing.T) {
		err := validatePortSpecs([]PortSpec{
			{Name: "port", Base: 1},
			{Name: "port", Base: 2},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate")
	})

	t.Run("alias unknown", func(t *testing.T) {
		err := validatePortSpecs([]PortSpec{
			{Name: "statusPort", AliasOf: "port"},
			{Name: "port", Base: 1},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "aliases unknown")
	})

	t.Run("base invalid", func(t *testing.T) {
		err := validatePortSpecs([]PortSpec{{Name: "port", Base: 0}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "base")
	})
}
