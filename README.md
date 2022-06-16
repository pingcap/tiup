[![LICENSE](https://img.shields.io/github/license/pingcap/tidb.svg)](https://github.com/pingcap/tiup/blob/master/LICENSE)
[![Language](https://img.shields.io/badge/Language-Go-blue.svg)](https://golang.org/)
[![Go Report Card](https://goreportcard.com/badge/github.com/pingcap/tiup)](https://goreportcard.com/badge/github.com/pingcap/tiup)
[![Coverage Status](https://codecov.io/gh/pingcap/tiup/branch/master/graph/badge.svg)](https://codecov.io/gh/pingcap/tiup/)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fpingcap%2Ftiup.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fpingcap%2Ftiup?ref=badge_shield)

# What is TiUP

`tiup` is a tool to download and install [TiDB](https://docs.pingcap.com/tidb/stable/overview) components.

## Documentation

- [English](https://docs.pingcap.com/tidb/stable/tiup-documentation-guide)
- [简体中文](https://docs.pingcap.com/zh/tidb/stable/tiup-documentation-guide)

## Blog

- [English](https://pingcap.com/blog/)
- [简体中文](https://pingcap.com/blog-cn/)

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

## Usage

After installing `tiup`, you can use it to install binaries of TiDB components and create clusters.

See our [doc](doc/user/README.md) for more information on how to use TiUP.

## Contributing to TiUP

Contributions of code, tests, docs, and bug reports are welcome! To get started, take a look at our [open issues](https://github.com/pingcap/tiup/issues).

For docs on how to build, test, and run TiUP, see our [dev docs](doc/dev/README.md).

See also the [Contribution Guide](https://github.com/pingcap/community/blob/master/contributors/README.md) in PingCAP's
[community](https://github.com/pingcap/community) repo.

## License

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fpingcap%2Ftiup.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fpingcap%2Ftiup?ref=badge_large)
