TiUP Changelog

## [1.8.2] 2022-01-11

### Fixes

- Fix global configuration not inherited correctly in `scale-out` command of `tiup-cluster` ([#1701](https://github.com/pingcap/tiup/pull/1701), [@srstack](https://github.com/srstack))
- Fix errors starting `tiup-playground` in some circumstances ([#1712](https://github.com/pingcap/tiup/pull/1712) [#1715](https://github.com/pingcap/tiup/pull/1715), [@nexustar](https://github.com/nexustar))
- Fix error comparing nightly versions in `tiup-cluster` ([#1702](https://github.com/pingcap/tiup/pull/1702), [@srstack](https://github.com/srstack))

### Improvements

- Update out-of-date documents of `tiup mirror` ([#1705](https://github.com/pingcap/tiup/pull/1705), [@dveeden](https://github.com/dveeden))

## [1.8.1] 2021-12-20

### Fixes

- Fix port conflict not checked for TiDB clusters imported from `tidb-ansible` on `scale-out` in `tiup-cluster` ([#1656](https://github.com/pingcap/tiup/pull/1656), [@srstack](https://github.com/srstack))
- Fix SSH commands stale in some circumstances ([#1664](https://github.com/pingcap/tiup/pull/1664), [@nexustar](https://github.com/nexustar))
- Fix default value of `initial-commit-ts` for drainer in `tiup-cluster` ([#1678](https://github.com/pingcap/tiup/pull/1678), [@nexustar](https://github.com/nexustar))

### Improvements

- Display check result of CPU frequency governor even when it's not available ([#1663](https://github.com/pingcap/tiup/pull/1663), [@srstack](https://github.com/srstack))

## [1.8.0] 2021-12-06

### New Features

- Add `data-dir` support for TiCDC in `tiup-playground` ([#1631](https://github.com/pingcap/tiup/pull/1631), [@nexustar](https://github.com/nexustar))
- Add support of using custom files as input of `edit-config`, and support dumping the current full config to a file with `show-config` command in `tiup-cluster` ([#1637](https://github.com/pingcap/tiup/pull/1637), [@haiboumich](https://github.com/haiboumich))
- Add support of next-generation monitor (`ng-monitor`) in `tiup-playground` ([#1648](https://github.com/pingcap/tiup/pull/1648), [@nexustar](https://github.com/nexustar))
- Add support of inserting custom `scrape_configs` to Prometheus configs in `tiup-cluster` ([#1641](https://github.com/pingcap/tiup/pull/1641), [@nexustar](https://github.com/nexustar))
- [experimental] Support 2-staged scaling out for `tiup-cluster` ([#1638](https://github.com/pingcap/tiup/pull/1638) [#1642](https://github.com/pingcap/tiup/pull/1642), [@srstack](https://github.com/srstack))
  - Scaling out of a TiDB cluster can be devided with `--stage1` and `--stage2` arguments, the stage 1 deploys files and configs but not starting the new instances, and the stage 2 actually starts the new instances and reload necessary configs
  - This could be useful if you want to modify config of the new instances or use a custom binary with `patch` **before** the first start of the new instances
- [experimental] Implement plain text output and support custom output writer for logs ([#1646](https://github.com/pingcap/tiup/pull/1646), [@AstroProfundis](https://github.com/AstroProfundis))

### Fixes

- Fix incorrect progress bar displaying in some tasks ([#1624](https://github.com/pingcap/tiup/pull/1624), [@nexustar](https://github.com/nexustar))
- Fix incorrect argument flags in `tiup-playground` ([#1635](https://github.com/pingcap/tiup/pull/1635), [@srstack](https://github.com/srstack))
- Fix files of monitoring agents and TiDB audit log not cleaned with `clean` command of `tiup-cluster` ([#1643](https://github.com/pingcap/tiup/pull/1643) [#1644](https://github.com/pingcap/tiup/pull/1644), [@srstack](https://github.com/srstack))
- Fix confirmation prompt in `scale-out` can not be skipped with `--yes` argument in `tiup-cluster` ([#1645](https://github.com/pingcap/tiup/pull/1645), [@srstack](https://github.com/srstack))
- Fix directory conflict error in some circumstances even when node is marked as `ignore_exporter` ([#1649](https://github.com/pingcap/tiup/pull/1649), [@AstroProfundis](https://github.com/AstroProfundis))
- Fix DM nodes not added to node_exporter target list in Prometheus config in `tiup-dm` ([#1654](https://github.com/pingcap/tiup/pull/1654), [@AstroProfundis](https://github.com/AstroProfundis))

### Improvements

- Add significant warning when `--force` argument is set for `scale-in` command in `tiup-cluster` ([#1629](https://github.com/pingcap/tiup/pull/1629), [@AstroProfundis](https://github.com/AstroProfundis))
- Add environment variables to skip topology sanity check in `scale-in` command in `tiup-cluster` ([#1627](https://github.com/pingcap/tiup/pull/1627), [@AstroProfundis](https://github.com/AstroProfundis))
- Update examples to use `--without-monitor` instead of `--monitor` for `tiup-playground` ([#1639](https://github.com/pingcap/tiup/pull/1639), [@dveeden](https://github.com/dveeden))

## [1.7.0] 2021-11-15

### New Features

- Support deploying and managing TLS enabled TiDB cluster with TiFlash nodes ([#1594](https://github.com/pingcap/tiup/pull/1594), [@nexustar](https://github.com/nexustar))
- Support rendering template for local deployment with vairables in `tiup-cluster` and `tiup-dm` ([#1596](https://github.com/pingcap/tiup/pull/1596), [@makocchi-git](https://github.com/makocchi-git))
- [experimental] Support optionally enable next-generation monitor (`ng-monitor`) for latest TiDB releases ([#1601](https://github.com/pingcap/tiup/pull/1601), [@nexustar](https://github.com/nexustar))
- [experimental] Support JSON output format for `tiup-cluster` and `tiup-dm` ([#1617](https://github.com/pingcap/tiup/pull/1617), [@AstroProfundis](https://github.com/AstroProfundis))

### Fixes

- Remove warning about tag argument for `tiup-playground` ([#1606](https://github.com/pingcap/tiup/pull/1606), [@nexustar](https://github.com/nexustar))
- Set `--external-url` for AlertManager in `tiup-cluster` ([#1608](https://github.com/pingcap/tiup/pull/1608), [@reAsOn2010](https://github.com/reAsOn2010))
- Fix auto detecting of system arch fail in certain circumstances ([#1610](https://github.com/pingcap/tiup/pull/1610), [@AstroProfundis](https://github.com/AstroProfundis))

### Improvements

- Support getting cluster ID from PD in `pdapi` package ([#1573](https://github.com/pingcap/tiup/pull/1573) [#1574](https://github.com/pingcap/tiup/pull/1574), [@nexustar](https://github.com/nexustar); [#1580](https://github.com/pingcap/tiup/pull/1580), [@AstroProfundis](https://github.com/AstroProfundis))
- Accurately get status of TiFlash nodes during operations ([#1600](https://github.com/pingcap/tiup/pull/1600), [@AstroProfundis](https://github.com/AstroProfundis))

## [1.6.1] 2021-10-21

### Fixes

- Fix `tiup-bench` reporting wrong latency for TPCC workloads ([#1577](https://github.com/pingcap/tiup/pull/1577), [@lobshunter](https://github.com/lobshunter))
- Fix test cases for `tiup-bench` and `tiup-client` ([#1579](https://github.com/pingcap/tiup/pull/1579), [@AstroProfundis](https://github.com/AstroProfundis))
- Fix fetching component manifest error on certain circumstances ([#1581](https://github.com/pingcap/tiup/pull/1581), [@nexustar](https://github.com/nexustar))

## [1.6.0] 2021-10-09

### New Features

- Add support of using `ssh-agent` auth socket in `tiup-cluster` ([#1416](https://github.com/pingcap/tiup/pull/1416), [@9547](https://github.com/9547))
- Add parallel task concurrency control in `tiup-cluster` and `tiup-dm` with `-c/--concurrency` argument ([#1420](https://github.com/pingcap/tiup/pull/1420), [@AstroProfundis](https://github.com/AstroProfundis))
  - The default value of max number of parallel tasks allowed is 5, this feature could help users managing very large clusters to avoid connection errors on operations.
- Add the ability to detect CPU arch for deployment servers automatically in `tiup-cluster` and `tiup-dm` if not set by user ([#1423](https://github.com/pingcap/tiup/pull/1423), [@9547](https://github.com/9547))
- Add `renew` subcommand for `tiup mirror` to extend the expiration date of component manifest ([#1479](https://github.com/pingcap/tiup/pull/1479), [@AstroProfundis](https://github.com/AstroProfundis))
- Add the ability to ignore monitor agents for specific instances in `tiup-cluster` ([#1492](https://github.com/pingcap/tiup/pull/1492), [@AstroProfundis](https://github.com/AstroProfundis))
- Add `--force` argument for `prune` subcommand in `tiup-cluster` ([#1552](https://github.com/pingcap/tiup/pull/1552), [@AstroProfundis](https://github.com/AstroProfundis))
- Add more configuration fields for Grafana in `tiup-cluster` and `tiup-dm` ([#1566](https://github.com/pingcap/tiup/pull/1566), [@haiboumich](https://github.com/haiboumich))
- [Experimental] Add support of SSH connections via proxy in `tiup-cluster` ([#1438](https://github.com/pingcap/tiup/pull/1438), [@9547](https://github.com/9547))
- Deprecate the `--monitor` argument and introduce a new `--without-monitor` argument to disable monitoring components in `tiup-playground` ([#1512](https://github.com/pingcap/tiup/pull/1512), [@LittleFall](https://github.com/LittleFall))
- Deprecate the `TIUP_WORK_DIR` environment as it's not actually been used, and make it possible for `tiup-playground` to run without `tiup` ([#1553](https://github.com/pingcap/tiup/pull/1553) [#1556](https://github.com/pingcap/tiup/pull/1556) [#1558](https://github.com/pingcap/tiup/pull/1558), [@nexustar](https://github.com/nexustar))

### Fixes

- Fix `blackbox_exporter` configs for TLS enabled clusters in `tiup-cluster` ([#1443](https://github.com/pingcap/tiup/pull/1443), [@9547](https://github.com/9547))
- Only try to apply THP fix if it's available on the deployment server in `tiup-cluster` ([#1458](https://github.com/pingcap/tiup/pull/1458), [@9547](https://github.com/9547))
- Fix sudo errors in `tiup-cluster` when devtoolset is enabled on deployment server ([#1516](https://github.com/pingcap/tiup/pull/1516), [@nexustar](https://github.com/nexustar))
- Fix test cases for `tiup-dm` ([#1540](https://github.com/pingcap/tiup/pull/1540), [@nexustar](https://github.com/nexustar))
- Fix downloading of uneeded component packages when `--binpath` is specifiedin `tiup-playground` ([#1495](https://github.com/pingcap/tiup/pull/1495), [@AstroProfundis](https://github.com/AstroProfundis); [#1545](https://github.com/pingcap/tiup/pull/1545), [@nexustar](https://github.com/nexustar))
- Fix panic when `tiup-bench` fails to connect to the database ([#1557](https://github.com/pingcap/tiup/pull/1557), [@nexustar](https://github.com/nexustar))
- Fix `numa_node` configs are not rendered into PD startup script in `tiup-cluster` ([#1565](https://github.com/pingcap/tiup/pull/1565), [@onlyacat](https://github.com/onlyacat))
- Correctly handle `--` in command line arguments passed to `tiup` ([#1569](https://github.com/pingcap/tiup/pull/1569), [@dveeden](https://github.com/dveeden))

### Improvements

- Reduce network usage on various operations and speed up the process
  - Update component manifests simultaneously in `tiup list` ([#1532](https://github.com/pingcap/tiup/pull/1532), [@nexustar](https://github.com/nexustar))
  - Not requesting manifest before actually using the component ([#1539](https://github.com/pingcap/tiup/pull/1539), [@nexustar](https://github.com/nexustar))
  - Update `root.json` only when necessary ([#1554](https://github.com/pingcap/tiup/pull/1554), [@nexustar](https://github.com/nexustar))
- Use the value of `--wait-timeout` argument as timeout of SSH command operations with the `builtin` executor ([#1445](https://github.com/pingcap/tiup/pull/1445), [@AstroProfundis](https://github.com/AstroProfundis))
- Refuse to `clone` a local mirror to the same location it is stored ([#1464](https://github.com/pingcap/tiup/pull/1464), [@dveeden](https://github.com/dveeden))
- Set terminal title to show session tag in `tiup-playground` ([#1506](https://github.com/pingcap/tiup/pull/1506), [@dveeden](https://github.com/dveeden))
- Show TiDB port when scale out in `tiup-playground` ([#1520](https://github.com/pingcap/tiup/pull/1520), [@nexustar](https://github.com/nexustar))
- Cleanup files if component fails to install ([#1562](https://github.com/pingcap/tiup/pull/1562), [@nexustar](https://github.com/nexustar))
- Update docs and examples ([#1484](https://github.com/pingcap/tiup/pull/1484), [@ichn-hu](https://github.com/ichn-hu); [#1502](https://github.com/pingcap/tiup/pull/1502), [@AstroProfundis](https://github.com/AstroProfundis))
- Use auto completion from `cobra` itself ([#1544](https://github.com/pingcap/tiup/pull/1544), [@AstroProfundis](https://github.com/AstroProfundis); [#1549](https://github.com/pingcap/tiup/pull/1549), [@nexustar](https://github.com/nexustar))

## [1.5.6] 2021-09-01

### Fixes

- Fix OS version check rules for `tiup-cluster check` ([#1535](https://github.com/pingcap/tiup/pull/1535), [@AstroProfundis](https://github.com/AstroProfundis))
- Fix component upgrade order for `tiup-cluster` to make sure TiCDC nodes work correctly ([#1542](https://github.com/pingcap/tiup/pull/1542), [@overvenus](https://github.com/overvenus))

### Improvements

- Adjust warning message of `tiup-cluster restart` to make users clear that the cluster will be unavailable during the process ([#1523](https://github.com/pingcap/tiup/pull/1523), [@glkappe](https://github.com/glkappe))
- Reverse the order of audit log listing to show latest records at the buttom ([#1538](https://github.com/pingcap/tiup/pull/1538), [@AstroProfundis](https://github.com/AstroProfundis))

## [1.5.5] 2021-08-19

### Fixes

- Fix error when reloading a stopped cluster with `--skip-restart` argument ([#1513](https://github.com/pingcap/tiup/pull/1513), [@AstroProfundis](https://github.com/AstroProfundis))
- Use absolute path for `sudo` command, to workaround errors on systems where `devtoolset` is enabled ([#1516](https://github.com/pingcap/tiup/pull/1516), [@nexustar](https://github.com/nexustar))
- Fix custom TiDB port not correctly set in playground ([#1511](https://github.com/pingcap/tiup/pull/1511), [@hecomlilong](https://github.com/hecomlilong))

### Improvements

- Adjust suggested argument order in playground examples ([#1522](https://github.com/pingcap/tiup/pull/1522), [@glkappe](https://github.com/glkappe))

## [1.5.4] 2021-08-05

### Fixes

- Allow editing of `lerner_config` field in TiFlash spec ([#1494](https://github.com/pingcap/tiup/pull/1494), [@AstroProfundis](https://github.com/AstroProfundis))
- Fix incorrect timeout for telemetry requests ([#1500](https://github.com/pingcap/tiup/pull/1500), [@AstroProfundis](https://github.com/AstroProfundis))
- Ingore `data_dir` of monitor agents when checking for directory overlaps ([#1510](https://github.com/pingcap/tiup/pull/1510), [@AstroProfundis](https://github.com/AstroProfundis))

### Improvements

- Distinguish cookie names of multiple grafana instances on the same host ([#1491](https://github.com/pingcap/tiup/pull/1491), [@AstroProfundis](https://github.com/AstroProfundis))

## [1.5.3] 2021-07-15

### Fixes

- Fix incorrect alert rules for TiDB version 3.x ([#1463](https://github.com/pingcap/tiup/pull/1463), [@9547](https://github.com/9547))
- Fix TiKV config check to correctly handle the `data_dir` value ([#1471](https://github.com/pingcap/tiup/pull/1471), [@tabokie](https://github.com/tabokie))

### Improvements

- Update dependencies and adjust error message of `ctl` ([#1459](https://github.com/pingcap/tiup/pull/1459), [@AstroProfundis](https://github.com/AstroProfundis))
- Use `$SHELL` environment variable for completion ([#1455](https://github.com/pingcap/tiup/pull/1455), [@dveeden](https://github.com/dveeden))
- Allow listing components from local cached manifests without network access ([#1466](https://github.com/pingcap/tiup/pull/1466), [@c4pt0r](https://github.com/c4pt0r))
- Adjust error message of SELinux check failure ([#1476](https://github.com/pingcap/tiup/pull/1476), [@AstroProfundis](https://github.com/AstroProfundis))
- Adjust warning message when `scale-in` with `--force` argument to make it more clear of potential risks ([#1477](https://github.com/pingcap/tiup/pull/1477), [@AstroProfundis](https://github.com/AstroProfundis))

## [1.5.2] 2021-06-24

### Fixes

- Fix native SSH not working with custom SSH port ([#1424](https://github.com/pingcap/tiup/pull/1424), [@9547](https://github.com/9547))
- Fix dashboard address displaying issue for `tikv-slim` clusters ([#1428](https://github.com/pingcap/tiup/pull/1428), [@iosmanthus](https://github.com/iosmanthus))
- Fix a typo in help message of `tiup-playground` ([#1429](https://github.com/pingcap/tiup/pull/1429), [@ekexium](https://github.com/ekexium))
- Fix TiFlash nodes not handled correctly in some commands ([#1431](https://github.com/pingcap/tiup/pull/1431), [@lucklove](https://github.com/lucklove))
- Fix jemalloc config for TiKV nodes ([#1435](https://github.com/pingcap/tiup/pull/1435), [@9547](https://github.com/9547))
- Fix the issue that slow log is not placed under `log_dir` ([#1441](https://github.com/pingcap/tiup/pull/1441), [@lucklove](https://github.com/lucklove))

### Improvements

- Update default alertmanager config template to avoid confusing ([#1425](https://github.com/pingcap/tiup/pull/1425) [#1426](https://github.com/pingcap/tiup/pull/1426), [@lucklove](https://github.com/lucklove))
- Increase default timeout of transferring leader in upgrade progress ([#1434](https://github.com/pingcap/tiup/pull/1434), [@AstroProfundis](https://github.com/AstroProfundis))
- Update dependencies ([#1433](https://github.com/pingcap/tiup/pull/1433), [@AstroProfundis](https://github.com/AstroProfundis))

## [1.5.1] 2021-06-11

### Fix

- Fix the issue that some versions of TiCDC node may fail to start in `tiup-cluster` ([#1421](https://github.com/pingcap/tiup/pull/1421), [@JinLingChristopher](https://github.com/JinLingChristopher))

## [1.5.0] 2021-06-09

### New Features

- Show more information in `display` subcommand of `tiup-cluster`
  - Add an `--uptime` argument to show time since the last state change of process ([#1231](https://github.com/pingcap/tiup/pull/1231), [@9547](https://github.com/9547))
  - Show deploy user in `display` output and adjust formats ([#1390](https://github.com/pingcap/tiup/pull/1390) [#1409](https://github.com/pingcap/tiup/pull/1409), [@AstroProfundis](https://github.com/AstroProfundis))
- Add JSON output for `display` subcommand of `tiup-cluster` ([#1358](https://github.com/pingcap/tiup/pull/1358), [@dveeden](https://github.com/dveeden))
- Add double confirmation for `scale-out` subcommand in `tiup-cluster` to let users be aware of global configs being used ([#1309](https://github.com/pingcap/tiup/pull/1309), [@AstroProfundis](https://github.com/AstroProfundis))
- Support deploying pure TiKV cluster with `--mode tikv-slim` in `playground` ([#1333](https://github.com/pingcap/tiup/pull/1333), [@iosmanthus](https://github.com/iosmanthus); [#1365](https://github.com/pingcap/tiup/pull/1365), [@tisonkun](https://github.com/tisonkun))
- Support data dir settings for TiCDC in `tiup-cluster` ([#1372](https://github.com/pingcap/tiup/pull/1372), [@JinLingChristopher](https://github.com/JinLingChristopher))
- Support change of `GCTTL` and `TZ` configs for TiCDC in `tiup-cluster` ([#1380](https://github.com/pingcap/tiup/pull/1380), [@amyangfei](https://github.com/amyangfei))
- Add a local deployment template for `tiup-cluster` ([#1404](https://github.com/pingcap/tiup/pull/1404), [@kolbe](https://github.com/kolbe))
- Support using dot (`.`) in cluster name ([#1412](https://github.com/pingcap/tiup/pull/1412), [@9547](https://github.com/9547))

### Fixes

- Fix a variety of typos ([#1306](https://github.com/pingcap/tiup/pull/1306), [@kolbe](https://github.com/kolbe))
- Fix non-common speed units shown in downloading progress ([#1312](https://github.com/pingcap/tiup/pull/1312), [@dveeden](https://github.com/dveeden))
- Fix the issue that it may panic when user try to list expired component ([#1391](https://github.com/pingcap/tiup/pull/1391), [@lucklove](https://github.com/lucklove))
- Fix the issue that tikv not upgraded on error increasing schedule limit  ([#1401](https://github.com/pingcap/tiup/pull/1401), [@AstroProfundis](https://github.com/AstroProfundis))

### Improvements

- Support specifying node counts in tests ([#1251](https://github.com/pingcap/tiup/pull/1251), [@9547](https://github.com/9547))
- Add double confirmation for `reload`, `patch` and `rename` subcommands in `tiup-cluster` ([#1263](https://github.com/pingcap/tiup/pull/1263), [@9547](https://github.com/9547))
- Add ability to list available make targets for developers ([#1277](https://github.com/pingcap/tiup/pull/1277), [@rkazak](https://github.com/rkazak))
- Update links in doc/dev/README.md file ([#1296](https://github.com/pingcap/tiup/pull/1296), [@mjonss](https://github.com/mjonss))
- Improve handling of latest versions in `mirror clone` subcommand ([#1313](https://github.com/pingcap/tiup/pull/1313), [@dveeden](https://github.com/dveeden))
- Add check for dependencies before downloading package in installation script ([#1348](https://github.com/pingcap/tiup/pull/1348), [@AstroProfundis](https://github.com/AstroProfundis))
- Simplified the handling of configs imported from TiDB-Ansible ([#1350](https://github.com/pingcap/tiup/pull/1350), [@lucklove](https://github.com/lucklove))
- Implement native scp downloading ([#1382](https://github.com/pingcap/tiup/pull/1382), [@AstroProfundis](https://github.com/AstroProfundis))
- Update and fix dependencies ([#1362](https://github.com/pingcap/tiup/pull/1362), [@AstroProfundis](https://github.com/AstroProfundis); [#1407](https://github.com/pingcap/tiup/pull/1407), [@dveeden](https://github.com/dveeden))

## [1.4.4] 2021.05.26

### Fixes

- Fix the issue that upgrade process may fail if the PD node is not available for longer than normal after restart ([#1359](https://github.com/pingcap/tiup/pull/1359), [@AstroProfundis](https://github.com/AstroProfundis))
- Fix incorrect `MALLOC_CONF` value for TiKV node, set `prof_active` to `false` ([#1361](https://github.com/pingcap/tiup/pull/1361) [#1369](https://github.com/pingcap/tiup/pull/1369), [@YangKeao](https://github.com/YangKeao))
  - Risk of this issue: Generating prof data for TiKV node with `prof_active=true` may cause high CPU systime usage in some circumstances, users need to regenerate startup scripts for TiKV nodes with `tiup cluster reload <cluster-name> -R tikv` to make the update applied
- Fix the issue that the global `log_dir` not generated correctly for absolute paths ([#1376](https://github.com/pingcap/tiup/pull/1376), [@lucklove](https://github.com/lucklove))
- Fix the issue that `display` command may report label mismatch warning if `placement-rule` is enabled ([#1378](https://github.com/pingcap/tiup/pull/1378), [@lucklove](https://github.com/lucklove))
- Fix the issue that SELinux setting is incorrect when `tiup-cluster` tries to disable it with `check --apply` ([#1383](https://github.com/pingcap/tiup/pull/1383), [@AstroProfundis](https://github.com/AstroProfundis))
- Fix the issue that when scaling out instance on a host imported from `tidb-ansible`, the process may report error about monitor directory conflict ([#1386](https://github.com/pingcap/tiup/pull/1386), [@lucklove](https://github.com/lucklove))

### Improvements

- Allow scale in cluster when there is no TiSpark master node but have worker node in the topology ([#1363](https://github.com/pingcap/tiup/pull/1363), [@AstroProfundis](https://github.com/AstroProfundis))
- Make port check error message more clear to users ([#1367](https://github.com/pingcap/tiup/pull/1367), [@JinLingChristopher](https://github.com/JinLingChristopher))

## [1.4.3] 2021.05.13

### Fixes

- Fix OS check for RHEL in `tiup-cluster` ([#1336](https://github.com/pingcap/tiup/pull/1336), [@AstroProfundis](https://github.com/AstroProfundis))
- Check for command depends before downloading packages in install script ([#1348](https://github.com/pingcap/tiup/pull/1348), [@AstroProfundis](https://github.com/AstroProfundis))
- Fix the issue that install script downloads an old TiUP package ([#1349](https://github.com/pingcap/tiup/pull/1349), [@lucklove](https://github.com/lucklove))
- Fix the issue that drainer node imported from TiDB-Ansible may have incorrect `data_dir` ([#1346](https://github.com/pingcap/tiup/pull/1346), [@AstroProfundis](https://github.com/AstroProfundis))

### Improvements

- Optimize some subcommands of `tiup mirror` ([#1331](https://github.com/pingcap/tiup/pull/1331), [@AstroProfundis](https://github.com/AstroProfundis))
- Set proper User-Agent for requests downloading manifests and files from remote ([#1342](https://github.com/pingcap/tiup/pull/1342), [@AstroProfundis](https://github.com/AstroProfundis))
- Add basic telemetry report for `tiup` and `playground` ([#1341](https://github.com/pingcap/tiup/pull/1341) [#1353](https://github.com/pingcap/tiup/pull/1353), [@AstroProfundis](https://github.com/AstroProfundis))

## [1.4.2] 2021.04.26

### Fixes

- Send meta output from `tiup` to `stderr` to not to mix with output of components ([#1298](https://github.com/pingcap/tiup/pull/1298), [@dveeden](https://github.com/dveeden))
- Update confusing version selection examples in help message of `playground` ([#1318](https://github.com/pingcap/tiup/pull/1318), [@AstroProfundis](https://github.com/AstroProfundis))
- Fix the issue that `tiup mirror clone` command does exclude yanked component correctly ([#1321](https://github.com/pingcap/tiup/pull/1321), [@lucklove](https://github.com/lucklove))

### Improvements

- Adjust output messages and operation processes of `tiup mirror` command ([#1302](https://github.com/pingcap/tiup/pull/1302), [@AstroProfundis](https://github.com/AstroProfundis))
- Add `tiup mirror show` subcommand to display current mirror address in use ([#1317](https://github.com/pingcap/tiup/pull/1317), [@baurine](https://github.com/baurine))
- Optimize error handling if `root.json` fails to load ([#1303](https://github.com/pingcap/tiup/pull/1303), [@AstroProfundis](https://github.com/AstroProfundis))
- Update MySQL client connection example in `playground` ([#1323](https://github.com/pingcap/tiup/pull/1323), [@tangenta](https://github.com/tangenta))
- Adjust data and fields report via telemetry ([#1327](https://github.com/pingcap/tiup/pull/1327), [@AstroProfundis](https://github.com/AstroProfundis))

## [1.4.1] 2021.04.07

### Fixes

- Fix pprof failing for TiKV in playground ([#1272](https://github.com/pingcap/tiup/pull/1272), [@hicqu](https://github.com/hicqu))
- Fix the issue that TiFlash node may be failed to restart in playground ([#1280](https://github.com/pingcap/tiup/pull/1280), [@lucklove](https://github.com/lucklove))
- Fix the issue that `binlog_enable` is not imported from tidb-ansible correctly ([#1261](https://github.com/pingcap/tiup/pull/1261), [@lucklove](https://github.com/lucklove))
- Fix directory conflict check error for TiDB and DM clusters imported from ansible deployment ([#1273](https://github.com/pingcap/tiup/pull/1273), [@lucklove](https://github.com/lucklove))
- Fix compatibility issue during upgrade for PD v3.x ([#1274](https://github.com/pingcap/tiup/pull/1274), [@lucklove](https://github.com/lucklove))
- Fix failure of parsing very long audit log in replay for tiup-cluster ([#1259](https://github.com/pingcap/tiup/pull/1259), [@lucklove](https://github.com/lucklove))
- Fix log dir path of Grafana for tiup-cluster ([#1276](https://github.com/pingcap/tiup/pull/1276), [@rkazak](https://github.com/rkazak))
- Fix config check error when the cluster was deployed with an legacy nightly version in tiup-cluster ([#1281](https://github.com/pingcap/tiup/pull/1281), [@AstroProfundis](https://github.com/AstroProfundis))
- Fix error when using nightly version while the actual component is not available in repo ([#1294](https://github.com/pingcap/tiup/pull/1294), [@lucklove](https://github.com/lucklove))

### Improvements

- Refine PD scaling script rendering to optimize the code ([#1253](https://github.com/pingcap/tiup/pull/1253), [@9547](https://github.com/9547))
- Start PD and DM master nodes sequentially in ([#1262](https://github.com/pingcap/tiup/pull/1262), [@9547](https://github.com/9547))
- Properly follow the ignore config check argument in reload for tiup-cluster ([#1265](https://github.com/pingcap/tiup/pull/1265), [@9547](https://github.com/9547))

## [1.4.0] 2021.03.31

### New Features

- EXPERIMENTAL: Add support of Apple M1 devices ([#1122](https://github.com/pingcap/tiup/issues/1122), [@terasum](https://github.com/terasum) [@AstroProfundis](https://github.com/AstroProfundis) [@sunxiaoguang](https://github.com/sunxiaoguang))
  - Playground may not fully work as some components don't yet have packages for `darwin-arm64` released
- Not displaying dashboard address if it's "none" or "auto" ([#1054](https://github.com/pingcap/tiup/pull/1054), [@9547](https://github.com/9547))
- Support filtering nodes and roles in `check` subcommand of tiup-cluster ([#1030](https://github.com/pingcap/tiup/pull/1030), [@AstroProfundis](https://github.com/AstroProfundis))
- Support retry of failed operations from where it broke with `replay` command of tiup-cluster and tiup-dm ([#1069](https://github.com/pingcap/tiup/pull/1069) [#1157](https://github.com/pingcap/tiup/pull/1157), [@lucklove](https://github.com/lucklove))
- Support upgrade and patch a stopped TiDB / DM cluster ([#1096](https://github.com/pingcap/tiup/pull/1096), [@lucklove](https://github.com/lucklove))
- Support setting global custom values for topology of tiup-cluster ([#1098](https://github.com/pingcap/tiup/pull/1098), [@lucklove](https://github.com/lucklove))
- Support custom `root_url` and anonymous login for Grafana in tiup-cluster ([#1085](https://github.com/pingcap/tiup/pull/1085), [@mianhk](https://github.com/mianhk))
- Support remote read and remote write for Prometheus node in tiup-cluster ([#1070](https://github.com/pingcap/tiup/pull/1070), [@XSHui](https://github.com/XSHui))
- Support custom external AlertManager target for Prometheus node in tiup-cluster ([#1149](https://github.com/pingcap/tiup/pull/1149), [@lucklove](https://github.com/lucklove))
- Support force reinstallation of already installed component ([#1145](https://github.com/pingcap/tiup/pull/1145), [@9547](https://github.com/9547))
- Add `--force` and retain data options to tiup-dm ([#1080](https://github.com/pingcap/tiup/pull/1080), [@9547](https://github.com/9547))
- Add `enable`/`disable` subcommands to tiup-dm ([#1114](https://github.com/pingcap/tiup/pull/1114), [@9547](https://github.com/9547))
- Add `template` subcommand to tiup-cluster to print pre-defined topology templates ([#1156](https://github.com/pingcap/tiup/pull/1156), [@lucklove](https://github.com/lucklove))
- Add `--version` option to `display` subcommand of tiup-cluster to print the cluster version ([#1207](https://github.com/pingcap/tiup/pull/1207), [@AstroProfundis](https://github.com/AstroProfundis))
- Allow value type change when editing topology with `edit-config` subcommand of tiup-cluster ([#1050](https://github.com/pingcap/tiup/pull/1050), [@AstroProfundis](https://github.com/AstroProfundis))

### Fixes

- Not allowing deployment if the input topology file is empty ([#994](https://github.com/pingcap/tiup/pull/994), [@AstroProfundis](https://github.com/AstroProfundis))
- Fix data dir setting for Prometheus ([#1040](https://github.com/pingcap/tiup/pull/1040), [@9547](https://github.com/9547))
- Fix the issue that pre-defined Prometheus rules may be missing if a custom `rule_dir` is set ([#1073](https://github.com/pingcap/tiup/pull/1073), [@9547](https://github.com/9547))
- Fix the issue that config files of Prometheus and Grafana are not checked before start ([#1074](https://github.com/pingcap/tiup/pull/1074), [@9547](https://github.com/9547))
- Fix the issue that cluster name is not validated for some operations ([#1177](https://github.com/pingcap/tiup/pull/1177), [@AstroProfundis](https://github.com/AstroProfundis))
- Fix the issue that tiup-cluster reloads a cluster even if the config may contain errors ([#1183](https://github.com/pingcap/tiup/pull/1183), [@9547](https://github.com/9547))
- Fix the issue that `publish` command may fail when uploading files without retry ([#1174](https://github.com/pingcap/tiup/pull/1174) [#1202](https://github.com/pingcap/tiup/pull/1202), [@AstroProfundis](https://github.com/AstroProfundis); [#1167](https://github.com/pingcap/tiup/pull/1163), [@lucklove](https://github.com/lucklove))
- Fix the issue that newly added TiFlash nodes may fail to start during `scale-out` in tiup-cluster ([#1227](https://github.com/pingcap/tiup/pull/1227), [@9547](https://github.com/9547))
- Fix incorrect cluster name in alert messages ([#1238](https://github.com/pingcap/tiup/pull/1238), [@9547](https://github.com/9547))
- Fix the issue that blackbox_exporter may not collecting ping metrics correctly ([#1250](https://github.com/pingcap/tiup/pull/1250), [@STRRL](https://github.com/STRRL))

### Improvements

- Reduce jitter during upgrade process of TiDB cluster
  - Make sure PD node is online and serving before upgrading the next one ([#1032](https://github.com/pingcap/tiup/pull/1032), [@HunDunDM](https://github.com/HunDunDM))
  - Upgrade PD leader node after upgrading other PD nodes ([#1086](https://github.com/pingcap/tiup/pull/1086), [@AstroProfundis](https://github.com/AstroProfundis))
  - Increase schedule limit during upgrade of TiKV nodes ([#1661](https://github.com/pingcap/tiup/pull/1161), [@AstroProfundis](https://github.com/AstroProfundis))
  - Add check to validate if all regions are healthy ([#1126](https://github.com/pingcap/tiup/pull/1126), [@AstroProfundis](https://github.com/AstroProfundis))
- Only reload Prometheus configs when needed ([#989](https://github.com/pingcap/tiup/pull/989), [@9547](https://github.com/9547))
- Show default option on prompted input messages ([#1132](https://github.com/pingcap/tiup/pull/1132) [#1134](https://github.com/pingcap/tiup/pull/1134), [@wangbinhe3db](https://github.com/wangbinhe3db))
- Include user's input in error message if prompted challenge didn't pass ([#1104](https://github.com/pingcap/tiup/pull/1104), [@AstroProfundis](https://github.com/AstroProfundis))
- Check for `data_dir` and `log_dir` overlap before deploying a cluster ([#1093](https://github.com/pingcap/tiup/pull/1093), [@9547](https://github.com/9547))
- Improve checking rules in `tiup cluster check` command ([#1099](https://github.com/pingcap/tiup/pull/1099) [#1107](https://github.com/pingcap/tiup/pull/1107), [@AstroProfundis](https://github.com/AstroProfundis); [#1118](https://github.com/pingcap/tiup/pull/1118) [#1124](https://github.com/pingcap/tiup/pull/1124), [@9547](https://github.com/9547))
- Refine `list` and `display` command for tiup-cluster ([#1139](https://github.com/pingcap/tiup/pull/1139), [@baurine](https://github.com/baurine))
- Mark patched nodes in `display` output of tiup-cluster and tiup-dm ([#1125](https://github.com/pingcap/tiup/pull/1125), [@AstroProfundis](https://github.com/AstroProfundis))
- Ignore `users.*` settings for TiFlash if the cluster version is later than v4.0.12 and v5.0.0-rc ([#1211](https://github.com/pingcap/tiup/pull/1211), [@JaySon-Huang](https://github.com/JaySon-Huang))
- Cache `timestamp` manifest in memory to reduce network requests ([#1212](https://github.com/pingcap/tiup/pull/1212), [@lucklove](https://github.com/lucklove))
- Upgrade toolchain to Go 1.16 ([#1151](https://github.com/pingcap/tiup/pull/1151) [#1153](https://github.com/pingcap/tiup/pull/1153) [#1130](https://github.com/pingcap/tiup/pull/1130), [@AstroProfundis](https://github.com/AstroProfundis))
- Use GitHub Actions to build and release TiUP components ([#1158](https://github.com/pingcap/tiup/pull/1158), [@AstroProfundis](https://github.com/AstroProfundis))
- Remove deprecated `v0manifest` support, TiUP version before v1.0.0 may not be able to download latest packages anymore ([#906](https://github.com/pingcap/tiup/issues/906))

## [1.3.7] 2021.03.25

### Fixes

- Fix the issue that metrics of tiflash-server instance may not collected correctly ([#1083](https://github.com/pingcap/tiup/pull/1083), [@yuzhibotao](https://github.com/yuzhibotao))
- Fix the issue that tiup-cluster disables monitoring services unexpectedly ([#1088](https://github.com/pingcap/tiup/pull/1088), [@lucklove](https://github.com/lucklove))
- Fix wrong dashboard name for lightning in Grafana after renaming a cluster with tiup-cluster ([#1196](https://github.com/pingcap/tiup/pull/1196), [@9547](https://github.com/9547))
- Fix the issue that tiup-cluster `prune` command may try to generate config for removed nodes ([#1237](https://github.com/pingcap/tiup/pull/1237), [@lucklove](https://github.com/lucklove))

## [1.3.6] 2021.03.19

### Fixes

- Fix the issue that can't deploy arm64 binary with offline mirror ([#1229](https://github.com/pingcap/tiup/pull/1229), [@lucklove](https://github.com/lucklove))

## [1.3.5] 2021.03.11

### Fixes

- Fix the issue that old nighlty may cause error ([#1198](https://github.com/pingcap/tiup/pull/1198), [@lucklove](https://github.com/lucklove))

## [1.3.4] 2021.03.05

### Fixes

- Fix the issue that tiup-cluster can't gernerate prometheus config ([#1185](https://github.com/pingcap/tiup/pull/1185), [@lucklove](https://github.com/lucklove))
- Fix the issue that tiup may choose yanked version if it's already installed ([#1191](https://github.com/pingcap/tiup/pull/1191), [@lucklove](https://github.com/lucklove))

## [1.3.3] 2021.03.04

### Fixes

- Fix the issue that tiup will hang forever when reloading a stopped cluster ([#1044](https://github.com/pingcap/tiup/pull/1044), [@9547](https://github.com/9547))
- Fix the issue that `tiup mirror merge` does not work on official offline package ([#1121](https://github.com/pingcap/tiup/pull/1121), [@lucklove](https://github.com/lucklove))
- Fix the issue that there may be no retry when download component failed ([#1137](https://github.com/pingcap/tiup/pull/1137), [@lucklove](https://github.com/lucklove))
- Fix the issue that PD dashboard does not report grafana address in playground ([#1142](https://github.com/pingcap/tiup/pull/1142), [@9547](https://github.com/9547))
- Fix the issue that the default selected version may be a preprelease version ([#1128](https://github.com/pingcap/tiup/pull/1128), [@lucklove](https://github.com/lucklove))
- Fix the issue that the error message is confusing when the patched tar is not correct ([#1175](https://github.com/pingcap/tiup/pull/1175), [@lucklove](https://github.com/lucklove))

### Improvements

- Add darwin-arm64 not support hint in install script ([#1123](https://github.com/pingcap/tiup/pull/1123), [@terasum](https://github.com/terasum))
- Improve playground welcome information for connecting TiDB ([#1133](https://github.com/pingcap/tiup/pull/1133), [@dveeden](https://github.com/dveeden))
- Bind latest stable grafana and prometheus in DM deploying ([#1129](https://github.com/pingcap/tiup/pull/1129), [@lucklove](https://github.com/lucklove))
- Use the advertised host instead of 0.0.0.0 for tiup-playground ([#1152](https://github.com/pingcap/tiup/pull/1152), [@9547](https://github.com/9547))
- Check tarball checksum on tiup-server when publish component ([#1163](https://github.com/pingcap/tiup/pull/1163), [@lucklove](https://github.com/lucklove))

## [1.3.2] 2021.01.29

### Fixes

- Fix the issue that the grafana and alertmanager target not set in prometheus.yaml ([#1041](https://github.com/pingcap/tiup/pull/1041), [@9547](https://github.com/9547))
- Fix the issue that grafana deployed by tiup-dm missing home.json ([#1056](https://github.com/pingcap/tiup/pull/1056), [@lucklove](https://github.com/lucklove))
- Fix the issue that the expires of cloned mirror is shourened after publish component to it ([#1051](https://github.com/pingcap/tiup/pull/1051), [@lucklove](https://github.com/lucklove))
- Fix the issue that tiup-cluster may remove wrong paths for imported cluster on scale-in ([#1068](https://github.com/pingcap/tiup/pull/1068), [@AstroProfundis](https://github.com/AstroProfundis))
  - Risk of this issue: If an imported cluster has deploy dir ending with `/`, and sub dirs as `<deploy-dir>//sub`, it could results to delete wrong paths on scale-in
- Fix the issue that imported `*_exporter` has wrong binary path ([#1101](https://github.com/pingcap/tiup/pull/1101), [@AstroProfundis](https://github.com/AstroProfundis))

### Improvements

- Apply more strict check on tar.gz file for `patch` command: check if the entry is an executable file ([#1091](https://github.com/pingcap/tiup/pull/1091), [@lucklove](https://github.com/lucklove))

## [1.3.1] 2020.12.31

### Fixes

- Workaround the issue that store IDs in PDs may not monotonically assigned ([#1011](https://github.com/pingcap/tiup/pull/1011), [@AstroProfundis](https://github.com/AstroProfundis))
  - Currently, the ID allocator is guaranteed not to allocate duplicated IDs, but when PD leader changes multiple times, the IDs may not be monotonic
  - For tiup < v1.2.1, the command `tiup cluster display` may delete store (without confirm) by mistake due to this issue (high risk)
  - For tiup >= v1.2.1 and <= v1.3.0, the command `tiup cluster display` may display `up` stores as `tombstone`, and encourages the user to delete them with the command `tiup cluster prune` (medium risk)
- Fix the issue that the `cluster check` always fail on thp check even though the thp is disabled ([#1005](https://github.com/pingcap/tiup/pull/1005), [@lucklove](https://github.com/lucklove))
- Fix the issue that the command `tiup mirror merge -h` outputs wrong usage ([#1008](https://github.com/pingcap/tiup/pull/1008), [@lucklove](https://github.com/lucklove))
  - The syntax of this command should be `tiup mirror merge <mirror-dir-1> [mirror-dir-N]` but it outputs `tiup mirror merge <base> <mirror-dir-1> [mirror-dir-N]`
- Fix the issue that prometheus doesn't collect drainer metrics ([#1012](https://github.com/pingcap/tiup/pull/1012), [@SE-Bin](https://github.com/SE-Bin))

### Improvements

- Reduce display duration when PD nodes encounter network problems and droping packages ([#986](https://github.com/pingcap/tiup/pull/986), [@9547](https://github.com/9547))
- cluster, dm: support version input without leading 'v' ([#1009](https://github.com/pingcap/tiup/pull/1009), [@AstroProfundis](https://github.com/AstroProfundis))
- Add a warning to explain that we will stop the cluster before clean logs ([#1029](https://github.com/pingcap/tiup/pull/1029), [@lucklove](https://github.com/lucklove))
  - When a user try to clean logs with the command `tiup cluster clean --logs`, he may expect that the cluster is still running during the clean operation
  - The actual situation is not what he expect, which may suprise the user (risk)

## [1.3.0] 2020.12.17

### New Features

- Modify TiFlash's query memory limit from 10GB to 0(unlimited) in playground cluster ([#907](https://github.com/pingcap/tiup/pull/907), [@LittleFall](https://github.com/LittleFall))
- Import configuration into topology meta when migrating a cluster from Ansible ([#766](https://github.com/pingcap/tiup/pull/766), [@yuzhibotao](https://github.com/yuzhibotao))
  - Before, we stored imported ansible config in ansible-imported-configs which is hidden for users, in this release, we merge the configs into meta.yaml so that the user can see the config with the command `tiup cluster edit`
- Enhance the `tiup mirror` command ([#860](https://github.com/pingcap/tiup/pull/860), [@lucklove](https://github.com/lucklove))
  - **Support merge two or more mirrors into one**
  - Support publish component to local mirror besides remote mirror
  - Support add component owner to local mirror
- Partially support deploy cluster with hostname besides ip address (**EXPERIMENTAL**) ([#948](https://github.com/pingcap/tiup/pull/948),[#949](https://github.com/pingcap/tiup/pull/949), [@fln](https://github.com/fln))
  - Not usable for production, as there would be issue if a hostname resolves to a new IP address after deployment
- Support setting custom timeout for waiting instances up in playground-cluster ([#968](https://github.com/pingcap/tiup/pull/968), [@unbyte](https://github.com/unbyte))
- Support check and disable THP in `tiup cluster check` ([#964](https://github.com/pingcap/tiup/pull/964), [@anywhy](https://github.com/anywhy))
- Support sign remote manifest and rotate root.json ([#967](https://github.com/pingcap/tiup/pull/967), [@lucklove](https://github.com/lucklove))

### Fixes

- Fixed the issue that the public key created by TiUP was not removed after the cluster was destroyed ([#910](https://github.com/pingcap/tiup/pull/910), [@9547](https://github.com/9547))
- Fix the issue that user defined grafana username and password not imported from tidb-ansible cluster correctly ([#937](https://github.com/pingcap/tiup/pull/937), [@AstroProfundis](https://github.com/AstroProfundis))
- Fix the issue that playground cluster not quiting components with correct order: TiDB -> TiKV -> PD ([#933](https://github.com/pingcap/tiup/pull/933), [@unbyte](https://github.com/unbyte))
- Fix the issue that TiKV reports wrong advertise address when `--status-addr` is set to a wildcard address like `0.0.0.0` ([#951](https://github.com/pingcap/tiup/pull/951), [@lucklove](https://github.com/lucklove))
- Fix the issue that Prometheus doesn't reload target after scale-in action ([#958](https://github.com/pingcap/tiup/pull/958), [@9547](https://github.com/9547))
- Fix the issue that the config file for TiFlash missing in playground cluster ([#969](https://github.com/pingcap/tiup/pull/969), [@unbyte](https://github.com/unbyte))
- Fix Tilfash startup failed without stderr output when numa is enabled but numactl cannot be found ([#984](https://github.com/pingcap/tiup/pull/984), [@lucklove](https://github.com/lucklove))
- Fix the issue that the deployment environment fail to copy config file when zsh is configured ([#982](https://github.com/pingcap/tiup/pull/982), [@9547](https://github.com/9547))

### Improvements

- Enable memory buddyinfo monitoring on node_exporter to collect exposes statistics of memory fragments ([#904](https://github.com/pingcap/tiup/pull/904), [@9547](https://github.com/9547))
- Move error logs dumped by tiup-dm and tiup-cluster to `${TIUP_HOME}/logs` ([#908](https://github.com/pingcap/tiup/pull/908), [@9547](https://github.com/9547))
- Allow run pure TiKV (without TiDB) cluster in playground cluster ([#926](https://github.com/pingcap/tiup/pull/926), [@sticnarf](https://github.com/sticnarf))
- Add confirm stage for upgrade action ([#963](https://github.com/pingcap/tiup/pull/963), [@Win-Man](https://github.com/Win-Man))
- Omit debug log from console output in tiup-cluster ([#977](https://github.com/pingcap/tiup/pull/977), [@AstroProfundis](https://github.com/AstroProfundis))
- Prompt list of paths to be deleted before processing in the clean action of tiup-cluster ([#981](https://github.com/pingcap/tiup/pull/981), [#993](https://github.com/pingcap/tiup/pull/993), [@AstroProfundis](https://github.com/AstroProfundis))
- Make error message of monitor port conflict more readable ([#966](https://github.com/pingcap/tiup/pull/966), [@JaySon-Huang](https://github.com/JaySon-Huang))

## [1.2.5] 2020.11.27

### Fixes

- Fix the issue that can't operate the cluster which have tispark workers without tispark master ([#924](https://github.com/pingcap/tiup/pull/924), [@AstroProfundis](https://github.com/AstroProfundis))
  - Root cause: once the tispark master been removed from the cluster, any later action will be reject by TiUP
  - Fix: make it possible for broken clusters to fix no tispark master error by scaling out a new tispark master node
- Fix the issue that it report `pump node id not found` while drainer node id not found ([#925](https://github.com/pingcap/tiup/pull/925), [@lucklove](https://github.com/lucklove))

### Improvements

- Support deploy TiFlash on multi-disks with "storage" configurations since v4.0.9 ([#931](https://github.com/pingcap/tiup/pull/931), [#938](https://github.com/pingcap/tiup/pull/938), [@JaySon-Huang](https://github.com/JaySon-Huang))
- Check duplicated pd_servers.name in the topology before truly deploy the cluster ([#922](https://github.com/pingcap/tiup/pull/922), [@anywhy](https://github.com/anywhy))

## [1.2.4] 2020.11.19

### Fixes

- Fix the issue that Pump & Drainer has different node id between tidb-ansible and TiUP ([#903](https://github.com/pingcap/tiup/pull/903), [@lucklove](https://github.com/lucklove))
  - For the cluster imported from tidb-ansible, if the pump or drainer is restarted, it will start with a new node id
  - Risk of this issue: binlog may not work correctly after restart pump or drainer
- Fix the issue that audit log may get lost in some special case ([#879](https://github.com/pingcap/tiup/pull/879), [#882](https://github.com/pingcap/tiup/pull/882), [@9547](https://github.com/9547))
  - If the user execute two commands one follows the other, and the second one quit in 1 second, the audit log of the first command will be overwirten by the second one
  - Risk caused by this issue: some audit logs may get lost in above case
- Fix the issue that new component deployed with `tiup cluster scale-out` doesn't auto start when rebooting ([#905](https://github.com/pingcap/tiup/pull/905), [@9547](https://github.com/9547))
  - Risk caused by this issue: the cluster may be unavailable after rebooting
- Fix the issue that data directory of TiFlash is not deleted if multiple data directories are specified ([#871](https://github.com/pingcap/tiup/pull/871), [@9547](https://github.com/9547))
- Fix the issue that `node_exporter` and `blackbox_exporter` not cleaned up after scale-in all instances on specified host ([#857](https://github.com/pingcap/tiup/pull/857), [@9547](https://github.com/9547))
- Fix the issue that the patch command will fail when try to patch dm cluster ([#884](https://github.com/pingcap/tiup/pull/884), [@lucklove](https://github.com/lucklove))
- Fix the issue that the bench component report `Error 1105: client has multi-statement capability disabled` ([#887](https://github.com/pingcap/tiup/pull/887), [@mahjonp](https://github.com/mahjonp))
- Fix the issue that the TiSpark node can't be upgraded ([#901](https://github.com/pingcap/tiup/pull/901), [@lucklove](https://github.com/lucklove))
- Fix the issue that playground cluster can't start TiFlash with newest nightly PD ([#902](https://github.com/pingcap/tiup/pull/902), [@lucklove](https://github.com/lucklove))

### Improvements

- Ignore no tispark master error when listing clusters since the master node may be remove by `scale-in --force` ([#920](https://github.com/pingcap/tiup/pull/920), [@AstroProfundis](https://github.com/AstroProfundis))

## [1.2.3] 2020.10.30

### Fixes

- Fix misleading warning message in the display command ([#869](https://github.com/pingcap/tiup/pull/869), [@lucklove](https://github.com/lucklove))

## [1.2.1] 2020.10.23

### Improvements

- Introduce a more safe way to cleanup tombstone nodes ([#858](https://github.com/pingcap/tiup/pull/858), [@lucklove](https://github.com/lucklove))
  - When an user `scale-in` a TiKV server, it's data is not deleted until the user executes a `display` command, it's risky because there is no choice for user to confirm
  - We have add a `prune` command for the cleanup stage, the display command will not cleanup tombstone instance any more
- Skip auto-start the cluster before the scale-out action because there may be some damaged instance that can't be started ([#848](https://github.com/pingcap/tiup/pull/848), [@lucklove](https://github.com/lucklove))
  - In this version, the user should make sure the cluster is working correctly by themselves before executing `scale-out`
- Introduce a more graceful way to check TiKV labels ([#843](https://github.com/pingcap/tiup/pull/843), [@lucklove](https://github.com/lucklove))
  - Before this change, we check TiKV labels from the config files of TiKV and PD servers, however, servers imported from tidb-ansible deployment don't store latest labels in local config, this causes inaccurate label information
  - After this we will fetch PD and TiKV labels with PD api in display command

### Fixes

- Fix the issue that there is datarace when concurrent save the same file ([#836](https://github.com/pingcap/tiup/pull/836), [@9547](https://github.com/9547))
  - We found that while the cluster deployed with TLS supported, the ca.crt file was saved multi times in parallel, this may lead to the ca.crt file to be left empty
  - The influence of this issue is that the tiup client may not communicate with the cluster
- Fix the issue that files copied by TiUP may have different mode with origin files ([#844](https://github.com/pingcap/tiup/pull/844), [@lucklove](https://github.com/lucklove))
- Fix the issue that the tiup script not updated after `scale-in` PD ([#824](https://github.com/pingcap/tiup/pull/824), [@9547](https://github.com/9547))

## [1.2.0] 2020.09.29

### New Features

- Support tiup env sub command ([#788](https://github.com/pingcap/tiup/pull/788), [@lucklove](https://github.com/lucklove))
- Support TiCDC for playground ([#777](https://github.com/pingcap/tiup/pull/777), [@leoppro](https://github.com/leoppro))
- Support limiting core dump size ([#817](https://github.com/pingcap/tiup/pull/817), [@lucklove](https://github.com/lucklove))
- Support using latest Spark and TiSpark release ([#779](https://github.com/pingcap/tiup/pull/779), [@lucklove](https://github.com/lucklove))
- Support new cdc arguments `gc-ttl` and `tz` ([#770](https://github.com/pingcap/tiup/pull/770), [@lichunzhu](https://github.com/lichunzhu))
- Support specifing custom ssh and scp path ([#734](https://github.com/pingcap/tiup/pull/734), [@9547](https://github.com/9547))

### Fixes

- Fix `tiup update --self` results to tiup's binary file deleted ([#816](https://github.com/pingcap/tiup/pull/816), [@lucklove](https://github.com/lucklove))
- Fix per-host custom port for drainer not handled correctly on importing ([#806](https://github.com/pingcap/tiup/pull/806), [@AstroProfundis](https://github.com/AstroProfundis))
- Fix the issue that help message is inconsistent ([#758](https://github.com/pingcap/tiup/pull/758), [@9547](https://github.com/9547))
- Fix the issue that dm not applying config files correctly ([#810](https://github.com/pingcap/tiup/pull/810), [@lucklove](https://github.com/lucklove))
- Fix the issue that playground display wrong TiDB number in error message ([#821](https://github.com/pingcap/tiup/pull/821), [@SwanSpouse](https://github.com/SwanSpouse))

### Improvements

- Automaticlly check if TiKV's label is set ([#800](https://github.com/pingcap/tiup/pull/800), [@lucklove](https://github.com/lucklove))
- Download component with stream mode to avoid memory explosion ([#755](https://github.com/pingcap/tiup/pull/755), [@9547](https://github.com/9547))
- Save and display absolute path for deploy directory, data dirctory and log directory to avoid confusion ([#822](https://github.com/pingcap/tiup/pull/822), [@lucklove](https://github.com/lucklove))
- Redirect DM stdout to log files ([#815](https://github.com/pingcap/tiup/pull/815), [@csuzhangxc](https://github.com/csuzhangxc))
- Skip download nightly package when it exists ([#793](https://github.com/pingcap/tiup/pull/793), [@lucklove](https://github.com/lucklove))

## [1.1.2] 2020.09.11

### Fixes

- Fix the issue that TiKV store leader count is not correct ([#762](https://github.com/pingcap/tiup/pull/762))
- Fix the issue that TiFlash's data is not clean up ([#768](https://github.com/pingcap/tiup/pull/768))
- Fix the issue that `tiup cluster deploy --help` display wrong help message ([#758](https://github.com/pingcap/tiup/pull/758))
- Fix the issue that tiup-playground can't display and scale ([#749](https://github.com/pingcap/tiup/pull/749))

## [1.1.1] 2020.09.01

### Fixes

- Remove the username `root` in sudo command [#731](https://github.com/pingcap/tiup/issues/731)
- Transfer the default alertmanager.yml if the local config file not specified [#735](https://github.com/pingcap/tiup/issues/735)
- Only remove corresponed config files in InitConfig for monitor service in case it's a shared directory [#736](https://github.com/pingcap/tiup/issues/736)

## [1.1.0] 2020.08.28

### New Features

- [experimental] Support specifying customized configuration files for monitor components ([#712](https://github.com/pingcap/tiup/pull/712), [@lucklove](https://github.com/lucklove))
- Support specifying user group or skipping creating a user in the deploy and scale-out stage ([#678](https://github.com/pingcap/tiup/pull/678), [@lucklove](https://github.com/lucklove))
  - to specify the group: https://github.com/pingcap/tiup/blob/master/examples/topology.example.yaml&#35;L7
  - to skip creating the user: `tiup cluster deploy/scale-out --skip-create-user xxx` 
- [experimental] Support rename cluster by the command `tiup cluster rename <old-name> <new-name>` ([#671](https://github.com/pingcap/tiup/pull/671), [@lucklove](https://github.com/lucklove))
  > Grafana stores some data related to cluster name to its grafana.db. The rename action will NOT delete them. So there may be some useless panel need to be deleted manually. 
- [experimental] Introduce `tiup cluster clean` command ([#644](https://github.com/pingcap/tiup/pull/644), [@lucklove](https://github.com/lucklove)):
  - Cleanup all data in specified cluster: `tiup cluster clean ${cluster-name} --data`
  - Cleanup all logs in specified cluster: `tiup cluster clean ${cluster-name} --log`
  - Cleanup all logs and data in specified cluster: `tiup cluster clean ${cluster-name} --all`
  - Cleanup all logs and data in specified cluster, excepting the Prometheus service: `tiup cluster clean ${cluster-name} --all --ignore-role Prometheus`
  - Cleanup all logs and data in specified cluster, expecting the node `172.16.13.11:9000`: `tiup cluster clean ${cluster-name} --all --ignore-node 172.16.13.11:9000`
  - Cleanup all logs and data in specified cluster, expecting the host `172.16.13.11`: `tiup cluster clean ${cluster-name} --all --ignore-node 172.16.13.12`
- Support skipping evicting store when there is only 1 TiKV ([#662](https://github.com/pingcap/tiup/pull/662), [@lucklove](https://github.com/lucklove))
- Support importing clusters with binlog enabled ([#652](https://github.com/pingcap/tiup/pull/652), [@AstroProfundis](https://github.com/AstroProfundis))
- Support yml source format with tiup-dm ([#655](https://github.com/pingcap/tiup/pull/655), [@july2993](https://github.com/july2993))
- Support detecting port conflict of monitoring agents between different clusters ([#623](https://github.com/pingcap/tiup/pull/623), [@AstroProfundis](https://github.com/AstroProfundis))

### Fixes

- Set correct `deploy_dir` of monitoring agents when importing ansible deployed clusters ([#704](https://github.com/pingcap/tiup/pull/704), [@AstroProfundis](https://github.com/AstroProfundis))
- Fix the issue that `tiup update --self` may make root.json invalid with offline mirror ([#659](https://github.com/pingcap/tiup/pull/659), [@lucklove](https://github.com/lucklove))

### Improvements

- Add `advertise-status-addr` for TiFlash to support host name ([#676](https://github.com/pingcap/tiup/pull/676), [@birdstorm](https://github.com/birdstorm))

## [1.0.9] 2020.08.03

### tiup

* Clone with yanked version [#602](https://github.com/pingcap/tiup/pull/602)
* Support yank a single version on client side [#602](https://github.com/pingcap/tiup/pull/605)
* Support bash and zsh completion [#606](https://github.com/pingcap/tiup/pull/606)
* Handle yanked version when update components [#635](https://github.com/pingcap/tiup/pull/635)


### tiup-cluster

* Validate topology changes after edit-config [#609](https://github.com/pingcap/tiup/pull/609)
* Allow continue editing when new topology has errors [#624](https://github.com/pingcap/tiup/pull/624)
* Fix wrongly setted data_dir of TiFlash when import from ansible [#612](https://github.com/pingcap/tiup/pull/612)
* Support native ssh client [#615](https://github.com/pingcap/tiup/pull/615)
* Support refresh configuration only when reload [#625](https://github.com/pingcap/tiup/pull/625)
* Apply config file on scaled pd server [#627](https://github.com/pingcap/tiup/pull/627)
* Refresh monitor configs on reload [#630](https://github.com/pingcap/tiup/pull/630)
* Support posix style argument for user flag [#631](https://github.com/pingcap/tiup/pull/631)
* Fix PD config incompatible when retrieving dashboard address [#638](https://github.com/pingcap/tiup/pull/638)
* Integrate tispark [#531](https://github.com/pingcap/tiup/pull/531) [#621](https://github.com/pingcap/tiup/pull/621)
