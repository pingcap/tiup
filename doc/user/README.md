# TiUp

`tiup` is a tool to download and install TiDB components.

For detailed documentation, see the manual which starts with an [overview](overview.md).

## Installation

```sh
curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh
```

## Quick start

### Run playground

```sh
tiup playground
```

### Install components

```sh
tiup install tidb tikv pd
```

### Uninstall components

```sh
tiup uninstall tidb tikv pd
```

### Update components

```sh
tiup update --all
```

# Usage
After installing `tiup`, you can use it to install binaries of TiDB components and create clusters.

```
Use "tiup [command] --help" for more information about a command.Use "tiup [command] --help" for more information about a command.
s a command-line component management tool that can help to download and install
TiDB platform components to the local system. You can run a specific version of a component via
"tiup <component>[:version]". If no version number is specified, the latest version installed
locally will be used. If the specified component does not have any version installed locally,
the latest stable version will be downloaded from the repository.

Usage:
  tiup [flags] <command> [args...]
  tiup [flags] <component> [args...]

Available Commands:
  install     Install a specific version of a component
  list        List the available TiDB components or versions
  uninstall   Uninstall components or versions of a component
  update      Update tiup components to the latest version
  status      List the status of instantiated components
  clean       Clean the data of instantiated components
  help        Help about any command or component

Available Components:
  tidb                TiDB is an open source distributed HTAP database compatible with the MySQL protocol
  tikv                Distributed transactional key-value database, originally created to complement TiDB
  pd                  PD is the abbreviation for Placement Driver. It is used to manage and schedule the TiKV cluster
  playground          Bootstrap a local TiDB cluster
  client              A simple mysql client to connect TiDB
  prometheus          The Prometheus monitoring system and time series database.
  tpc                 A toolbox to benchmark workloads in TPC
  package             A toolbox to package tiup component
  tiops               Bootstrap a remote TiDB cluster
  grafana             Grafana is the open source analytics & monitoring solution for every database
  alertmanager        Prometheus Alertmanager
  blackbox_exporter   Blackbox prober exporter
  node_exporter       Blackbox prober exporter
  pushgateway         Blackbox prober exporter
  tiflash             
  drainer             The drainer componet of TiDB binlog service
  pump                The pump componet of TiDB binlog service
  cluster             Deploy a TiDB cluster for production

Flags:
  -B, --binary <component>[:version]   Print binary path of a specific version of a component <component>[:version]
                                       and the latest version installed will be selected if no version specified
      --binpath string                 Specify the binary path of component instance
  -h, --help                           help for tiup
      --skip-version-check             Skip the strict version check, by default a version must be a valid SemVer string
  -T, --tag string                     Specify a tag for component instance
      --version                        version for tiup

Component instances with the same "tag" will share a data directory ($TIUP_HOME/data/$tag):
  $ tiup --tag mycluster playground

Examples:
  $ tiup playground                    # Quick start
  $ tiup playground nightly            # Start a playground with the latest nightly version
  $ tiup install <component>[:version] # Install a component of specific version
  $ tiup update --all                  # Update all installed components to the latest version
  $ tiup update --nightly              # Update all installed components to the nightly version
  $ tiup update --self                 # Update the "tiup" to the latest version
  $ tiup list                          # Fetch the latest supported components list
  $ tiup status                        # Display all running/terminated instances
  $ tiup clean <name>                  # Clean the data of running/terminated instance (Kill process if it's running)
  $ tiup clean --all                   # Clean the data of all running/terminated instances

Use "tiup [command] --help" for more information about a command.
```
