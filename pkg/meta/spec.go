package meta

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/joomcode/errorx"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/file"
	"github.com/pingcap/tiup/pkg/utils"
	"gopkg.in/yaml.v2"
)

var (
	errNS        = errorx.NewNamespace("spec")
	errNSCluster = errNS.NewSubNamespace("cluster")
	// ErrClusterCreateDirFailed is ErrClusterCreateDirFailed
	ErrClusterCreateDirFailed = errNSCluster.NewType("create_dir_failed")
	// ErrClusterSaveMetaFailed is ErrClusterSaveMetaFailed
	ErrClusterSaveMetaFailed = errNSCluster.NewType("save_meta_failed")
)

const (
	// MetaFileName is the file name of the meta file.
	MetaFileName = "meta.yaml"
	// PatchDirName is the directory to store patch file eg. {PatchDirName}/tidb-hotfix.tar.gz
	PatchDirName = "patch"
	// BackupDirName is the directory to save backup files.
	BackupDirName = "backup"
)

// Spec control management of spec meta data.
type Spec struct {
	base string
}

// NewSpec create a spec instance.
func NewSpec(base string) *Spec {
	return &Spec{
		base: base,
	}
}

// ClusterPath returns the full path to a subpath (file or directory) of a
// cluster, it is a subdir in the profile dir of the user, with the cluster name
// as its name.
func (s *Spec) ClusterPath(cluster string, subpath ...string) string {
	if cluster == "" {
		// keep the same behavior with legacy version of TiOps, we could change
		// it in the future if needed.
		cluster = "default-cluster"
	}

	return filepath.Join(append([]string{
		s.base,
		cluster,
	}, subpath...)...)
}

// SaveClusterMeta save the meta with specified cluster name.
func (s *Spec) SaveClusterMeta(clusterName string, meta interface{}) error {
	wrapError := func(err error) *errorx.Error {
		return ErrClusterSaveMetaFailed.Wrap(err, "Failed to save cluster metadata")
	}

	metaFile := s.ClusterPath(clusterName, MetaFileName)
	backupDir := s.ClusterPath(clusterName, BackupDirName)

	if err := s.ensureClusterDir(clusterName); err != nil {
		return wrapError(err)
	}

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return wrapError(err)
	}

	data, err := yaml.Marshal(meta)
	if err != nil {
		return wrapError(err)
	}

	err = file.SaveFileWithBackup(metaFile, data, backupDir)
	if err != nil {
		return wrapError(err)
	}

	return nil
}

// ClusterMetadata tries to read the metadata of a cluster from file
func (s *Spec) ClusterMetadata(clusterName string, meta interface{}) error {
	fname := s.ClusterPath(clusterName, MetaFileName)

	yamlFile, err := ioutil.ReadFile(fname)
	if err != nil {
		return errors.AddStack(err)
	}

	err = yaml.Unmarshal(yamlFile, meta)
	if err != nil {
		return errors.AddStack(err)
	}

	return nil
}

// List return the cluster names.
func (s *Spec) List() (names []string, err error) {
	fileInfos, err := ioutil.ReadDir(s.base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errors.AddStack(err)
	}

	for _, info := range fileInfos {
		if utils.IsNotExist(s.ClusterPath(info.Name(), MetaFileName)) {
			continue
		}
		names = append(names, info.Name())
	}

	return
}

// ensureClusterDir ensures that the cluster directory exists.
func (s *Spec) ensureClusterDir(clusterName string) error {
	if err := utils.CreateDir(s.ClusterPath(clusterName)); err != nil {
		return ErrClusterCreateDirFailed.
			Wrap(err, "Failed to create cluster metadata directory '%s'", s.ClusterPath(clusterName)).
			WithProperty(cliutil.SuggestionFromString("Please check file system permissions and try again."))
	}
	return nil
}
