# Separate Component Version in Cluster

## Summary

Add a version field to each component on topology file. Allow user to upgrade single component to given version.

## Motivation

- New component TiDB Dashboard need to deployed with TiDB but not release with TiDB. Need a way to specify Dashboard version when deployment and upgrade.
- User want to upgrade node_exporter without upgrade TiDB cluster
- Maybe it could replace tiup cluster patch function to provide patch version of single component

## Detailed design

1. ~~add "latest" alias to tiup download function.It cloud be used to download latest release package of component.~~ just use "" as latest alias

2. add ComponentVersions struct to topology

```
// ComponentVersions represents the versions of components
ComponentVersions struct {
	TiDB         string `yaml:"tidb,omitempty"`
	TiKV         string `yaml:"tikv,omitempty"`
	TiFlash      string `yaml:"tiflash,omitempty"`
	PD           string `yaml:"pd,omitempty"`
	Dashboard    string `yaml:"tidb_dashboard,omitempty"`
	Pump         string `yaml:"pump,omitempty"`
	Drainer      string `yaml:"drainer,omitempty"`
	CDC          string `yaml:"cdc,omitempty"`
	TiKVCDC      string `yaml:"kvcdc,omitempty"`
	TiProxy      string `yaml:"tiproxy,omitempty"`
	Prometheus   string `yaml:"prometheus,omitempty"`
	Grafana      string `yaml:"grafana,omitempty"`
	AlertManager string `yaml:"alertmanager,omitempty"`
}
```

3. add node_exporter and blackbox_exporter version to MonitoredOptions struct

```
MonitoredOptions struct {
    ...
	NodeExporterVersion     string    `yaml:"node_exporter_version,omitempty" default:"latest"`
	BlackboxExporterVersion string    `yaml:"blackbox_exporter_version,omitempty" default:"latest"`
    ...
	}
```

4. Add CalculateVersion for each component struct. It returns cluster version if component version is not set for components like pd, tikv. It returns "" by default for components like alertmanager

5. Add flags to specify component versions

6. Merge ComponentVersion struct when scale-out

7. apply those version to deploy,scale-out and upgrade command
