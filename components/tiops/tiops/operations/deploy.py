# coding: utf-8


import semver
from os.path import join
from tiops import exceptions
from tiops.operations.action import Action
from tiops.operations import OperationBase
from tiops.tui import term


class OprDeploy(OperationBase):
    def __init__(self, args=None, topology=None, demo=False):
        super(OprDeploy, self).__init__(args, topology, demo=demo)
        self.act = Action(ans=self.ans, topo=self.topology)
        self.demo = demo

    def _check_config(self):
        _servers = [
            {'pd': 'pd_servers'},
            {'tikv': 'tikv_servers'},
            {'tidb': 'tidb_servers'},
        ]

        for _service in _servers:
            _component, _pattern = self.check_exist(
                _service, config=self.topology())
            if not _component and not _pattern:
                continue
            term.normal('Check {} configuration.'.format(_component))
            self.act.configCheck(component=_component, pattern=_pattern, node=self.topology()[
                                 _pattern][0]['uuid'])

    def _prepare(self, component=None, pattern=None, node=None, role=None):
        if self.topology.version and self._args.tidb_version:
            new_ver = self._args.tidb_version.lstrip('v')
            curr_ver = self.topology.version.lstrip('v')
            _cmp = semver.compare(curr_ver, new_ver)
            if _cmp > 0:
                raise exceptions.TiOPSArgumentError(
                    'Running version is {}, can\'t downgrade.'.format(curr_ver))

        term.notice('Begin installing TiDB cluster.')
        # download packages
        term.info(
            'Downloading TiDB related binary, it may take a few minutes.')
        try:
            _local = self._args.local_pkg
        except AttributeError:
            _local = None
        self.act.download(local_pkg=_local)

        if not self.demo:
            # edit config
            self.act.edit_file()

            term.info('Check ssh connection.')
            self.act.check_ssh_connection()

            if self._args.enable_check_config:
                self._check_config()


    def _process(self, component=None, pattern=None, node=None, role=None):
        # creart directory
        term.info('Create directory in all nodes.')
        for service in self.topology.service_group:
            component, pattern = self.check_exist(
                service, config=self.topology())
            if not component and not pattern:
                continue
            self.act.create_directory(component=component, pattern=pattern)

        if not self.demo:
            self.act.check_machine_config()

        # start run deploy
        if self.demo:
            term.warn('FirewallD is being disabled on deployment machines in quick deploy mode.')
        for service in self.topology.service_group:
            component, pattern = self.check_exist(
                service, config=self.topology())
            if not component and not pattern:
                continue
            term.normal('Deploy {}.'.format(component))
            self.act.deploy_component(component=component, pattern=pattern)
            self.act.deploy_firewall(component=component, pattern=pattern)

        if not self.demo:
            self.act.deploy_tool()

    def _post(self, component=None, pattern=None, node=None, role=None):
        self.topology.set_meta()
        self.topology._save_topology()
        if self.demo:
            term.notice('Finished deploying TiDB cluster {} ({}).'.format(
                self.topology.cluster_name, self.topology.version))
        else:
            term.notice('Finished deploying TiDB cluster {} ({}), don\'t forget to start it.'.format(
                self.topology.cluster_name, self.topology.version))
