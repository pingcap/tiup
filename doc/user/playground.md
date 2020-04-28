# Local Rapid Deployment TiDB Cluster

TiDB clusters are distributed systems consisting of multiple components, and a typical TiDB cluster consists of at least 3 PD nodes, 3 TiKV nodes, and 2 TiDB nodes. Deploying so many components by hand can be time consuming and a headache for users and even TiDB developers who want to experience TiDB. In this section, we will introduce the playground component in TiUP and use this component to build a native TiDB test environment.

## Playground component introduction

playground 的基本用法：

```bash
tiup playground [version] [flags]
```

The simplest startup command `tiup playground` will start a cluster of 1 KV, 1 DB, 1 PD using locally installed TiDB/TiKV/PD or their stable version. This order actually does the following.
- Since no version is specified, TiUP looks for the latest version of the installed playground first, assuming the latest version is v0.0.6, which is equivalent to tiup playground:v0.0.6.
- If the playground component has never had any version installed, TiUP will install the latest stable version before starting the runtime instance.
- Since playground does not specify the version of each TiDB/PD/TiKV component, by default it will use the latest release version of each component, and assuming the current version is v4.0.0-rc, this command is equivalent to tiup playground:v0.0.6 v4.0.0-rc
- Since playground also does not specify the number of components, by default it starts a minimized cluster of 1 TiDB, 1 TiKV and 1 PD
- After starting each component in turn, the playground will tell you that it started successfully and tell you some useful information, such as how to connect the cluster through the MySQL client, how to access the dashboard

The command line arguments for playground state:

```bash
Usage:
  tiup playground [version] [flags]

Flags:
      --db int                   TiDB instance number (default 1)
      --db.binpath string        TiDB instance binary path
      --db.config string         TiDB instance configuration file
  -h, --help                     help for tiup
      --host string              Playground cluster host (default "127.0.0.1")
      --kv int                   TiKV instance number (default 1)
      --kv.binpath string        TiKV instance binary path
      --kv.config string         TiKV instance configuration file
      --monitor                  Start prometheus component
      --pd int                   PD instance number (default 1)
      --pd.binpath string        PD instance binary path
      --pd.config string         PD instance configuration file
      --tiflash int              TiFlash instance number
      --tiflash.binpath string   TiFlash instance binary path
      --tiflash.config string    TiFlash instance configuration file
```

## Example

### Start a TiDB Cluster with Daily Build

```shell
tiup playground nightly
```

Nightly is the version number of this cluster, and similar ones can be `tiup playground v4.0.0-rc` etc.

### Start a cluster with monitoring.

```shell
tiup playground nightly --monitor
```

This command launches prometheus on port 9090 for displaying timing data within the cluster.

### Overrides the default configuration of the PD

Copy PD's [configuration template](https://github.com/pingcap/pd/blob/master/conf/config.toml), modify some content, and then execute:
```shell
tiup playground --pd.config ~/config/pd.toml
```

> Here the configuration is assumed to be placed at `~/config/pd.toml`

### Replace the default binary file
    
If a temporary binary is compiled locally and you want to put it into a cluster for testing, you can use the flag --{comp}.binpath to replace it, for example, the TiDB binary:
    
```shell
tiup playground --db.binpath /xx/tidb-server 
```

### Start multiple component instances
    
By default, TiDB, TiKV and PD each start one, and if you want to start more than one, you can do this:

```shell
tiup playground v3.0.10 --db 3 --pd 3 --kv 3
```
