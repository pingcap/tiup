# Dockerized tiup-cluster

This docker image attempts to simplify the setup required by tiup-cluster.
It is intended to be used by a CI tool or anyone with docker who wants to try tiup-cluster themselves.

It contains all the tiup-cluster dependencies and code. It uses [Docker Compose](https://github.com/docker/compose) to spin up the five
containers used by tiup-cluster.

To start run

```
    ./up.sh
    docker exec -it tiup-cluster-control bash
```

During development, it's convenient to run with `--dev` option, which mounts `$TIUP_CLUSTER_ROOT` dir as `/tiup-cluster` on tiup-cluster control container.

Run `./up.sh --help` for more info.
