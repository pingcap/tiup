package client

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils"
	"go.uber.org/zap"
)

type Client struct {
	tiupHome string
	config   *localdata.TiUPConfig
	// repo represents the components repository of TiUP, it can be a
	// local file system or a HTTP URL
	repositories map[string]*repository.V1Repository
}

func NewTiUPClient(tiupHome string) (*Client, error) {
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

	if config.Aliases == nil {
		config.Aliases = make(map[string]string)
	}

	c := &Client{
		tiupHome:     tiupHome,
		config:       config,
		repositories: make(map[string]*repository.V1Repository),
	}

	for _, mirror := range config.Mirrors {
		v1repo, err := c.initRepository(mirror.Name, mirror.URL)
		if err != nil {
			return nil, err
		}
		c.repositories[mirror.Name] = v1repo
	}

	return c, err
}

func (c *Client) TiUPHomePath() string {
	return c.tiupHome
}

// ListMirrors show all Mirrors
func (c *Client) ListMirrors() []localdata.SingleMirror {
	return c.config.Mirrors
}

// AddMirror add a new tiup morror
func (c *Client) AddMirror(mirror localdata.SingleMirror, rootJSON io.Reader) error {
	if _, ok := c.repositories[mirror.Name]; ok {
		return errors.Errorf("mirror %s already exists", mirror.Name)
	}

	// todo: add check
	c.config.Mirrors = append(c.config.Mirrors, mirror)

	os.MkdirAll(filepath.Join(c.tiupHome, localdata.TrustedDir, mirror.Name), 0755)
	f, err := os.OpenFile(filepath.Join(c.tiupHome, localdata.TrustedDir, mirror.Name, "root.json"), os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	io.Copy(f, rootJSON)

	v1repo, err := c.initRepository(mirror.Name, mirror.URL)
	if err != nil {
		return err
	}
	c.repositories[mirror.Name] = v1repo
	return nil
}

func (c *Client) DownloadComponents(specs []string, nightly, force bool) error {
	mirrorSpecs := map[string][]repository.ComponentSpec{}
	for _, spec := range specs {
		mirror, component, version, err := c.ParseComponentVersion(spec)
		if err != nil {
			return err
		}
		if version == "" && nightly {
			version = utils.NightlyVersionAlias
		}
		mirrorSpecs[mirror] = append(mirrorSpecs[mirror], repository.ComponentSpec{ID: component, Version: version, Force: force})
	}

	// download
	var errs []string
	for mirror, specs := range mirrorSpecs {
		repo, err := c.GetRepository(mirror)
		if err != nil {
			return err
		}
		if err := repo.UpdateComponents(specs); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(append([]string{"update falied"}, errs...), "\n"))
	}

	return nil
}

func (c *Client) Remove(name, version string) error {
	return nil
}

func (c *Client) Install(s string) error {
	mirror, component, version, err := c.ParseComponentVersion(s)
	if err != nil {
		return err
	}
	var v1specs []repository.ComponentSpec
	v1specs = append(v1specs, repository.ComponentSpec{ID: component, Version: version, Force: false})

	if mirror != "" {
		if v1repo, ok := c.repositories[mirror]; ok {
			c.tryAddAlias(component, fmt.Sprintf("%s/%s", v1repo.Local().Name(), component))
			return v1repo.UpdateComponents(v1specs)
		}
	}

	for _, v1repo := range c.repositories {
		err = v1repo.UpdateComponents(v1specs)
		if err == nil {
			c.tryAddAlias(component, fmt.Sprintf("%s/%s", v1repo.Local().Name(), component))
			return nil
		}
	}
	return fmt.Errorf("Component %s not found in all mirrors", s)
}

func (c *Client) Uninstall(s string) error {
	mirror, component, version, err := c.ParseComponentVersion(s)
	if err != nil {
		return err
	}

	paths := []string{}

	repo, err := c.GetRepository(mirror)
	if err != nil {
		return err
	}

	dir, err := os.ReadDir(repo.Local().ProfilePath(localdata.ComponentParentDir, mirror, component))
	if err != nil {
		return errors.Trace(err)
	}
	if version == utils.NightlyVersionAlias {
		for _, fi := range dir {
			if utils.Version(fi.Name()).IsNightly() {
				paths = append(paths, repo.Local().ProfilePath(localdata.ComponentParentDir, mirror, component, fi.Name()))
			}
		}
	} else {
		paths = append(paths, repo.Local().ProfilePath(localdata.ComponentParentDir, mirror, component, version))
	}
	if len(dir)-len(paths) < 1 {
		paths = append(paths, repo.Local().ProfilePath(localdata.ComponentParentDir, mirror, component))
	}

	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

func (c *Client) SaveConfig() error {
	return c.config.Flush()
}

func (c *Client) tryAddAlias(component, mirrorComp string) {
	_, exist := c.config.Aliases[component]
	if !exist {
		c.config.Aliases[component] = mirrorComp
	}
}

func (c *Client) initRepository(name, url string) (*repository.V1Repository, error) {
	initRepo := time.Now()
	profile := localdata.NewProfile(c.tiupHome, name, c.config)

	// Initialize the repository
	// Replace the mirror if some sub-commands use different mirror address
	mirrorAddr := url
	if mirrorAddr == "" {
		mirrorAddr = "https://" + name
	}
	m := repository.NewMirror(mirrorAddr, repository.MirrorOptions{})
	if err := m.Open(); err != nil {
		return nil, err
	}

	var v1repo *repository.V1Repository
	var local v1manifest.LocalManifests
	local, err := v1manifest.NewManifests(profile)
	if err != nil {
		return nil, errors.Annotatef(err, "initial repository from mirror(%s) failed", mirrorAddr)
	}
	v1repo = repository.NewV1Repo(m, repository.Options{}, local)
	zap.L().Debug("Initialize repository finished", zap.Duration("duration", time.Since(initRepo)))

	return v1repo, nil
}

func (c *Client) ParseComponentVersion(s string) (mirror, component, tag string, err error) {

	// get mrror/component from alias
	if _, ok := c.config.Aliases[s]; ok {
		s = c.config.Aliases[s]
	}

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

// Repositories return all repo
func (c *Client) Repositories() map[string]*repository.V1Repository {
	return c.repositories
}

// Repositories return all repo
func (c *Client) GetRepository(mirror string) (*repository.V1Repository, error) {
	repo, ok := c.repositories[mirror]
	if !ok {
		return nil, errors.Errorf("morror [%s] not found", mirror)
	}

	return repo, nil
}

// SelfUpdate updates TiUP.
func (c *Client) SelfUpdate() error {
	// get default mirror
	mirror, _, _, err := c.ParseComponentVersion(repository.TiUPBinaryName)
	if err != nil {
		return err
	}

	repo, err := c.GetRepository(mirror)
	if err != nil {
		return err
	}

	err = repo.DownloadTiUP(repo.Local().ProfilePath("bin"))
	if err != nil {
		return err
	}

	url, err := c.config.GetMirrorAddress(mirror)
	if err != nil {
		return err
	}

	return repo.Local().ResetMirror(url, "")
}
