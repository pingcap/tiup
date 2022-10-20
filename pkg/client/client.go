package client

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"go.uber.org/zap"
)

type client struct {
	config       *localdata.TiUPConfig
	repositories map[string]*repository.V1Repository
}

func NewTiUPClient(tiupHome string) (*client, error) {
	if tiupHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		tiupHome = filepath.Join(homeDir, localdata.ProfileDirName)
	}

	config, err := localdata.InitConfig(tiupHome)
	if err != nil {
		return nil, err
	}
	c := &client{
		config:       config,
		repositories: make(map[string]*repository.V1Repository),
	}

	for _, mirror := range config.Mirrors {

		initRepo := time.Now()
		profile := localdata.NewProfile(tiupHome, mirror, config)

		// Initialize the repository
		// Replace the mirror if some sub-commands use different mirror address
		mirrorAddr := "https://" + mirror
		m := repository.NewMirror(mirrorAddr, repository.MirrorOptions{})
		if err := m.Open(); err != nil {
			return nil, err
		}

		var v1repo *repository.V1Repository
		var err error

		var local v1manifest.LocalManifests
		local, err = v1manifest.NewManifests(profile)
		if err != nil {
			return nil, errors.Annotatef(err, "initial repository from mirror(%s) failed", mirrorAddr)
		}
		v1repo = repository.NewV1Repo(m, repository.Options{}, local)

		zap.L().Debug("Initialize repository finished", zap.Duration("duration", time.Since(initRepo)))

		c.repositories[mirror] = v1repo
	}

	return c, err
}

// List components from all mirror, duplicate will be hide
func (c *client) ListComponents() error {
	return nil
}

// list component info from first available mirror
func (c *client) ListComponentDetail(component string) error {
	return nil
}

func (c *client) Download(name, version string) error {
	return nil
}

func (c *client) Remove(name, version string) error {
	return nil
}

func (c *client) Install(s string) error {
	mirror, component, version, err := ParseComponentVersion(s)
	if err != nil {
		return err
	}
	var v1specs []repository.ComponentSpec
	v1specs = append(v1specs, repository.ComponentSpec{ID: component, Version: version, Force: false})

	if mirror != "" {
		if v1repo, ok := c.repositories[mirror]; ok {
			return v1repo.UpdateComponents(v1specs)
		}
	}

	for _, v1repo := range c.repositories {
		err = v1repo.UpdateComponents(v1specs)
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("cannot found %s", s)
}

func (c *client) Uninstall(name, version string) error {
	return nil
}

func (c *client) SaveConfig(name, version string) error {
	return c.config.Flush()
}

func (c *client) addAlias(k, v string) error {
	return nil
}

func ParseComponentVersion(s string) (mirror, component, tag string, err error) {
	splited := strings.Split(s, ":")
	switch len(splited) {
	case 1:
		tag = ""
	case 2:
		tag = splited[1]
	default:
		return "", "", "", fmt.Errorf("fail to parse %s", s)
	}

	splited = strings.Split(splited[0], "/")
	switch len(splited) {
	case 1:
		// TBD: use default mirror
		component = splited[0]
	case 2:
		mirror = splited[0]
		component = splited[1]
	default:
		return "", "", "", fmt.Errorf("fail to parse %s", s)
	}

	// TBD: convert mirror from alias to url

	return mirror, component, tag, nil
}
