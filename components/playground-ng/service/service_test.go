package service

import (
	"testing"

	"github.com/pingcap/tiup/components/playground-ng/proc"
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

	t.Run("multiple FromConfigPort", func(t *testing.T) {
		err := validatePortSpecs([]PortSpec{
			{Name: proc.PortNamePort, Base: 1, FromConfigPort: true},
			{Name: "service", Base: 2, FromConfigPort: true},
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

func TestRegister_AllowModifyPortRequiresFromConfigPort(t *testing.T) {
	err := Register(Spec{
		ServiceID: "test-service-allow-port-without-from-config-port",
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return nil, nil
		},
		Catalog: Catalog{
			AllowModifyPort: true,
			Ports: []PortSpec{
				{Name: proc.PortNamePort, Base: 4000},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "FromConfigPort")
}

func TestRegister_AllowModifyPortRequiresPorts(t *testing.T) {
	err := Register(Spec{
		ServiceID: "test-service-allow-port-without-ports",
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return nil, nil
		},
		Catalog: Catalog{
			AllowModifyPort: true,
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "AllowModifyPort")
}

func TestRegister_DefaultPortRequiresAllowModifyPort(t *testing.T) {
	err := Register(Spec{
		ServiceID: "test-service-default-port-without-allow-port",
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return nil, nil
		},
		Catalog: Catalog{
			DefaultPort: 4000,
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "DefaultPort")
}

func TestRegister_DefaultPortFromRequiresAllowModifyPort(t *testing.T) {
	err := Register(Spec{
		ServiceID: "test-service-default-port-from-without-allow-port",
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return nil, nil
		},
		Catalog: Catalog{
			DefaultPortFrom: proc.ServicePD,
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "DefaultPortFrom")
}

func TestRegister_DefaultBinPathFromRequiresAllowModifyBinPath(t *testing.T) {
	err := Register(Spec{
		ServiceID: "test-service-default-binpath-from-without-allow-binpath",
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return nil, nil
		},
		Catalog: Catalog{
			DefaultBinPathFrom: proc.ServicePD,
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "DefaultBinPathFrom")
}

func TestRegister_DefaultConfigPathFromRequiresAllowModifyConfig(t *testing.T) {
	err := Register(Spec{
		ServiceID: "test-service-default-config-from-without-allow-config",
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return nil, nil
		},
		Catalog: Catalog{
			DefaultConfigPathFrom: proc.ServicePD,
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "DefaultConfigPathFrom")
}

func TestRegister_DefaultHostFromRequiresAllowModifyHost(t *testing.T) {
	err := Register(Spec{
		ServiceID: "test-service-default-host-from-without-allow-host",
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return nil, nil
		},
		Catalog: Catalog{
			DefaultHostFrom: proc.ServicePD,
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "DefaultHostFrom")
}

func TestRegister_DefaultTimeoutRequiresAllowModifyTimeout(t *testing.T) {
	err := Register(Spec{
		ServiceID: "test-service-default-timeout-without-allow-timeout",
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return nil, nil
		},
		Catalog: Catalog{
			DefaultTimeout: 1,
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "DefaultTimeout")
}

func TestFillPlannedPorts(t *testing.T) {
	t.Run("fills standard ports and alias", func(t *testing.T) {
		allocCalls := make([]string, 0, 2)
		alloc := func(host string, base int) (int, error) {
			allocCalls = append(allocCalls, host)
			return base, nil
		}

		plan := proc.ServicePlan{
			Shared: proc.ServiceSharedPlan{Host: "127.0.0.1"},
		}
		cfg := proc.Config{Port: 5000}
		specs := []PortSpec{
			{Name: proc.PortNamePort, Base: 4000, FromConfigPort: true},
			{Name: proc.PortNameStatusPort, Base: 10080, Host: "0.0.0.0"},
			{Name: "service", Base: 3930},
			{Name: "alias", AliasOf: "service"},
		}
		require.NoError(t, FillPlannedPorts(alloc, cfg, &plan, specs))

		require.Equal(t, 5000, plan.Shared.Port)
		require.Equal(t, 10080, plan.Shared.StatusPort)
		require.Equal(t, map[string]int{
			proc.PortNamePort:       5000,
			proc.PortNameStatusPort: 10080,
			"service":               3930,
			"alias":                 3930,
		}, plan.Shared.Ports)
		require.Equal(t, []string{"127.0.0.1", "0.0.0.0", "127.0.0.1"}, allocCalls)
	})

	t.Run("missing host", func(t *testing.T) {
		plan := proc.ServicePlan{}
		err := FillPlannedPorts(func(host string, base int) (int, error) { return base, nil }, proc.Config{}, &plan, []PortSpec{
			{Name: proc.PortNamePort, Base: 4000},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "planned host is empty")
	})
}

func TestAllocPortsForNewProc_NormalizesAndValidatesHost(t *testing.T) {
	serviceID := proc.ServiceID("test-service-alloc-ports-for-new-proc")
	require.NoError(t, Register(Spec{
		ServiceID: serviceID,
		NewProc: func(rt ControllerRuntime, params NewProcParams) (proc.Process, error) {
			return nil, nil
		},
	}))

	shared, err := allocPortsForNewProc(serviceID, NewProcParams{Host: " 127.0.0.1 "}, 0)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", shared.Host)

	_, err = allocPortsForNewProc(serviceID, NewProcParams{Host: "  "}, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "host is empty")
}
