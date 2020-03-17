# coding: utf-8

import os
import semver

from tiops import exceptions
from tiops import modules
from tiops import utils
from tiops.operations import OprDeploy
from tiops.tui import term
from tiops.operations.action import Action


class OprUpgrade(OprDeploy):
    def __init__(self, args=None, topology=None):
        super(OprUpgrade, self).__init__(args, topology)
        self.act = Action(ans=self.ans, topo=self.topology)
        try:
            self.arg_ver = args.tidb_version
        except AttributeError:
            raise exceptions.TiOPSConfigError(
                '--tidb-version is not set when upgrade, abort.')
        try:
            self.force = args.force
        except AttributeError:
            self.force = False

    # check versions, it update version related variables in memory, but not writting them to disk
    def __check_version(self):
        new_ver = self.arg_ver.lstrip('v')
        curr_ver = self.topology.version.lstrip('v')
        _cmp = semver.compare(curr_ver, new_ver)
        if _cmp == 0:
            raise exceptions.TiOPSArgumentError(
                'Already running version {}.'.format(curr_ver))
        elif _cmp > 0:
            raise exceptions.TiOPSRuntimeError(
                'Downgrade is not supported, keep running {}.'.format(curr_ver), operation='upgrade')

        # update version and related variables
        self.old_ver = curr_ver
        self.new_ver = new_ver
        self.topology.version = 'v{}'.format(new_ver)
        self.topology.tiversion_dir = os.path.join(
            self.topology.tidown_dir, '{}'.format(self.topology.version))
        self.topology.resource_dir = utils.profile_path(
            'downloads', '{}/resources'.format(self.topology.version))
        self.topology.dashboard_dir = utils.profile_path(
            'downloads', '{}/dashboards'.format(self.topology.version))
        self.topology.package_dir = utils.profile_path(
            'downloads', '{}/packages'.format(self.topology.version))
        self.topology.config_dir = utils.profile_path(
            'downloads', '{}/configs'.format(self.topology.version))

    # Check if the configuration of the tidb component is reasonable
    def _check_config(self, topology=None):
        if not topology:
            topology = self.topology()
        _servers = [
            {'pd': 'pd_servers'},
            {'tikv': 'tikv_servers'},
            {'tidb': 'tidb_servers'},
        ]

        for _service in _servers:
            _component, _pattern = self.check_exist(
                _service, config=topology)
            if not _component and not _pattern:
                continue
            term.info('Check {} configuration.'.format(_component))
            self.act.configCheck(component=_component, pattern=_pattern, node=topology[_pattern][0]['uuid'])

    # TODO: check and merge configs
    def __check_config(self):
        pass

    def _prepare(self, component=None, pattern=None, node=None, role=None):
        # check versions before processing
        self.__check_version()

        term.notice('Upgrading from v{} to v{}.'.format(
            self.old_ver, self.new_ver))

        # download packages for new version
        term.info('Downloading TiDB related binary, it may take a few minutes.')
        try:
            _local = self._args.local_pkg
        except AttributeError:
            _local = None
        self.act.download(version=self.new_ver, local_pkg=_local)

        # check configs
        self.__check_config()

    def _process(self, component=None, pattern=None, node=None, role=None):
        if node:
            term.notice('Upgrade specified node in cluster.')
        elif role:
            term.notice('Upgrade specified role in cluster.')
        else:
            term.notice('Upgrade TiDB cluster.')
        _topology = self.topology.role_node(roles=role, nodes=node)
        if self._args.enable_check_config:
            self._check_config()
        # for service in ['pd', 'tikv', 'pump', 'tidb']:
        # grp = [x for x in self.topology.service_group if service in x.keys()]
        _cluster = modules.ClusterAPI(topology=self.topology)
        _unhealth_node = []
        for _pd_node in _cluster.status():
            if not _pd_node['health']:
                _unhealth_node.append(_pd_node['name'])
                msg = 'Some pd node is unhealthy, maybe server stoppd or network unreachable, unhealthy node list: {}'.format(
                    ','.join(_unhealth_node))
                term.fatal(msg)
                raise exceptions.TiOPSRuntimeError(msg, operation='upgrade')

        term.info('Check ssh connection.')
        self.act.check_ssh_connection()

        if self.force:
            for service in self.topology.service_group:
                component, pattern = self.check_exist(
                    service=service, config=_topology)
                if not component and not pattern:
                    continue
                if pattern in ['monitored_servers', 'monitoring_server', 'grafana_server', 'alertmanager_server']:
                    term.normal('Upgrade {}.'.format(component))
                    self.act.deploy_component(
                        component=component, pattern=pattern)
                    self.act.stop_component(
                        component=component, pattern=pattern)
                    self.act.start_component(
                        component=component, pattern=pattern)
                    continue

                for _node in _topology[pattern]:
                    _uuid = _node['uuid']
                    term.normal('Upgrade {}, node id: {}.'.format(
                        component, _uuid))
                    self.act.deploy_component(
                        component=component, pattern=pattern, node=_uuid)
                    self.act.stop_component(
                        component=component, pattern=pattern, node=_uuid)
                    self.act.start_component(
                        component=component, pattern=pattern, node=_uuid)
            return

        # every time should only contain one item
        for service in self.topology.service_group:
            component, pattern = self.check_exist(
                service=service, config=_topology)
            if not component and not pattern:
                continue
            # upgrade pd server, upgrade leader node finally
            if component == 'pd':
                _pd_list = []
                for _node in _topology[pattern]:
                    if _node['uuid'] == _cluster.pd_leader():
                        _leader = _node
                    else:
                        _pd_list.append(_node)
                _pd_list.append(_leader)

                for _node in _pd_list:
                    _uuid = _node['uuid']
                    _host = _node['ip']
                    term.normal('Upgrade {}, node id: {}.'.format(
                        component, _uuid))
                    if _uuid == _cluster.pd_leader():
                        _cluster.evict_pd_leader(uuid=_uuid)

                    self.act.deploy_component(
                        component=component, pattern=pattern, node=_uuid)
                    self.act.stop_component(
                        component=component, pattern=pattern, node=_uuid)
                    self.act.start_component(
                        component=component, pattern=pattern, node=_uuid)
                continue

            if pattern in ['monitored_servers', 'monitoring_server', 'grafana_server', 'alertmanager_server']:
                term.normal('Upgrade {}.'.format(component))
                self.act.deploy_component(component=component, pattern=pattern)
                self.act.stop_component(component=component, pattern=pattern)
                self.act.start_component(component=component, pattern=pattern)
                continue

            for _node in _topology[pattern]:
                _uuid = _node['uuid']
                _host = _node['ip']
                term.normal('Upgrade {}, node id: {}.'.format(component, _uuid))
                if pattern == 'tikv_servers':
                    _port = _node['port']
                    _cluster.evict_store_leaders(host=_host, port=_port)

                self.act.deploy_component(
                    component=component, pattern=pattern, node=_uuid)
                self.act.stop_component(
                    component=component, pattern=pattern, node=_uuid)
                self.act.start_component(
                    component=component, pattern=pattern, node=_uuid)

                if pattern == 'tikv_servers':
                    _cluster.remove_evict(host=_host, port=_port)

    def _post(self, component=None, pattern=None, node=None, role=None):
        self.topology.set_meta(version=self.new_ver)
        term.notice('Upgraded to {}.'.format(self.topology.version))
