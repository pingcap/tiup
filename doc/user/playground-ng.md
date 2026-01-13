# Playground-NG

Playground-NG (`tiup playground-ng`) is the new generation local cluster bootstrap tool in TiUP. It starts a TiDB cluster (TiDB/TiKV/PD/optional monitor) on your machine and provides a local HTTP control plane for runtime operations such as display/scale/stop.

Basic usage:

```bash
tiup playground-ng [version] [flags]
```

## Start a playground

Start in foreground (the process stays in the current terminal until stopped):

```bash
tiup playground-ng
tiup playground-ng nightly
```

Start in background (daemon mode):

```bash
tiup playground-ng --background
tiup playground-ng --background --tag my-cluster
```

In daemon mode, TiUP starts a short-lived starter process which spawns a daemon process. After the cluster is ready, the starter exits and the playground keeps running in background.

If you do not specify `--tag`, a random tag will be generated and printed when the starter reports success. Use that tag for subsequent `display/stop/scale-*` commands.

## Display and stop

Display running instances:

```bash
tiup playground-ng display --tag my-cluster
```

Stop a running playground:

```bash
tiup playground-ng stop --tag my-cluster
```

`stop` waits until the playground exits. Use `--timeout <seconds>` to change the max wait time.

## Data directory and logs

The playground data directory is `$TIUP_HOME/data/<tag>` (default: `~/.tiup/data/<tag>`).

When started in daemon mode, the daemon writes its stdout/stderr to:

```bash
$TIUP_HOME/data/<tag>/daemon.log
```
