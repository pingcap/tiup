# Cluster Web UI

This is the web ui for tiup-cluster command.

## How to Run

### Release Mode

```shell
$ cd tiup
$ make embed_cluster_ui
$ UI=1 make
$ bin/tiup-cluster --ui
```

Then access `http://127.0.0.1:8080` in the browser.

### Develop Mode

```shell
$ cd tiup
$ make
$ bin/tiup-cluster --ui
# a new tab
$ cd cluster-ui
$ yarn && yarn start
```

It will auto open `http://127.0.0.1:3000/tiup` in the browser.
