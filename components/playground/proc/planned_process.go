package proc

import (
	"strings"

	"github.com/pingcap/errors"
)

// NewProcessFromPlan creates a proc.Process instance from a planned service entry.
//
// It is a centralized mapping from service IDs to proc implementations, so the
// controller does not need to maintain a parallel "service -> proc type" switch.
//
// NOTE: This function does not validate ports/dirs beyond what is necessary for
// process construction. Planner and controller are responsible for those checks.
func NewProcessFromPlan(plan ServicePlan, info ProcessInfo, shOpt SharedOptions, dataDir string) (Process, error) {
	serviceID := info.Service
	if serviceID == "" {
		return nil, errors.New("planned service id is empty")
	}

	if pid := strings.TrimSpace(plan.ServiceID); pid != "" && ServiceID(pid) != serviceID {
		return nil, errors.Errorf("planned service id mismatch: plan=%q info=%q", plan.ServiceID, serviceID)
	}

	name := ""
	if n := info.Name(); n != "" {
		name = n
	} else if n := strings.TrimSpace(plan.Name); n != "" {
		name = n
	} else {
		name = serviceID.String()
	}

	switch serviceID {
	case ServicePD, ServicePDAPI, ServicePDTSO, ServicePDScheduling, ServicePDRouter, ServicePDResourceManager:
		if plan.PD == nil {
			return nil, errors.Errorf("missing pd plan for %s", name)
		}
		return &PDInstance{ShOpt: shOpt, Plan: *plan.PD, ProcessInfo: info}, nil
	case ServiceTiKV:
		if plan.TiKV == nil {
			return nil, errors.Errorf("missing tikv plan for %s", name)
		}
		return &TiKVInstance{ShOpt: shOpt, Plan: *plan.TiKV, ProcessInfo: info}, nil
	case ServiceTiDB, ServiceTiDBSystem:
		if plan.TiDB == nil {
			return nil, errors.Errorf("missing tidb plan for %s", name)
		}
		return &TiDBInstance{ShOpt: shOpt, Plan: *plan.TiDB, TiProxyCertDir: dataDir, ProcessInfo: info}, nil
	case ServiceTiKVWorker:
		if plan.TiKVWorker == nil {
			return nil, errors.Errorf("missing tikv-worker plan for %s", name)
		}
		return &TiKVWorkerInstance{ShOpt: shOpt, Plan: *plan.TiKVWorker, ProcessInfo: info}, nil
	case ServiceTiFlash, ServiceTiFlashWrite, ServiceTiFlashCompute:
		if plan.TiFlash == nil {
			return nil, errors.Errorf("missing tiflash plan for %s", name)
		}
		return &TiFlashInstance{ShOpt: shOpt, Plan: *plan.TiFlash, ProcessInfo: info}, nil
	case ServiceTiProxy:
		if plan.TiProxy == nil {
			return nil, errors.Errorf("missing tiproxy plan for %s", name)
		}
		return &TiProxyInstance{Plan: *plan.TiProxy, ProcessInfo: info}, nil
	case ServicePrometheus:
		return &PrometheusInstance{ProcessInfo: info}, nil
	case ServiceGrafana:
		promURL := ""
		if plan.Grafana != nil {
			promURL = plan.Grafana.PrometheusURL
		}
		return &GrafanaInstance{PrometheusURL: promURL, ProcessInfo: info}, nil
	case ServiceNGMonitoring:
		if plan.NGMonitoring == nil {
			return nil, errors.Errorf("missing ng-monitoring plan for %s", name)
		}
		return &NGMonitoringInstance{Plan: *plan.NGMonitoring, ProcessInfo: info}, nil
	case ServiceTiCDC:
		if plan.TiCDC == nil {
			return nil, errors.Errorf("missing ticdc plan for %s", name)
		}
		return &TiCDC{Plan: *plan.TiCDC, ProcessInfo: info}, nil
	case ServiceTiKVCDC:
		if plan.TiKVCDC == nil {
			return nil, errors.Errorf("missing tikv-cdc plan for %s", name)
		}
		return &TiKVCDCInstance{Plan: *plan.TiKVCDC, ProcessInfo: info}, nil
	case ServiceDMMaster:
		if plan.DMMaster == nil {
			return nil, errors.Errorf("missing dm-master plan for %s", name)
		}
		return &DMMaster{Plan: *plan.DMMaster, ProcessInfo: info}, nil
	case ServiceDMWorker:
		if plan.DMWorker == nil {
			return nil, errors.Errorf("missing dm-worker plan for %s", name)
		}
		return &DMWorker{Plan: *plan.DMWorker, ProcessInfo: info}, nil
	case ServicePump:
		if plan.Pump == nil {
			return nil, errors.Errorf("missing pump plan for %s", name)
		}
		return &Pump{Plan: *plan.Pump, ProcessInfo: info}, nil
	case ServiceDrainer:
		if plan.Drainer == nil {
			return nil, errors.Errorf("missing drainer plan for %s", name)
		}
		return &Drainer{Plan: *plan.Drainer, ProcessInfo: info}, nil
	default:
		return nil, errors.Errorf("unsupported service %s", serviceID)
	}
}
