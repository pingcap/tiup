# TiUP

`tiup` is a tool to download and install TiDB components.

# Installation

```sh
curl --proto '=https' --tlsv1.2 -sSf https://raw.githubusercontent.com/pingcap-incubator/tiup/master/install.sh | sh
```

# Usage
After installing `tiup`, you can use it to install binaries of TiDB components.

```
tiup [FLAGS]
  -V, --version   Show tiup version and quit

tiup COMMAND [FLAGS]
  show        Show available TiDB components
        --all    Show all available components and versions (refresh online).
  install     Install component(s) of specific version
  uninstall   Remove installed component(s)
```
