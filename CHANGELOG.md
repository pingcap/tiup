TiUP Changelog

## [1.1.2] 2020.09.11

### Fixes

- Fix the issue that tikv store leader count is not correct ([#762](https://github.com/pingcap/tiup/pull/762))
- Fix the issue that tiflash's data is not clean up ([#768](https://github.com/pingcap/tiup/pull/768))
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
  - Cleanup all logs and data in specified cluster, excepting the prometheus service: `tiup cluster clean ${cluster-name} --all --ignore-role prometheus`
  - Cleanup all logs and data in specified cluster, expecting the node `172.16.13.11:9000`: `tiup cluster clean ${cluster-name} --all --ignore-node 172.16.13.11:9000`
  - Cleanup all logs and data in specified cluster, expecting the host `172.16.13.11`: `tiup cluster clean ${cluster-name} --all --ignore-node 172.16.13.12`
- Support skipping evicting store when there is only 1 tikv ([#662](https://github.com/pingcap/tiup/pull/662), [@lucklove](https://github.com/lucklove))
- Support importing clusters with binlog enabled ([#652](https://github.com/pingcap/tiup/pull/652), [@AstroProfundis](https://github.com/AstroProfundis))
- Support yml source format with tiup-dm ([#655](https://github.com/pingcap/tiup/pull/655), [@july2993](https://github.com/july2993))
- Support detecting port conflict of monitoring agents between different clusters ([#623](https://github.com/pingcap/tiup/pull/623), [@AstroProfundis](https://github.com/AstroProfundis))

### Fixes

- Set correct `deploy_dir` of monitoring agents when importing ansible deployed clusters ([#704](https://github.com/pingcap/tiup/pull/704), [@AstroProfundis](https://github.com/AstroProfundis))
- Fix the issue that `tiup update --self` may make root.json invalid with offline mirror ([#659](https://github.com/pingcap/tiup/pull/659), [@lucklove](https://github.com/lucklove))

### Improvements

- Add `advertise-status-addr` for tiflash to support host name ([#676](https://github.com/pingcap/tiup/pull/676), [@birdstorm](https://github.com/birdstorm))

## [1.0.9] 2020.08.03

### tiup

* Clone with yanked version [#602](https://github.com/pingcap/tiup/pull/602)
* Support yank a single version on client side [#602](https://github.com/pingcap/tiup/pull/605)
* Support bash and zsh completion [#606](https://github.com/pingcap/tiup/pull/606)
* Handle yanked version when update components [#635](https://github.com/pingcap/tiup/pull/635)


### tiup-cluster

* Validate topology changes after edit-config [#609](https://github.com/pingcap/tiup/pull/609)
* Allow continue editing when new topology has errors [#624](https://github.com/pingcap/tiup/pull/624)
* Fix wrongly setted data_dir of tiflash when import from ansible [#612](https://github.com/pingcap/tiup/pull/612)
* Support native ssh client [#615](https://github.com/pingcap/tiup/pull/615)
* Support refresh configuration only when reload [#625](https://github.com/pingcap/tiup/pull/625)
* Apply config file on scaled pd server [#627](https://github.com/pingcap/tiup/pull/627)
* Refresh monitor configs on reload [#630](https://github.com/pingcap/tiup/pull/630)
* Support posix style argument for user flag [#631](https://github.com/pingcap/tiup/pull/631)
* Fix PD config incompatible when retrieving dashboard address [#638](https://github.com/pingcap/tiup/pull/638)
* Integrate tispark [#531](https://github.com/pingcap/tiup/pull/531) [#621](https://github.com/pingcap/tiup/pull/621)
