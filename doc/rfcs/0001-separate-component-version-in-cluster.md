# Separate Component Version in Cluster

## Summary

Add a version field to each component on topology file. Allow user to upgrade single component to given version.

## Motivation

- New component TiDB Dashboard need to deployed with TiDB but not release with TiDB. Need a way to specify Dashboard version when deployment and upgrade.
- User want to upgrade node_exporter without upgrade TiDB cluster
- Maybe it could replace tiup cluster patch function to provide patch version of single component

## Detailed design

1. add "latest" alias to tiup download function.It cloud be used to download latest release package of component.

2. add version field to DashboardSpec struct.

```
type DashboardSpec struct {
	...
	Version    string    `yaml:"version,omitempty" default:"latest"`
	...
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

4. if user not specify version, those version has default value latest which means use newest stable version in tiup repo.

5. maybe add version field to all other component, deafult value is empty which means use global cluster version

6. apply those version to deploy,scale-out and upgrade command

7. how user upgrade single exist component? add a new subcommand or just edit-config and reload? I don't know yet