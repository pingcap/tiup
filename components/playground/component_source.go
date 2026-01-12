package main

import (
	"github.com/pingcap/errors"
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

func (s *envComponentSource) PlanInstall(component, resolved string, forcePull bool) (*DownloadPlan, error) {
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
	if err == nil && !forcePull && binaryExists(binPath) {
		return nil, nil
	}

	reason := "not_installed"
	if err == nil && binPath != "" && !binaryExists(binPath) {
		reason = "missing_binary"
	}
	if forcePull {
		reason = "force_pull"
	}

	return &DownloadPlan{
		ComponentID:     component,
		ResolvedVersion: resolved,
		DebugReason:     reason,
		DebugBinPath:    binPath,
	}, nil
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
