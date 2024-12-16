# Local Rapid Deployment TiDB Cluster

TiDB clusters are distributed systems consisting of multiple components, and a typical TiDB cluster consists of at least 3 PD nodes, 3 TiKV nodes, and 2 TiDB nodes. Deploying so many components by hand can be time consuming and a headache for users and even TiDB developers who want to experience TiDB. In this section, we will introduce the playground component in TiUP and use this component to build a native TiDB test environment.

## Playground component introduction

playground's basic usage:

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
  tiup [command]

Available Commands:
  completion  generate the autocompletion script for the specified shell
  display
  help        Help about any command
  scale-in
  scale-out

Flags:
      --db int                   TiDB instance number
      --db.host host             Playground TiDB host. If not provided, TiDB will still use host flag as its host
      --db.port int              Playground TiDB port. If not provided, TiDB will use 4000 as its port
      --db.binpath string        TiDB instance binary path
      --db.config string         TiDB instance configuration file
      --db.timeout int           TiDB max wait time in seconds for starting, 0 means no limit
      --drainer int              Drainer instance number
      --drainer.binpath string   Drainer instance binary path
      --drainer.config string    Drainer instance configuration file
      --grafana.port int         grafana port. If not provided, grafana will use 3000 as its port. (default 3000)
  -h, --help                     help for tiup
      --host string              Playground cluster host
      --kv int                   TiKV instance number
      --kv.binpath string        TiKV instance binary path
      --kv.config string         TiKV instance configuration file
      --mode string              TiUP playground mode: 'tidb', 'tikv-slim' (default "tidb")
      --pd int                   PD instance number
      --pd.Host host             Playground PD host. If not provided, PD will still use host flag as its host
      --pd.binpath string        PD instance binary path
      --pd.config string         PD instance configuration file
      --pump int                 Pump instance number
      --pump.binpath string      Pump instance binary path
      --pump.config string       Pump instance configuration file
  -T, --tag string               Specify a tag for playground, data dir of this tag will not be removed after exit
      --ticdc int                TiCDC instance number
      --ticdc.binpath string     TiCDC instance binary path
      --ticdc.config string      TiCDC instance configuration file
      --tiflash int              TiFlash instance number
      --tiflash.binpath string   TiFlash instance binary path
      --tiflash.config string    TiFlash instance configuration file
      --tiflash.timeout int      TiFlash max wait time in seconds for starting, 0 means no limit
  -v, --version                  version for tiup
      --without-monitor          Don't start prometheus and grafana component
```

## Example

### Start a TiDB Cluster with Daily Build

```shell
tiup playground nightly
```

Nightly is the version number of this cluster, and similar ones can be `tiup playground v4.0.0-rc` etc.

### Start a cluster with or without monitoring.

```shell
tiup playground nightly
```

This command launches Prometheus on port 9090 and Grafana on port 3000 for displaying timing data within the cluster.

```shell
tiup playground nightly --without-monitor
```

This won't launch Prometheus or Grafana. This can be used to save resources.

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
