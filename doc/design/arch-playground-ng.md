# TiUP Playground-NG Overall Architecture

> Scope: `components/playground-ng/**` (and a small set of dependent modules in `pkg/**`).
>
> Goal: Help readers quickly understand the module boundaries of playground-ng, and the startup / runtime / shutdown flows.

## 1. Code Layout (package boundaries)

The core playground-ng code is split into only 3 packages (excluding tests):

- `components/playground-ng` (`package main`):
  - The CLI for the component binary `tiup-playground-ng`, boot/scale/display, the controller actor loop, HTTP control plane, progress UI, and persisting state to disk.
- `components/playground-ng/service` (`package service`):
  - A registry and declarative metadata (`Spec`/`Catalog`) at the granularity of “services”.
  - Describes: whether a service is enabled, default instance count, whether it is critical, boot ordering dependencies (`StartAfter`), whether scale-out is allowed, scale-in hooks, etc.
- `components/playground-ng/proc` (`package proc`):
  - The implementation layer at the granularity of “process instances”.
  - Defines: `Process`/`ProcessInfo`/`OSProcess`/`ReadyWaiter`, and the `Prepare()` / `WaitReady()` logic for instances like PD/TiDB/TiKV/TiFlash/Prometheus/Grafana/…

Dependency direction among these three packages (top-down):

`main` → depends on `service` (get Spec/Catalog) → depends on `proc` (create concrete instances and run them)

## 2. Key Abstractions (who is responsible for what)

### 2.1 `service.Spec` / `service.Catalog`: declarative “service definitions”

`components/playground-ng/service/service.go` defines the “service metadata layer” of playground-ng.

- `Spec`:

  - `ServiceID`: logical service name (e.g. `tidb`, `pd`, `tiflash.compute`, etc.).
  - `NewProc(rt, params)`: constructs a `proc.Process` instance (but does not start an OS process).
  - `StartAfter []ServiceID`: startup order dependencies.
  - `ScaleInHook(...)`: pre scale-in hook (can optionally stop asynchronously).
  - `PostScaleOut(w, inst)`: prints hints after a successful scale-out.

- `Catalog`:
  - Used to generate CLI flags (`FlagPrefix` + `AllowModify*`), and decides:
    - Whether enabled: `IsEnabled(ctx)`
    - Config snapshot at planning time: `PlanConfig(ctx)`
    - Whether critical: `IsCritical(ctx)`
    - Whether scale-out is allowed: `AllowScaleOut`
    - Version constraint binding: `VersionBind`

Key point: orchestration in `main` tries not to hard-code service-specific behaviors; instead it iterates `service.AllSpecs()` to do planning / defaults / start ordering.

### 2.2 `proc.Process` / `proc.OSProcess`: separating an instance from the “real OS process”

`components/playground-ng/proc/proc.go` defines two interface layers:

- `proc.Process`: the “instance” as perceived by playground-ng (includes config, directories, ports, component ID, etc.)
  - `Prepare(ctx)`: only responsible for “constructing the command” (write config / assemble argv / set `info.Proc`), not for starting.
  - `LogFile()`: used by playground-ng to redirect stdout/stderr.
- `proc.OSProcess`: a lightweight wrapper around `*exec.Cmd` (`cmdProcess`)
  - `Start()/Wait()`: the actual OS process lifecycle.
  - `SetOutputFile(path)`: directs `Cmd.Stdout/Stderr` to the log file.

This layering means:

- Services/instances only need to translate “how to start” into `exec.Cmd` (and config files).
- Playground-ng centrally controls “when to start, how to monitor, how to shut down”.

### 2.3 `Playground` + controller actor loop: single-writer rule for runtime state

`components/playground-ng/playground.go` / `components/playground-ng/controller.go`:

- `Playground`: holds `dataDir`, `bootOptions`, `processGroup`, and the controller channels (`evtCh`/`cmdReqCh`).
- `controllerLoop`: an actor-style event loop that exclusively owns the runtime state (`controllerState`):
  - `procs map[ServiceID][]Process`
  - `requiredServices`/`criticalRunning` (minimum count for critical services + running count, used for auto shutdown)
  - `expectedExit` (exits caused by user stop/scale-in, to avoid false alarms)
  - `procByPID`/`procByName` (PID/Name indexes for display/scale-in)

Cross-goroutine interactions must go through:

- event: `EmitEvent` → `evtCh`
- command: HTTP handler → `doCommand` → `cmdReqCh`

The UI mutex (`progressMu`) is an exception, used only for shared state in progress rendering.

### 2.4 `ProcessGroup`: dynamic goroutine management

`Playground.processGroup` is a group manager that supports “dynamic Add, early Wait, and no more Add after Close”.

Use cases:

- Register each process waiter (`osProc.Wait()`)
- Register the HTTP server lifecycle
- Runtime scale-out may dynamically add watchers/waiters as well

On shutdown, call `processGroup.Close()` first, then wait on `processGroup.Wait()`, to avoid Wait/Add concurrency bugs.

## 3. Startup Flow (`bootCluster`)

Core entry: root command `RunE` in `components/playground-ng/main.go` → `p.bootCluster(ctx, &options)`.

Key steps in `components/playground-ng/boot.go:bootCluster` (in order):

1. Normalize paths: `normalizeBootOptionPaths` (convert `*.ConfigPath` to absolute paths).
2. Start controller: `p.startController()`.
3. Set booting state: `setControllerBooting(true)`.
4. Validate (pure): `ValidateBootOptionsPure` (e.g. PD count; mode/version gates; CSE endpoint parsing; etc.).
5. Plan: `planProcs(options)` + `buildBootPlanWithProcs(...)` to produce a `BootPlan`.
   - Port allocation happens in planning (policy: `alloc_free` for real runs; `none` for tests/dry-run determinism).
   - Version resolution and “needs download?” decisions are done via `ComponentSource` and saved into `plan.Downloads`.
6. Save `bootBaseConfigs` (default config snapshots for runtime scale-out).
7. Execute plan (no more flag/env reads in executor):
   - `bootExecutor.Download(plan)`: install missing components from `plan.Downloads` (can be canceled via boot ctx).
   - `bootExecutor.PreRun(plan)`: execution-time preflight (e.g. S3 bucket check/create in CSE/Disagg/NextGen)
     and per-service pre-run hooks (e.g. TiProxy session cert generation).
   - `bootExecutor.AddProcs(plan)`: create `proc.Process` instances from `plan.Services` and add them into controller state.
8. Start instances: `bootStarter.startPlanned` (honor `Spec.StartAfter`, send `startProcRequest` via controller).
9. Wait for critical ready: `bootStarter.waitRequiredReady()`.
10. Close the “Start instances” progress group and print Cluster info.
11. Write `dsn` file: `dumpDSN(dataDir/dsn, ...)`.
12. Generate Prometheus targets: `renderSDFile()` (write `prometheus-*/targets.json`).
13. Write monitor topology into PD etcd: `updateMonitorTopology`.
14. Mark booted: `setControllerBooted(true)` (after this, scale-out uses join logic).
15. Start local HTTP server: `listenAndServeHTTP()`.

Dry-run entry: `components/playground-ng/main.go` uses the same planner to produce a `BootPlan` and renders it
(`--dry-run-output=text|json`), without entering the execute stages above.

## 4. Runtime Control Plane (HTTP + Command)

- server: `components/playground-ng/command.go` (`(*Playground).listenAndServeHTTP` + `commandHandler`)

  - Listens on `127.0.0.1:<port>`, exposes `POST /command`
  - Strict JSON validation: `DisallowUnknownFields`, with a body size limit.

- client (subcommands): `components/playground-ng/command.go`
  - `display/scale-in/scale-out/stop` first locate the target via `resolvePlaygroundTarget`, then request `/command`.

Target selection rules (for multiple co-existing playground-ngs): `components/playground-ng/command.go` (`resolvePlaygroundTarget`)

- If `--tag` or `TIUP_INSTANCE_DATA_DIR` is explicitly specified: read `dataDir/port` and probe the HTTP server, no guessing.
- Otherwise scan `<tiupHome>/data/*/port` and probe each candidate:
  - 0 reachable: report “no playground running”
  - 1 reachable: use it directly
  - multiple reachable: prompt that `--tag` must be specified

### 4.1 Daemon Mode (background start)

- CLI: root command supports `--background/-d`, which runs a short-lived starter that spawns a daemon process of the same binary (`--run-as-daemon`).
- Data dir selection: in daemon mode, always use `<tiupHome>/data/<tag>` (never `TIUP_INSTANCE_DATA_DIR`) so the TiUP runner won't clean it up when the starter exits.
- Runtime markers:
  - `dataDir/pid`: exclusive claim file to prevent concurrent startups and to detect stale instances.
  - `dataDir/port`: created after the command server successfully listens; removed on server exit.
  - `dataDir/daemon.log`: daemon stdout/stderr for debugging / operations.
  - `dataDir/tuiv2.events.jsonl`: tuiv2 progress event log; starter tails + replays it to render boot progress in a real TTY.

### 4.2 Progress UI (`pkg/tuiv2/progress`)

Playground-ng uses a unified progress system (`tuiv2`) for both:

- interactive TTY output (spinners, live-updated Active area + immutable History area)
- non-interactive logs (CI / redirects) via a stable, append-only plain renderer

Key points:

- `Group` / `Task` are **emit-only handles** that can be updated from any goroutine.
- Rendering state is exclusively owned by the UI engine loop (Bubble Tea for TTY; plain renderer otherwise), which avoids cross-goroutine state mutations.
- `UI.Writer()` converts arbitrary `io.Writer` usage (callouts, fmt.Fprintf, log printers) into `PrintLines` events, so output never corrupts TTY rendering.
- In daemon mode, the daemon process writes the event stream to `dataDir/tuiv2.events.jsonl`; the starter tails it and calls `UI.ReplayEvent` to reproduce the exact same output in the user’s terminal.

## 5. Scaling (scale-out / scale-in)

- `scale-out`: `components/playground-ng/scale.go:handleScaleOut`

  - First use `bootBaseConfigs` (`BootConfig`) to fill defaults (binpath/config/host), and convert config paths to absolute paths.
  - For each new instance:
    - `addProcInController`: create `dataDir/<service>-<id>` + `spec.NewProc`
    - `startProc`: Resolve/Prepare/SetOutputFile/Start + waiter + optional WaitReady
  - After success: `spec.PostScaleOut` + `OnProcsChanged()` (refresh prom targets)

- `scale-in`: `components/playground-ng/scale.go:handleScaleIn`
  - Supports locating instances by `--pid` or `--name` (via controller indexes `procByPID/procByName`).
  - Run `spec.ScaleInHook` first:
    - async mode: the hook triggers stop itself; the instance is eventually removed via events
    - sync mode: controller marks expected exit, sends `SIGQUIT`, removes it from `procs`
  - Directories are not deleted automatically (it only stops processes and updates status/targets).

## 6. Exit and Cleanup

- Shutdown triggers:

  - User Ctrl+C (signal handler sends stop event; double Ctrl+C triggers force-kill).
  - Boot failure (`requestStopInternal`).
  - Critical service count drops below required (auto shutdown triggered by `handleProcExited`).

- Shutdown execution: `components/playground-ng/shutdown.go`

  - Snapshot current proc records (so it can continue to kill/wait even after controller cancel).
  - Stop accepting new events/commands: `controllerCancel()` + `processGroup.Close()`.
  - Send `SIGTERM` to PIDs in sequence, then `SIGKILL` after timeout.

- dataDir deletion policy:
  - playground-ng standalone: when no `--tag` is provided, a random tag is generated and `os.RemoveAll(dataDir)` is performed on exit.
  - tiup runner: `pkg/exec/run.go` removes `InstanceDir` for “temporary runs” (no tag); keeps it if a tag is provided.

## 7. Persisted State and Directory Structure

**Root directory (`dataDir`)**

- `dataDir/pid`: exclusive occupancy marker for a running playground.
- `dataDir/port`: port of the HTTP control server (`dumpPort/loadPort`).
- `dataDir/daemon.log`: daemon mode stdout/stderr log file.
- `dataDir/tuiv2.events.jsonl`: daemon mode tuiv2 progress event log file.
- `dataDir/dsn`: connection info written after boot completes (`dumpDSN`).

**Instance directories (one per service instance)**

- `dataDir/<serviceID>-<id>/`: created by `addProcInController`.
- config/log/data files are determined by each `proc/*` instance’s `Prepare()`:
  - TOML merge: `proc.prepareConfig` merges default config + user config and writes into the instance directory.
  - stdout/stderr: playground-ng redirects to `inst.LogFile()` uniformly (with truncation).
  - Prometheus: writes `prometheus.yml` + `targets.json`.
  - Grafana: writes `conf/custom.ini`, `conf/provisioning/**`, `dashboards/**`.
