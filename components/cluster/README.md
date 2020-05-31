# Usage

The `tiup-cluster` is distributed via [tiup](https://tiup.io) in a production environment. After installing the `tiup`,
the user can use the `tiup-cluster` by typing command `tiup cluster`.

1. Deploy a cluster for production `tiup-cluster deploy <cluster-name> <version> <topology.yaml> [flags]`
2. Start a TiDB cluster `tiup cluster start <cluster-name>`
3. Stop a TiDB cluster `tiup cluster stop <cluster-name>`
4. Restart a TiDB cluster `tiup cluster restart <cluster-name>`
5. Scale in a TiDB cluster `tiup cluster scale-in <cluster-name> --node <node-id-1> [node-id-2..N]`
6. Scale out a TiDB cluster `tiup cluster scale-out <cluster-name> <incr-topology.yaml>`
7. Destroy a specified cluster `tiup cluster destroy <cluster-name>`
8. Upgrade a specified TiDB cluster `tiup cluster upgrade <cluster-name> <version>`
9. Run shell command on host in the tidb cluster `tiup cluster exec <cluster-name> --command "ls"`
10. Display information of a TiDB cluster `tiup cluster display <cluster-name>`
11. List all clusters `tiup cluster list`
12. Show audit log of cluster operation `tiup cluster audit`
13. Import an exist TiDB cluster from TiDB-Ansible `tiup cluster import`
14. Edit TiDB cluster config `tiup cluster edit-config`
15. Reload a TiDB cluster's config and restart if needed `tiup cluster reload <cluster-name>`

# Contributing to TiUp

Contributions of code, tests, docs, and bug reports are welcome! To get started take a look at our [open issues](https://github.com/pingcap-incubator/tiup-cluster/issues).

## Environment

If you haven't cluster deploy environment, you can use the docker environment provided by `tiup-cluster` project.

1. Before initialize the environment, you should install [Docker](https://docs.docker.com/install/) first.
2. After installing docker, you can bootstrap the development environment by following instructions

    ```shell script
    cd docker
    sh up.sh --dev
    docker exec -it tiup-cluster-control bash
    ```

## Quick start

1. Prepare a topology.

    ```yaml
    global:
      user: tidb
    
    tidb_servers:
      - host: 172.19.0.101
    
    pd_servers:
      - host: 172.19.0.102
      - host: 172.19.0.104
      - host: 172.19.0.105
    
    tikv_servers:
      - host: 172.19.0.103
    
    grafana_servers:
      - host: 172.19.0.101
    
    monitoring_servers:
      - host: 172.19.0.102
    
    alertmanager_servers:
      - host: 172.19.0.103
    ```
2. Deploy a cluster

    ```yaml
    tiup-cluster deploy test1 v3.0.10 topology.yaml -i ~/.ssh/id_rsa
    ```

The development doesn't depend on `tiup`, you can use `tiup-cluster` directly, which equal `tiup cluster` in the `tiup` mode.

## Building

You can build `tiup-cluster` on any platform that supports Go.

Prerequisites:

* Go (minimum version: 1.13; [installation instructions](https://golang.org/doc/install))
* make

To build `tiup-cluster`, run `make build`.

## Local mirrors

The components depended by `tiup-cluster` will be downloaded from the tiup components repository, you can build a local
mirror repository via the script https://github.com/pingcap-incubator/tiup/blob/master/localmirrors.sh.
And set the environment variable via `export TIUP_MIRRORS=/path/to/local/mirrors` before `sh up.sh --dev`.

## Architecture overview

TBD
