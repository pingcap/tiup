# TiUP

`tiup` is a tool to download and install TiDB components.

For detailed documentation, see the manual which starts with an [overview](overview.md).

To get started with TiDB using TiUP, see the [TiDB quick start guide](https://docs.pingcap.com/tidb/stable/quick-start-with-tidb). To deploy TiDB using TiUP in a production-like environment, see the [TiDB deploymeny guide](https://docs.pingcap.com/tidb/stable/production-deployment-using-tiup).

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
TiUP is a command-line component management tool that can help to download and install
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
  mirror      Manage a repository mirror for TiUP components
  telemetry   Controls things about telemetry
  completion  Output shell completion code for the specified shell (bash or zsh)
  env         Show the list of system environment variable that related to TiUP
  help        Help about any command or component

Components Manifest:
  use "tiup list" to fetch the latest components manifest

Flags:
  -B, --binary <component>[:version]   Print binary path of a specific version of a component <component>[:version]
                                       and the latest version installed will be selected if no version specified
      --binpath string                 Specify the binary path of component instance
      --help                           Help for this command
      --skip-version-check             Skip the strict version check, by default a version must be a valid SemVer string
  -T, --tag string                     Specify a tag for component instance
  -v, --version                        Print the version of tiup

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

The official components available are

```
Name               Owner      Description
----               -----      -----------
alertmanager       pingcap    Prometheus alertmanager
bench              pingcap    Benchmark database with different workloads
blackbox_exporter  pingcap    Blackbox prober exporter
br                 pingcap    TiDB/TiKV cluster backup restore tool
cdc                pingcap    CDC is a change data capture tool for TiDB
client             pingcap    Client to connect playground
cluster            pingcap    Deploy a TiDB cluster for production
ctl                pingcap    TiDB controller suite
diag               pingcap    Diagnostic Collector
dm                 pingcap    Data Migration Platform manager
dm-master          pingcap    dm-master component of Data Migration Platform
dm-worker          pingcap    dm-worker component of Data Migration Platform
dmctl              pingcap    dmctl component of Data Migration Platform
drainer            pingcap    The drainer componet of TiDB binlog service
dumpling           pingcap    Dumpling is a CLI tool that helps you dump MySQL/TiDB data
errdoc             pingcap    Document about TiDB errors
grafana            pingcap    Grafana is the open source analytics & monitoring solution for every database
insight            pingcap    TiDB-Insight collector
node_exporter      pingcap    Exporter for machine metrics
package            pingcap    A toolbox to package tiup component
pd                 pingcap    PD is the abbreviation for Placement Driver. It is used to manage and schedule the TiKV cluster
pd-recover         pingcap    PD Recover is a disaster recovery tool of PD, used to recover the PD cluster which cannot start or provide services normally
playground         pingcap    Bootstrap a local TiDB cluster for fun
prometheus         pingcap    The Prometheus monitoring system and time series database
pump               pingcap    The pump componet of TiDB binlog service
pushgateway        pingcap    Push acceptor for ephemeral and batch jobs
server             pingcap    TiUP publish/cache server
spark              pingcap    Spark is a fast and general cluster computing system for Big Data
tidb               pingcap    TiDB is an open source distributed HTAP database compatible with the MySQL protocol
tidb-lightning     pingcap    TiDB Lightning is a tool used for fast full import of large amounts of data into a TiDB cluster
tiflash            pingcap    The TiFlash Columnar Storage Engine
tikv               pingcap    Distributed transactional key-value database, originally created to complement TiDB
tikv-importer      pingcap
tispark            pingcap    tispark
tiup               pingcap    TiUP is a command-line component management tool that can help to download and install TiDB platform components to the local system
```
