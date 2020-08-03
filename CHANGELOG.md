TiUP Changelog

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
