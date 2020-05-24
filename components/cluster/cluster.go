package cluster

import (
	"io"
	"os"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
	"github.com/pingcap-incubator/tiup/pkg/utils"
)

// Repository exports interface to tiup-cluster
type Repository interface {
	DownloadComponent(comp, version, target string) error
	VerifyComponent(comp, version, target string) error
	ComponentBinEntry(comp, version string) (string, error)
}

type repositoryT struct {
	repo *repository.V1Repository
}

// NewRepository returns repository
func NewRepository(os, arch string) Repository {
	profile := localdata.InitProfile()
	mirror := repository.NewMirror(meta.Mirror(), repository.MirrorOptions{})
	repo := repository.NewV1Repo(mirror, repository.Options{
		GOOS:              os,
		GOARCH:            arch,
		DisableDecompress: true,
	}, v1manifest.NewManifests(profile))
	return &repositoryT{repo}
}

func (r *repositoryT) DownloadComponent(comp, version, target string) error {
	versionItem, err := r.repo.ComponentVersion(comp, version)
	if err != nil {
		return err
	}

	reader, err := r.repo.DownloadComponent(versionItem)
	if err != nil {
		return err
	}

	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

func (r *repositoryT) VerifyComponent(comp, version, target string) error {
	versionItem, err := r.repo.ComponentVersion(comp, version)
	if err != nil {
		return err
	}

	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()

	return utils.CheckSHA256(file, versionItem.Hashes["sha256"])
}

func (r *repositoryT) ComponentBinEntry(comp, version string) (string, error) {
	versionItem, err := r.repo.ComponentVersion(comp, version)
	if err != nil {
		return "", err
	}

	return versionItem.Entry, nil
}
