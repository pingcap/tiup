# Packet management

Component management is done primarily through TiUP's subcommands, which currently exist.
- list: look up the list of components to know which ones are available for installation and which versions are available
- install: Install a specific version of a component.
- update: Upgrade a component to the latest version
- uninstall: uninstall components
- status: View component run status
- clean: clean component example
- help: Print the help information, followed by the command to print how to use it

## Query component list: tipped list

When you want to install something with TiUP, you first need to know what components are available and what versions of those components are available, which is what the list command does. It supports several of these uses.
- tiup list: See what components are currently available for installation
- tiup list ${component}: See what versions of a component are available

For the above two methods of use, two flags can be used in combination.
--installed: which components are already installed locally, or which versions of a component are already installed

Example 1: View all components currently installed

```shell
tiup list --installed
```

Example 2: Get a list of all TiKV installable version components from the server

```shell
tiup list tikv
```

## Install component: tiup install

After viewing the list of components, installation is also very simple, using the tiup install command, which is used as follows.
- tiup install <component>: Install the latest stable version of the specified component
- tiup install <component>:[version]: Install the specified version of the specified component

Example 1: Installing the latest stable version of TiDB using TiUP

```shell
tiup install tidb
```

Example 2: Installing the nightly version of TiDB with TiUP

```shell
tiup install tidb:nightly
```

Example 3: Installing TiKV version v3.0.6 with TiUP

```shell
tiup install tikv:v3.0.6
```

## Upgrade components

After the new version of the official component is available, it is also possible to upgrade with TiUP, which is used in much the same way as install, except for a few additional flags:
---all: Upgrade all components
--nightly: Upgrade to nightly version
--self: Upgrade TiUP yourself to the latest version
--force: mandatory upgrade to the latest version

Example 1: Upgrade all components to the latest version

```shell
tiup update --all
```

Example 2: Upgrade all components to the nightly version

```shell
tiup update --all --nightly
```

Example 3: Upgrade TiUP to the latest version

```shell
tiup update --self
```

## Run component: tiup <component>

After the installation is complete, the appropriate components can be launched using TiUP.

```shell
tiup [flags] <component>[:version] [args...]
```

This command requires the name of a component and an optional version, or if no version is provided, the latest stable version of the component installed.

Before the component starts, TiUP creates a directory for it and then puts the component into that directory to run. The component generates all the data in that directory, and the name of the directory is the tag name specified by the component at runtime. If no tag is specified, a tag name is generated at random and the working directory is *automatically deleted* upon instance termination.

If we want to start the same component multiple times and reuse the previous working directory, we can specify the same name at startup with --tag. By specifying a tag, the working directory is *not automatically deleted* when the instance is terminated, making it easy to reuse at the next startup.

Example 1: Running TiDB version v3.0.8

```shell
tiup tidb:v3.0.8
```

Example 2: Specifying the tag to run TiKV

```shell
tiup--tag=experiment tikv
```

### Query component runtime status: tiup status

The tiup status allows you to view the component's operational status, which is very simple to use.

```shell
tiup status
```

Running this command will get a list of instances, one per line. The list contains these columns.
- Name: tag name of the instance
- Component: The component name of the instance.
- PID: Process ID of the instance running
- Status: Instance status, RUNNING means running, TERM means terminated
- Created Time: Start time of the instance
- Directory: the working directory of the instance, which can be specified by --tag
- Binary: The executable of the instance, which can be specified by --binpath
- Args: the running parameters of the instance

### Clean component example: tiup clean

The component instances can be cleaned with tiup clean and the working directory can be removed. If the instance is still running before the cleanup, the associated process is killed first. The corresponding commands and parameters are as follows.

```bash
tiup clean [tag] [tags] [flags]
```

The following flags are supported:
--all Clear all instance information.

where tag indicates the instance tag to be cleaned, and if `--all` is used, no tag is passed

Example 1: Example of a component whose tag name is experiment

```shell
tiup clean experiment
```

Example 2: Clean up all component examples

{{< copyable "shell-regular">}

```shell
tiup clean --all
```

### Uninstall component: tiup uninstall

TiUP supports uninstalling all or specific versions of a component, as well as uninstalling all components. The basic usage is as follows.

```bash
tiup uninstall [component][:version] [flags]
```

Supported flags:
--all uninstall all components or versions
--self Uninstall TiUP itself

component is the name of the component to be uninstalled, and version is the version to be uninstalled, both of which can be omitted, either of which needs to be added `--all` to use.
- If the version is omitted, add `--all` to uninstall all versions of the component
- If both versions and components are omitted, add `--all' to indicate that all components and all their versions are uninstalled

Example 1: Uninstalling TiDB in v3.0.8

```shell
tiup uninstall tidb:v3.0.8
```

Example 2: Uninstall all versions of TiKV

```shell
tiup uninstall tikv --all
```

Example 3: Uninstall all installed components

```shell
tiup uninstall --all
```
