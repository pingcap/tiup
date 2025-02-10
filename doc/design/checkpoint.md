# The checkpoint implementation for tiup-cluster and tiup-dm

When there is an occasional error on `tiup cluster` or `tiup dm` command, some users may want to retry previews action from the fail point instead of from scratch.

For example, the following tasks:

```
1. download package
          |
2. scp package to remote
          |
3. unzip remote package     <- failed here
          |
4. start service
```

If something wrong with the third task, retry from the first task is OK because TiUP provides a guarantee that all commands are idempotent. However, for some large clusters, it may waste a lot of time on successful tasks (task 1 and stpe 2) and the user may want to restart the process from task 3.

## The audit log

tiup-cluster and tiup-dm will generate an audit log file in `${TIUP_HOME}/storage/{cluster,dm}/audit/`, you can view the audit list with the command `tiup cluster audit` or `tiup dm audit`. The list looks like this:

```
ID           Time                       Command
--           ----                       -------
fxgcScKJ2Kd  2021-01-21T18:36:10+08:00  /home/tidb/.tiup/components/cluster/v1.3.1/tiup-cluster display test
fxgcRrMnBz8  2021-01-21T18:35:56+08:00  /home/tidb/.tiup/components/cluster/v1.3.1/tiup-cluster start test
```

The first column is the id of the audit, to view a specified audit log, use the command `tiup cluster audit <id>` or `tiup dm audit <id>`, the content of the audit log is something like this:

```
/home/tidb/.tiup/components/cluster/v1.3.1/tiup-cluster display test
2021-01-21T18:36:08.380+0800    INFO    Execute command {"command": "tiup cluster display test"}
2021-01-21T18:36:09.805+0800    INFO    SSHCommand      {"host": "172.16.5.140", "port": "22", "cmd": "xxx command", "stdout": "xxxx", "stderr": ""}
```

The first line of the file is the command the user executed, the following lines are structure logs.

## The checkpoint

In the implementation of the checkpoint, we mix checkpoint in the audit log, like this:

```
/home/tidb/.tiup/components/cluster/v1.3.1/tiup-cluster display test
2021-01-21T18:36:08.380+0800    INFO    Execute command {"command": "tiup cluster display test"}
2021-01-21T18:36:09.805+0800    INFO    SSHCommand      {"host": "172.16.5.140", "port": "22", "cmd": "echo task 1", "stdout": "task 1", "stderr": ""}
2021-01-21T18:36:09.806+0800    INFO    CheckPoint      {"host": "172.16.5.140", "n": 1, "result": true}
2021-01-21T18:36:09.805+0800    INFO    SSHCommand      {"host": "172.16.5.140", "port": "22", "cmd": "echo task 2", "stdout": "task 2", "stderr": ""}
2021-01-21T18:36:09.806+0800    INFO    CheckPoint      {"host": "172.16.5.140", "n": 2, "result": true}
2021-01-21T18:36:09.805+0800    INFO    SSHCommand      {"host": "172.16.5.140", "port": "22", "cmd": "echo task 3", "stdout": "task 2", "stderr": ""}
2021-01-21T18:36:09.806+0800    INFO    CheckPoint      {"host": "172.16.5.140", "n": 3, "result": true}
```

If the user runs tiup-cluster or tiup-dm in replay mode by giving an audit id, we will parse that audit log file and pick up all `CheckPoint` by order into a queue, then in corresponding functions, we check if the checkpoint is in the queue, if hit, we dequeue the checkpoint and return the result directly instead of doing the real work. Example:

```golang
func init() {
    // Register checkpoint fields so that we know how to compare checkpoints
    countHost := checkpoint.Register(
        checkpoint.Field("host", reflect.DeepEqual),
        checkpoint.Field("n", func(a, b interface{}) bool {
            // the n is a int, however, it will be float after it write to json because json only has float number.
            // so we just compare the string format.
            return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
        }),
    )
}

func processCommand() {
    // we will explain the context in the next section
    ctx := checkpoint.NewContext(context.Background())
    r, err := task(ctx, host1, 1)
    handleResult(r, err)
    r, err = task(ctx, host2, 2)
    handleResult(r, err)
    r, err = task(ctx, host3, 3)
    handleResult(r, err)
}

func task(ctx context.Context, host string, n int) (result bool, err error) {
    // first, we get the checkpoint from audit log
    point := checkpoint.Acquire(ctx, countHost, map[string]interface{}{
        "host": host,
        "n":  n,
    })
    defer func() {
        // we must call point.Release, otherwise there will be a resource leak.
        // the release function will write the checkpoint into current audit log (not the one user specified)
        // for latter replay.
        point.Release(err,
            zap.String("host", host),
            zap.Int("n", n),
            zap.Bool("result", result),
        )
    }()
    // Then, if the checkpoint exists in the specified audit file, point.Hit() will return map[string]interface{}
    if point.Hit() != nil {
        return point.Hit()["result"].(bool), nil
    }

    // Last, if the checkpoint does not exist in the specified audit file, we should do real work and return the result
    return do_real_work(host, n)
}
```

## The context

If the function with checkpoint calls another function with checkpoint, we will get into trouble:

```golang
func processCommand() {
    ctx := checkpoint.NewContext(context.Background())
    r, err := task(ctx, host1, 1)
    handleResult(r, err)
    r, err := task(ctx, host1, 0)
    handleResult(r, err)
}

// we use a flag mock that the task only return true once
var flag = true
func task(ctx context.Context, host string, n int) (result bool, err error) {
    point := checkpoint.Acquire(ctx, countHost, map[string]interface{}{ ... }
    defer func() { point.Release( ... ) }
    if point.Hit() != nil { ... }

    if n > 1 {
        return task(ctx, host, n - 1)
    }

    defer func() { flag = false }()
    return flag, nil
}
```

The execution flow and return value will be:

```
task(1)[called by processCommand]:  return true
|
task(0)[called by task(1)]:         return true
|
task(0)[called by processCommand]:  return false
```

the checkpoint in audit log will be:

```
... {"host": "...", "n": 1, "result": true}
... {"host": "...", "n": 0, "result": true}
... {"host": "...", "n": 0, "result": false}
```

There are three checkpoints, but when we try to replay the process, the `task(0)[called by task(1)]` will not be called at all since `task(1)` will return early with the cached result, so the execution flow will be:

```
task(1)[called by processCommand]:  return true (cached by {"host": "...", "n": 1, "result": true})
|
task(0)[called by processCommand]:  return true (cached by {"host": "...", "n": 0, "result": true})
```

The trouble is coming: in the real case the `task(0)[called by processCommand]` returns false but in replay case it return true because it takes the result of `task(0)[called by task(1)]` by mistake. The problem is that the `CheckPoint` of `task(0)[called by task(1)]` should not be record because it's parent, `task(0)[called by processCommand]`, has record a `CheckPoint`.

So we implement a semaphore and insert it into the context passed to `checkpoint.Acquire`, the context or it's ancestor must be generated by `checkpoint.NewContext` where the semaphore is generated. When `checkpoint.Acquire` called, it will try to acquire the semaphore and record if it success in the returned value, when we call `Release` on the returned value, it will check if previews semaphore acquire success, if not, the `Release` will not writing checkpoint.

## Parallel task

Because we use a semaphore in the context to trace if it's the first stack layer that wants to write checkpoint, the context can't be shared between goroutines:

```golang
func processCommand() {
    ctx := checkpoint.NewContext(context.Background())

    for _, n := range []int{1, 2, 3} {
        go func() {
            r, err := task(ctx, host, n)
            handleResult(r, err)
        }()
    }
}
```

There are three tasks, `task(1)`, `task(2)` and `task(3)`, they run parallelly. What if the `task(0)` run first but return last?

```
task(1): start -------------------------------------------> return
task(2):         start ------------------------> return
task(3):         start ------------------------> return
```

The checkpoint of `task(2)` and `task(3)` will not be recorded because they think they are called by `task(1)`. The solution is to add a semaphore for every goroutine:

```golang
func processCommand() {
    ctx := checkpoint.NewContext(context.Background())

    for _, n := range []int{1, 2, 3} {
        go func(ctx context.Context) {
            r, err := task(ctx, host, n)
            handleResult(r, err)
        }(checkpoint.NewContext(ctx))
    }
}
```

What if the `processCommand` or its' ancestor has its' own checkpoint?

```golang
func processTask(ctx context.Context) {
    p := checkpoint.Acquire(...)
    defer func() { p.Release(...) }()
    if p.Hit() != nil { ... }

    return processCommand(ctx)
}

func processCommand(ctx context.Context) {
    for _, n := range []int{1, 2, 3} {
        go func(ctx context.Context) {
            r, err := task(ctx, host, n)
            handleResult(r, err)
        }(checkpoint.NewContext(ctx))
    }
}
```

If `checkpoint.NewContext` just append a unacquired semaphore, the checkpoint of `processTask` and it's children(`task(1..3)`) will be all recorded, that's not correct (we have talked this before).

So the `checkpoint.NewContext` should check if there is already a semaphore in current context, if there is, just copy it's value. By this way, if `processTask` has acquired the semaphore, the `task(1..3)` will get their own acquired semaphore, otherwise, they will get their own unacquired semaphore.
