# singleHost

TiDB, PD, TiKV, TiFlash in the same host.

## Usage

1. Start the box:

   ```bash
   vagrant up
   ```

1. Use [TiUP](https://tiup.io/) to deploy the cluster to the box (only need to do it once):

   ```bash
   tiup cluster deploy singleHost v4.0.4 topology.yaml -i ../_shared/vagrant_key -y --user vagrant
   ```

1. Start the cluster in the box:

   ```bash
   tiup cluster start singleHost
   ```

1. Start TiDB Dashboard server:

   ```bash
   bin/tidb-dashboard --pd http://10.0.1.2:2379
   ```

## Cleanup

```bash
tiup cluster destroy singleHost -y
vagrant destroy --force
```
