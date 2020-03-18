[![LICENSE](https://img.shields.io/github/license/pingcap/tidb.svg)](https://github.com/pingcap-incubator/tiup/blob/master/LICENSE)
[![Language](https://img.shields.io/badge/Language-Go-blue.svg)](https://golang.org/)
[![Go Report Card](https://goreportcard.com/badge/github.com/pingcap-incubator/tiup)](https://goreportcard.com/badge/github.com/pingcap-incubator/tiup)
[![Coverage Status](https://codecov.io/gh/pingcap-incubator/tiup/branch/master/graph/badge.svg)](https://codecov.io/gh/pingcap-incubator/tiup/)

# What is TiUP

`tiup` is a tool to download and install TiDB components.

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
After installing `tiup`, you can use it to install binaries of TiDB components.

```
The tiup is a component management CLI utility tool that can help to download and install
the TiDB components to the local system. You can run a specific version of a component via
"tiup <component>[:version]". If no version number is specified, the latest version installed
locally will be run. If the specified component does not have any version installed locally,
the latest stable version will be downloaded from the repository. You can run the following
commands if you want to have a try.
  
  $ tiup playground            # Quick start
  $ tiup playground --tag p1   # Start a playground with a specified tag

Usage:
  tiup [component]|[command] [flags]

Available Commands:
  install     Install a specific version of a component
  list        List the available TiDB components or versions
  uninstall   Uninstall components or versions of a component
  update      Update tiup components to the latest version
  status      List the status of running components
  clean       Clean the data of the instantiated component
  help        Help about any command or component

Available Components:
  tidb         TiDB is an open source distributed HTAP database compatible with the MySQL protocol
  tikv         Distributed transactional key-value database, originally created to complement TiDB
  pd           PD is the abbreviation for Placement Driver. It is used to manage and schedule the TiKV cluster
  playground   Bootstrap a local TiDB cluster
  client       A simple mysql client to connect TiDB
  prometheus   An open-source systems monitoring and alerting toolkit
  tpc          A toolbox to benchmark workloads in TPC
  package      A toolbox to package tiup component

Flags:
  -B, --binary <component>[:version]   Print binary path of a specific version of a component <component>[:version]
                                       and the latest version installed will be selected if no version specified
  -h, --help                           help for tiup
      --rm                             Remove the data directory when the component instance finishes its run
      --skip-version-check             Skip the strict version check, by default a version must be a valid SemVer string
  -T, --tag string                     Specify a tag for component instance
      --version                        version for tiup

Use "tiup [command] --help" for more information about a command.
```
