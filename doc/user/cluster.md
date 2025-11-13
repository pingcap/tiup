# Online cluster deployment and maintenance

The cluster component deploys production clusters as quickly as playground deploys local clusters, and it provides more powerful cluster management capabilities than playground, including upgrades to the cluster, downsizing, scaling and even operational auditing. It supports a very large number of commands:

```bash
$ tiup cluster
The component `cluster` is not installed; downloading from repository.
download https://tiup-mirrors.pingcap.com/cluster-v0.4.9-darwin-amd64.tar.gz 15.32 MiB / 15.34 MiB 99.90% 10.04 MiB p/s
Starting component `cluster`: /Users/joshua/.tiup/components/cluster/v0.4.9/cluster
Deploy a TiDB cluster for production

Usage:
  tiup cluster [flags]
  tiup [command]

Available Commands:
  deploy        Deployment Cluster
  start         Start deployed cluster
  stop          Stop Cluster
  restart       restart cluster
  scale-in      cluster shrinkage
  Scale-out     Cluster Scaling
  destroy       Destroy cluster
  upgrade       Upgrade Cluster
  exec          executes commands on one or more machines in the cluster
  display       Get cluster information
  list          Get cluster list
  audit         View cluster operation log
  edit-config   Editing the configuration of TiDB clusters
  reload        for overriding cluster configurations when necessary
  patch         replaces deployed components on its cluster with temporary component packages
  help          Print Help Information

Flags:
  -h, -help                 Help Information
      --ssh-timeout int     SSH connection timeout
  -y, --yes                 Skip all confirmation steps.
```

## Deployment cluster

The command used for deploying clusters is tiup cluster deploy, and its general usage is.

```bash
tiup cluster deploy <cluster-name> <version> <topology.yaml> [flags]
```

This command requires us to provide the name of the cluster, the version of TiDB used by the cluster, and a topology file for the cluster, which can be written with reference to [example](/examples/topology.example.yaml). Take a simplest topology as an example:

```yaml
---

pd_servers:
  - host: 172.16.5.134
    name: pd-134
  - host: 172.16.5.139
    name: pd-139
  - host: 172.16.5.140
    name: pd-140

tidb_servers:
  - host: 172.16.5.134
  - host: 172.16.5.139
  - host: 172.16.5.140

tikv_servers:
  - host: 172.16.5.134
  - host: 172.16.5.139
  - host: 172.16.5.140

grafana_servers:
  - host: 172.16.5.134

monitoring_servers:
  - host: 172.16.5.134
```

Save the file as `/tmp/topology.yaml`. If we want to use TiDB's v4.0.0-rc version with the cluster name prod-cluster, run:

```shell
tiup cluster deploy prod-cluster v3.0.12 /tmp/topology.yaml
```

During execution, the topology is reconfirmed and prompted for the root password on the target machine.

```bash
Please confirm your topology:
TiDB Cluster: prod-cluster
TiDB Version: v3.0.12
Type        Host          Ports        Directories
----        ----          -----        -----------
pd          172.16.5.134  2379/2380    deploy/pd-2379,data/pd-2379
pd          172.16.5.139  2379/2380    deploy/pd-2379,data/pd-2379
pd          172.16.5.140  2379/2380    deploy/pd-2379,data/pd-2379
tikv        172.16.5.134  20160/20180  deploy/tikv-20160,data/tikv-20160
tikv        172.16.5.139  20160/20180  deploy/tikv-20160,data/tikv-20160
tikv        172.16.5.140  20160/20180  deploy/tikv-20160,data/tikv-20160
tidb        172.16.5.134  4000/10080   deploy/tidb-4000
tidb        172.16.5.139  4000/10080   deploy/tidb-4000
tidb        172.16.5.140  4000/10080   deploy/tidb-4000
prometheus  172.16.5.134  9090         deploy/prometheus-9090,data/prometheus-9090
grafana     172.16.5.134  3000         deploy/grafana-3000
Attention:
    1. If the topology is not what you expected, check your yaml file.
    1. Please confirm there is no port/directory conflicts in same host.
Do you want to continue? [y/N]:
```

After entering the password, the tiup-cluster will download the required components and deploy them to the corresponding machine, indicating a successful deployment when you see the following prompt:

```bash
Deployed cluster `prod-cluster` successfully
```

## View cluster list

Once the cluster is deployed we will be able to see it in the cluster list via the tiup cluster list:

```bash
[user@localhost ~]# tiup cluster list
Starting /root/.tiup/components/cluster/v0.4.5/cluster list
Name          User  Version    Path                                               PrivateKey
----          ----  -------    ----                                               ----------
prod-cluster  tidb  v3.0.12    /root/.tiup/storage/cluster/clusters/prod-cluster  /root/.tiup/storage/cluster/clusters/prod-cluster/ssh/id_rsa
```

## Start the cluster.

If you have forgotten the name of the cluster you have deployed, you can use the tiup cluster list to see the command to start the cluster:

```shell
tiup cluster start prod-cluster
```

## Checking cluster status

We often want to know the operating status of each component in a cluster, and it's obviously inefficient to look at it from machine to machine, so it's time for the tiup cluster display, which is used as follows:

```bash
[user@localhost ~]# tiup cluster display prod-cluster
Starting /root/.tiup/components/cluster/v0.4.5/cluster display prod-cluster
TiDB Cluster: prod-cluster
TiDB Version: v3.0.12
ID                  Role        Host          Ports        Status     Data Dir              Deploy Dir
--                  ----        ----          -----        ------     --------              ----------
172.16.5.134:3000   grafana     172.16.5.134  3000         Up         -                     deploy/grafana-3000
172.16.5.134:2379   pd          172.16.5.134  2379/2380    Healthy|L  data/pd-2379          deploy/pd-2379
172.16.5.139:2379   pd          172.16.5.139  2379/2380    Healthy    data/pd-2379          deploy/pd-2379
172.16.5.140:2379   pd          172.16.5.140  2379/2380    Healthy    data/pd-2379          deploy/pd-2379
172.16.5.134:9090   prometheus  172.16.5.134  9090         Up         data/prometheus-9090  deploy/prometheus-9090
172.16.5.134:4000   tidb        172.16.5.134  4000/10080   Up         -                     deploy/tidb-4000
172.16.5.139:4000   tidb        172.16.5.139  4000/10080   Up         -                     deploy/tidb-4000
172.16.5.140:4000   tidb        172.16.5.140  4000/10080   Up         -                     deploy/tidb-4000
172.16.5.134:20160  tikv        172.16.5.134  20160/20180  Up         data/tikv-20160       deploy/tikv-20160
172.16.5.139:20160  tikv        172.16.5.139  20160/20180  Up         data/tikv-20160       deploy/tikv-20160
172.16.5.140:20160  tikv        172.16.5.140  20160/20180  Up         data/tikv-20160       deploy/tikv-20160
```

For normal components, the Status column will show "Up" or "Down" to indicate whether the service is normal or not, and for PD, the Status column will show Healthy or Down, and may have a |L to indicate that the PD is Leader.

## Condensation

Sometimes the business volume decreases and the cluster takes up some of the original resources, so we want to safely release some nodes and reduce the cluster size, so we need to downsize. The reduction is offline service, which eventually removes the specified node from the cluster and deletes the associated data files left behind. Since the downlinking of TiKV and Binlog components is asynchronous (requires removal through the API) and the downlinking process is time-consuming (requires constant observation to see if the node has been downlinked successfully), special treatment has been given to TiKV and Binglog components:

- Operation of TiKV and Binlog components
  - TiUP cluster exits directly after it is offline via API without waiting for the offline to complete
  - When you wait until later, you will check for the presence of TiKV or Binlog nodes that have already been downlinked when you execute commands related to cluster operations. If it does not exist, the specified operation continues; if it does, the following operation is performed.
    - Stopping the service of nodes that have been downlinked
    - Clean up the data files associated with nodes that have been taken offline
    - Update the topology of the cluster and remove nodes that have been dropped
- Operation of other components
  - The downlink of the PD component removes the specified node from the cluster via the API (a quick process), then disables the service of the specified PD and clears the data file associated with that node
  - Directly stop and clear the data files associated with the node when other components are downlinked

Basic usage of the condensation command:

```bash
tiup cluster-scale-in <cluster-name> -N <node-id>
````

It needs to specify at least two parameters, one is the cluster name and the other is the node ID, which can be obtained using the tiup cluster display command with reference to the previous section. For example, I want to kill the TiKV on 172.16.5.140, so I can execute:

```bash
[user@localhost ~]# tiup cluster display prod-cluster
Starting /root/.tiup/components/cluster/v0.4.5/cluster display prod-cluster
TiDB Cluster: prod-cluster
TiDB Version: v3.0.12
ID                  Role        Host          Ports        Status     Data Dir              Deploy Dir
--                  ----        ----          -----        ------     --------              ----------
172.16.5.134:3000   grafana     172.16.5.134  3000         Up         -                     deploy/grafana-3000
172.16.5.134:2379   pd          172.16.5.134  2379/2380    Healthy|L  data/pd-2379          deploy/pd-2379
172.16.5.139:2379   pd          172.16.5.139  2379/2380    Healthy    data/pd-2379          deploy/pd-2379
172.16.5.140:2379   pd          172.16.5.140  2379/2380    Healthy    data/pd-2379          deploy/pd-2379
172.16.5.134:9090   prometheus  172.16.5.134  9090         Up         data/prometheus-9090  deploy/prometheus-9090
172.16.5.134:4000   tidb        172.16.5.134  4000/10080   Up         -                     deploy/tidb-4000
172.16.5.139:4000   tidb        172.16.5.139  4000/10080   Up         -                     deploy/tidb-4000
172.16.5.140:4000   tidb        172.16.5.140  4000/10080   Up         -                     deploy/tidb-4000
172.16.5.134:20160  tikv        172.16.5.134  20160/20180  Up         data/tikv-20160       deploy/tikv-20160
172.16.5.139:20160  tikv        172.16.5.139  20160/20180  Up         data/tikv-20160       deploy/tikv-20160
172.16.5.140:20160  tikv        172.16.5.140  20160/20180  Offline    data/tikv-20160       deploy/tikv-20160
```

The node is automatically deleted after the PD schedules its data to other TiKVs.

## Expansion.

The internal logic of scaling is similar to deployment in that the TiUP cluster first guarantees the SSH connection of the node, creates the necessary directory on the target node, then executes the deployment and starts the service. The PD node's expansion is added to the cluster by join, and the configuration of the services associated with the PD is updated; other services are added directly to the cluster. All services do correctness validation at the time of expansion and eventually return whether the expansion was successful.

For example, expanding a TiKV node and a PD node in a cluster tidb-test:

### 1. New scale.yaml file, add TiKV and PD node IP

> **Note**
>
> Note that a new topology file is created that writes only the description of the expanded node, not the existing node.

```yaml
---

pd_servers:
  - ip: 172.16.5.140

tikv_servers:
  - ip: 172.16.5.140
````

### 2. Perform capacity expansion operations

TiUP cluster add the corresponding node to the cluster according to the information such as port, directory, etc. declared in the scale.yaml file:

```shell
tiup cluster scale-out tidb-test scale.yaml
````

After execution, you can check the expanded cluster status with the `tiup cluster display tidb-test` command.

## Rolling upgrade

The rolling upgrade feature leverages TiDB's distributed capabilities to keep the upgrade process as transparent and non-aware of the front-end business as possible. If there is a problem with the configuration, the tool will be upgraded node by node. Which has different operations for different nodes.

### The operation of different nodes

- Upgrade PD
  - Prioritize upgrading non-Leader nodes
  - Upgrade all non-Leader nodes after the upgrade is complete.
    - The tool sends a command to the PD to migrate the Leader to the node where the upgrade is complete
    - When Leader has been switched to another node, upgrade the old Leader node.
  - At the same time, if there is an unhealthy node in the upgrade process, the tool will suspend the upgrade and exit, at this time, the manual judgment, repair and then perform the upgrade.
- Upgrade TiKV
  - First add a migration to the PD that corresponds to the scheduling of the region leader on TiKV, and ensure that the upgrade process does not affect the front-end business by migrating the leader
  - Wait for the migration leader to complete before updating the TiKV node
  - Wait for the updated TiKV to start normally before removing the migration leader's scheduling.
- Upgrade other services
  - Normal out-of-service updates

### Upgrade operation

The upgrade command parameters are as follows:

```bash''
Usage:
  tiup cluster upgrade <cluster-name> <version> [flags]

Flags:
      --force                   forces escalation without transfer leader (dangerous operation)
  -h, --help                    help manual
      --transfer-timeout int    transfer leader's timeout

Global Flags:
      --ssh-timeout int     SSH connection timeout
  -y, --yes                 Skip all confirmation steps.
````

For example, to upgrade a cluster to v4.0.0-rc, you need only one command:

```bash
$ tiup cluster upgrade tidb-test v4.0.0-rc
````

## Update configuration

Sometimes we want to dynamically update the configuration of a component, tiup-cluster saves a copy of the current configuration for each cluster, and if we want to edit this configuration, we execute `tiup cluster edit-config <cluster-name>`, for example:

```bash
tiup cluster edit-config prod-cluster
````

The tiup-cluster then uses vi to open the configuration file for editing and save it after editing. The configuration is not applied to the cluster at this point, and if you want it to take effect, you need to execute:

```bash
tiup cluster reload prod-cluster
````

This action sends the configuration to the target machine, restarts the cluster, and makes the configuration effective.

## Update components

Regular upgrade clusters can use the upgrade command, but in some scenarios (e.g. Debug) it may be necessary to replace a running component with a temporary package, in which case you can use the patch command

```bash
[user@localhost ~]# tiup cluster patch --help
Replace the remote package with a specified package and restart the service

Usage:
  tiup cluster patch <cluster-name> <package-path> [flags]

Flags:
  -h, --help                    Help Information
  -N, --node strings            specify the node to be replaced
      --overwrite               uses the currently specified temporary package in future scale-out operations
  -R, -role strings             Specify the type of service to be replaced
      --transfer-timeout int    transfer leader's timeout

Global Flags:
      --ssh-timeout int   SSH connection timeout
  -y, --yes               Skip all confirmation steps
```

For example, if there is a TiDB hotfix package in /tmp/tidb-hotfix.tar.gz, and we want to replace all TiDBs on the cluster, we can:

```bash
tiup cluster patch test-cluster /tmp/tidb-hotfix.tar.gz -R tidb
```

Or just replace one of the TiDBs:

```
tiup cluster patch test-cluster /tmp/tidb-hotfix.tar.gz -N 172.16.4.5:4000
```
