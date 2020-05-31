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
After installing `tiup`, you can use it to install binaries of TiDB components and create clusters.

See our [doc](doc/user/README.md) for more information on how to use TiUp.

# Contributing to TiUp

Contributions of code, tests, docs, and bug reports are welcome! To get started take a look at our [open issues](https://github.com/pingcap-incubator/tiup/issues).

For docs on how to build, test, and run TiUp, see our [dev docs](docs/dev/README.md).

See also the [Contribution Guide](https://github.com/pingcap/community/blob/master/CONTRIBUTING.md) in PingCAP's
[community](https://github.com/pingcap/community) repo.
