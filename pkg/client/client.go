package client

import (
	"fmt"
	"strings"

	"github.com/pingcap/tiup/pkg/localdata"
)

type client struct {
	config *localdata.TiUPConfig
}

func NewTiUPClient(tiupHome string) (client, error) {
	c, err := localdata.InitConfig(tiupHome)
	return client{config: c}, err
}

// List components from all mirror, duplicate will be hide
func (c client) ListComponents() error {
	return nil
}

// list component info from first available mirror
func (c client) ListComponentDetail(component string) error {
	return nil
}

func (c client) Download(name, version string) error {
	return nil
}

func (c client) Remove(name, version string) error {
	return nil
}

func (c client) Install(name, version string) error {
	return nil
}

func (c client) Uninstall(name, version string) error {
	return nil
}

func (c client) SaveConfig(name, version string) error {
	return c.config.Flush()
}

func (c client) addAlias(k, v string) error {
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
