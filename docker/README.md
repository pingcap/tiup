# Dockerized tiops

This docker image attempts to simplify the setup required by tiops.
It is intended to be used by a CI tool or anyone with docker who wants to try tiops themselves.

It contains all the tiops dependencies and code. It uses [Docker Compose](https://github.com/docker/compose) to spin up the five
containers used by tiops.

To start run

```
    ./up.sh
    docker exec -it tiup-cluster-control bash
```

During development, it's convenient to run with `--dev` option, which mounts `$TIUP_CLUSTER_ROOT` dir as `/tiops` on tiops control container.

Run `./up.sh --help` for more info.
