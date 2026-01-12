package main

import (
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/utils"
)

type envComponentSource struct {
	env *environment.Environment
}

func newEnvComponentSource(env *environment.Environment) *envComponentSource {
	return &envComponentSource{env: env}
}

func (s *envComponentSource) ResolveVersion(component, constraint string) (string, error) {
	if s == nil || s.env == nil {
		return "", errors.New("environment not initialized")
	}
	v, err := s.env.V1Repository().ResolveComponentVersion(component, constraint)
	if err != nil {
		return "", err
	}
	return v.String(), nil
}

func requiredBinaryPathForService(serviceID proc.ServiceID, baseBinPath string) string {
	baseBinPath = strings.TrimSpace(baseBinPath)
	if baseBinPath == "" {
		return ""
	}

	switch serviceID {
	case proc.ServiceTiKVWorker:
		return proc.ResolveTiKVWorkerBinPath(baseBinPath)
	case proc.ServiceNGMonitoring:
		if filepath.Base(baseBinPath) == "ng-monitoring-server" {
			return baseBinPath
		}
		path, _ := proc.ResolveSiblingBinary(baseBinPath, "ng-monitoring-server")
		return path
	default:
		return baseBinPath
	}
}

func planInstallByResolvedBinaryPath(serviceID proc.ServiceID, component, resolved, baseBinPath string, binPathErr error, forcePull bool) *DownloadPlan {
	// PlanInstall treats any binary path error as "not installed": it should
	// produce a download plan rather than failing planning.
	if binPathErr == nil && !forcePull {
		checkPath := requiredBinaryPathForService(serviceID, baseBinPath)
		if binaryExists(checkPath) {
			return nil
		}
	}

	reason := "not_installed"
	if forcePull {
		reason = "force_pull"
	} else if binPathErr == nil {
		reason = "missing_binary"
	}

	debugBinPath := strings.TrimSpace(baseBinPath)
	if binPathErr == nil {
		debugBinPath = strings.TrimSpace(requiredBinaryPathForService(serviceID, baseBinPath))
	}

	return &DownloadPlan{
		ComponentID:     component,
		ResolvedVersion: resolved,
		DebugReason:     reason,
		DebugBinPath:    debugBinPath,
	}
}

func (s *envComponentSource) PlanInstall(serviceID proc.ServiceID, component, resolved string, forcePull bool) (*DownloadPlan, error) {
	if s == nil || s.env == nil {
		return nil, errors.New("environment not initialized")
	}
	if component == "" {
		return nil, errors.New("component is empty")
	}
	if resolved == "" {
		return nil, errors.Errorf("component %s resolved version is empty", component)
	}

	v := utils.Version(resolved)
	binPath, err := s.env.BinaryPath(component, v)
	return planInstallByResolvedBinaryPath(serviceID, component, resolved, binPath, err, forcePull), nil
}

func (s *envComponentSource) EnsureInstalled(component, resolved string) error {
	if s == nil || s.env == nil {
		return errors.New("environment not initialized")
	}
	if component == "" {
		return errors.New("component is empty")
	}
	if resolved == "" {
		return errors.Errorf("component %s resolved version is empty", component)
	}
	spec := repository.ComponentSpec{ID: component, Version: resolved, Force: true}
	return s.env.V1Repository().UpdateComponents([]repository.ComponentSpec{spec})
}

func (s *envComponentSource) BinaryPath(component, resolved string) (string, error) {
	if s == nil || s.env == nil {
		return "", errors.New("environment not initialized")
	}
	if component == "" {
		return "", errors.New("component is empty")
	}
	if resolved == "" {
		return "", errors.Errorf("component %s resolved version is empty", component)
	}
	return s.env.BinaryPath(component, utils.Version(resolved))
}
