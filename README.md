# TiUP Demo
This is a prototype demo of TiUP.

# Usage
After installing `tiup`, you can use it to install binaries of TiDB components.

```
tiup MODULE [OPTIONS] COMMAND <TARGET>
  component
    list                         List installed components and their versions.
      --all                      List full list of available components and versions (fetch from remote).
    install
    uninstall
      --version                  Specify version of components to (un)install, default is latest.
  service
    list                         List all available services and their statuses.
    create                       Create a new service from installed components.
    start                        Start a service.
    stop                         Stop a service.
    restart                      Restart a service.
    delete                       Delete a service.
      --cluster <cluster_name>   Specify which cluster to operate on.
    upgrade                      Upgrade a service to a newer version.
      --version                  Specify the new version.
```

# Note
This is only a demo of prototype design, it shows how things are working, but not doing the operations for real.
