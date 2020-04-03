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
      --skip-version-check             Skip the strict version check, by default a version must be a valid SemVer string
  -T, --tag string                     Specify a tag for component instance
      --version                        version for tiup

Use "tiup [command] --help" for more information about a command.
```

# Contributing to TiUp

Contributions of code, tests, docs, and bug reports are welcome! To get started take a look at our [open issues](https://github.com/pingcap-incubator/tiup/issues).

## Building

You can build TiUp on any platform that supports Go.

Prerequisites:

* Go (minimum version: 1.13; [installation instructions](https://golang.org/doc/install))
* golint (`go get -u golang.org/x/lint/golint`)
* make

To build TiUp, run `make`.

## Running locally for development

For development, you don't want to use any global directories. You may also want to supply your own metadata. TiUp can be modified using the following environment variables:

* `TIUP_HOME` the profile directory, where TiUp stores its metadata.
* `TIUP_MIRRORS` set the location of TiUp's registry, can be a directory or URL

## Testing

TiUp has unit and integration tests; you can run both by running `make test`.

Unit tests are alongside the code they test, following the Go convention of using a `_test` suffix for test files. Integration tests are in the [tests](tests) directory.

## Architecture overview

Each TiUp command has its own executable, their source is in the [cmd](cmd) directory. The main TiUp executable is [root.go](cmd/root.go).

The core of TiUp is defined in the [pkg](pkg) directory.

[localdata](pkg/localdata) manages TiUp's metadata held on the user's computer.

[meta](pkg/meta) contains high-level functions for managing components.

[repository](pkg/repository) handles remote repositories.

The [set](pkg/set), [tui](pkg/tui), and [utils](pkg/utils) packages contain utility types and functions. The [version](pkg/version) package contains version data for TiUp and utility functions for handling that data.

The [mock](pkg/mock) package is a utility for testing.

Some key concepts:

* *Repository* a source of components and metadata concerning those components and TiUp in general.
* *Profile* the state of an installation of TiUp and the components it manages.
* *Component* a piece of software that can be managed by TiUp, e.g., TiDB or the playground.
* *Command* a top-level command run by TiUp, e.g., `update`, `list`.

### TiUp registry structure

* tiup-manifest.index: the manifest file in json format.
* Manifests for each component named tiup-component-$name.index, where %name is the name of the component.
* Component tarballs, one per component, per version; named $name-$version-$os-$arch.tar.gz, where $name and $version identify the component, and $os and $arch are a supported platform. Each tarball has a sha1 hash with the same name, but extension .sha1, instead of .tar.gz.

### Manifest formats

See `ComponentManifest` and `VersionManifest` data structures in [component.go](pkg/repository/component.go) and [version.go](pkg/repository/version.go).
