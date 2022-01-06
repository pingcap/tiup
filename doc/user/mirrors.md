# Build a private mirror

When building a private cloud, it is common to use an isolated network environment where the official mirror of TiUP is not accessible, so we provide a solution for building a private mirror, which is mainly implemented by the mirror component, which can also be used for offline deployment.

## Mirrors component introduction

First, let's look at the `mirror' help file.

```bash
$ tiup mirror --help
The 'mirror' command is used to manage a component repository for TiUP, you can use
it to create a private repository, or to add new component to an existing repository.
The repository can be used either online or offline.
It also provides some useful utilities to help managing keys, users and versions
of components or the repository itself.

Usage:
  tiup mirror <command> [flags]

Available Commands:
  init        Initialize an empty repository
  sign        Add signatures to a manifest file
  genkey      Generate a new key pair
  clone       Clone a local mirror from remote mirror and download all selected components
  merge       Merge two or more offline mirror
  publish     Publish a component
  show        Show mirror address
  set         Set mirror address
  modify      Modify published component
  renew       Renew the manifest of a published component.
  grant       grant a new owner
  rotate      Rotate root.json

Global Flags:
      --help                 Help for this command
      --skip-version-check   Skip the strict version check, by default a version must be a valid SemVer string

Use "tiup mirror [command] --help" for more information about a command.
```

Its basic use is `tiup mirror clone <target-dir> [global-version] [flags]`, the target-dir is the directory in which the cloned data needs to be placed. global-version is used to quickly set a common version for all components.

Then this order has very scary dozens of flags and even more later. But there is no need to be intimidated by the number of these flags, which are in fact of four types.

### 1. specify whether to override local packages

The `--overwrite` parameter means that if the specified <target-dir> already has a package that you want to download, you should overwrite it with the official image of the package, if this flag is set it will overwrite it.

### 2. Whether to clone in full quantity

If `--full` is specified, the official image will be cloned intact.

> **Note**
>
> If `--full` is not specified and no other flag is specified, then only some meta information will be cloned.

### 3. Platform limitation

If you only want to clone packages for a particular platform, you can use `-os` and `-arch` to qualify:
- `tiup mirror clone <target-dir> ---os=linux`
- Just want to clone amd64 architecture: `tiup mirror clone <target-dir> --arch=amd64`
- Just want to clone linux/amd64: `tiup mirror clone <target-dir> --os=linux --arch=amd64`

### 4. Component version limited

If you want to clone only one version of a component and not all versions, use `--<component>=<version>` to qualify, for example "
- Just want to clone the v4 version of tidb: `tiup mirror clone <target-dir> --tidb v4`
- Just want to clone the v4 version of tidb, and all versions of tikv: `tiup mirror clone <target-dir> --tidb v4 --tikv all` 
- Clone specific versions of all components that start a cluster: `tiup mirror clone <target-dir> v4.0.0-rc`

## The real thing

### Offline installation

For example, if we want to install a v4.0.0-rc TiDB cluster in an isolated environment, we can execute the following command on a machine connected to the extranet to pull the required components:

```bash
tiup mirror package --os=linux v4.0.0-rc
```

This command creates a directory called `package` in the current directory that contains the package of components necessary to start a cluster, which is then packaged by the tar command and sent to a central control unit in an isolated environment:

```bash
tar czvf package.tar.gz package
```

package.tar.gz is a standalone offline environment. After sending it to the target cluster's central controller, install TiUP with the following command:

```bash
tar xzvf package.tar.gz
cd package
sh local_install.sh
```

After installing TiUP as prompted, deploy the TiDB cluster (assuming the working directory is still in the package):

```bash
export TIUP_MIRRORS=/path/to/mirror
tiup cluster xxx
```

`/path/to/mirror` is the location of <target-dir> in `tiup mirror clone <target-dir>`, or if in /tmp/package:
```bash
export TIUP_MIRRORS=/tmp/package
```

For cluster operations, refer to the [cluster command](. /cluster.md).

### Private Mirror

The way to build a private image is the same as for an offline installation package, just upload the contents of the package directory to a CDN or file server:

```bash
cd package
python -m SimpleHTTPServer 8000
```

This creates a private image at the address http://127.0.0.1:8000. Installation of TiUP:

```bash
export TIUP_MIRRORS=http://127.0.0.1:8000
curl $TIUP_MIRRORS/install.sh | sh
```

After importing the PATH variable, you can use TiUP normally (you need to keep the TIUP_MIRRORS variable pointing to a private image).
