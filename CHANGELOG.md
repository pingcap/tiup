TiUP Changelog

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
- 

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
