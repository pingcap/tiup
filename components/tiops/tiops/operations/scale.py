# coding: utf-8

import os

from tiops import exceptions
from tiops import modules
from tiops import utils
from tiops.operations import OperationBase
from tiops.tui import term
from tiops.ansibleapi import ansibleapi
from tiops.operations.action import Action


class OprScaleOut(OperationBase):
    def __init__(self, args=None, topology=None, new_srvs=None):
        if os.path.exists(topology.topology_file):
            term.warn('Check TiDB cluster {} status, it may take a few minutes.'.format(
                topology.cluster_name))
            self.check_tombstone(topology, args)
        self._new_topo, self._diff = topology.add(new_srvs)
        topology.replace(self._new_topo, write=False)
        super(OprScaleOut, self).__init__(args, topology, action='deploy')
        self.act = Action(ans=self.ans, topo=self.topology)

    def _prepare(self, component=None, pattern=None, node=None, role=None):
        if not self._diff:
            msg = 'No new nodes to scale out.'
            term.error(msg)
            raise exceptions.TiOPSConfigError(msg)
        term.notice('Begin add node for TiDB cluster.')

        # copy template
        utils.create_dir(self.topology.cache_template_dir)
        utils.copy_template(
            source=os.path.join(self.topology.titemplate_dir),
            target=os.path.join(self.topology.cache_template_dir))

        # update scripts when scale-out.
        for service in ['pd', 'tikv', 'tidb', 'pump', 'drainer']:
            if '{}_servers'.format(service) in self._diff:
                template_path = os.path.join(
                    self.topology.cache_template_dir, 'scripts/run_{}.sh.j2'.format(service))
                _original, new_template = utils.script_template(path=self.topology.cluster_dir,
                                                                template=template_path, service=service)
                utils.write_template(template_path, new_template)

    def _process(self, component=None, pattern=None, node=None, role=None):
        term.info('Check ssh connection.')
        self.act.check_ssh_connection()
        self.act.edit_file()
        try:
            term.info('Create directory in all add nodes.')
            for service in self.topology.service_group:
                component, pattern = self.check_exist(service, self._diff)
                if not component and not pattern:
                    continue
                uuid = [x['uuid'] for x in self._diff[pattern]]
                self.act.create_directory(
                    component=component, pattern=pattern, node=','.join(uuid))

            # check machine cpu / memory / disk
            self.act.check_machine_config(self._diff)
            # start run scale-out
            for service in self.topology.service_group:
                component, pattern = self.check_exist(service, self._diff)
                if not component and not pattern:
                    continue
                uuid = [x['uuid'] for x in self._diff[pattern]]
                term.normal('Add {}, node list: {}.'.format(
                    component, ','.join(uuid)))
                _template_dir = self.topology.cache_template_dir
                self.act.deploy_component(component=component, pattern=pattern, node=','.join(
                    uuid), template_dir=_template_dir)
                self.act.deploy_firewall(
                    component=component, pattern=pattern, node=','.join(uuid))
                self.act.start_component(
                    component=component, pattern=pattern, node=','.join(uuid))
        finally:
            os.popen('rm -rf {}'.format(self.topology.cache_template_dir))

    def _post(self, component=None, pattern=None, node=None, role=None):
        # if 'pd_servers' in self._diff:
        #    reload_pd = True
        # else:
        #    reload_pd = False
        self.topology.replace(self._new_topo)
        term.info('Update configuration.')
        ans = ansibleapi.ANSRunner(
            user=self.topology.user, topology=self.topology._topology(self._new_topo), tiargs=self._args)
        act = Action(ans=ans, topo=self.topology)
        if 'pd_servers' in self._diff:
            act.deploy_component(component='pd', pattern='pd_servers')
            act.deploy_component(component='tikv', pattern='tikv_servers')
            act.deploy_component(component='tidb', pattern='tidb_servers')
            act.deploy_component(component='pump', pattern='pump_servers')
            act.deploy_component(component='drainer',
                                 pattern='drainer_servers')

        act.deploy_component(component='prometheus',
                             pattern='monitoring_server')
        act.stop_component(component='prometheus', pattern='monitoring_server')
        act.start_component(component='prometheus',
                            pattern='monitoring_server')
        term.notice('Finished scaling out.')


class OprScaleIn(OperationBase):
    def __init__(self, args=None, topology=None, node=None):
        if not node:
            msg = 'Node ID not specified.'
            term.error(msg)
            raise exceptions.TiOPSConfigError(msg)

        self._new_topo, self._diff = topology.remove(node)

        super(OprScaleIn, self).__init__(args, topology)
        self.act = Action(ans=self.ans, topo=self.topology)

    def _prepare(self, component=None, pattern=None, node=None, role=None):
        term.notice('Begin delete node for TiDB cluster.')
        self._cluster = modules.ClusterAPI(topology=self.topology)
        self._pd_status = self._cluster.status()
        self._tikv_stores = self._cluster.tikv_stores()

    def _process(self, component=None, pattern=None, node=None, role=None):
        _unhealth_node = []
        for _pd_node in self._cluster.status():
            if not _pd_node['health']:
                _unhealth_node.append(_pd_node['name'])
                msg = 'Some pd node is unhealthy, maybe server stoppd or network unreachable, unhealthy node list: {}'.format(
                    ','.join(_unhealth_node))
                term.fatal(msg)
                raise exceptions.TiOPSRuntimeError(msg, operation='scaleIn')

        _current_pd_num = len(self._pd_status)
        _current_tikv_num = len(self._tikv_stores)

        if 'pd_servers' in self._diff and len(self._diff['pd_servers']) == _current_pd_num:
            term.fatal('Can not delete all pd node.')
            exit(1)

        if 'tikv_servers' in self._diff and len(self._diff['tikv_servers']) == _current_tikv_num:
            term.fatal('Can not delete all tikv node.')
            exit(1)

        term.info('Check ssh connection.')
        self.act.check_ssh_connection()

        for service in self.topology.service_group[::-1]:
            component, pattern = self.check_exist(service, self._diff)
            if not component and not pattern:
                continue
            uuid = [x['uuid'] for x in self._diff[pattern]]
            term.normal('Delete {}, node list: {}'.format(
                component, ','.join(uuid)))
            for _uuid in uuid:
                self.__delete_component(self._diff, component, pattern, _uuid)
                if component not in ['tikv', 'pump', 'drainer']:
                    self.act.stop_component(
                        component=component, pattern=pattern, node=_uuid)
                    self.act.destroy_component(
                        component=component, pattern=pattern, node=_uuid)
                if component != 'blackbox_exporter':
                    self.topology.replace(self.topology.remove(_uuid)[0])

    def _post(self, component=None, pattern=None, node=None, role=None):
        ans = ansibleapi.ANSRunner(
            user=self.topology.user, topology=self.topology(), tiargs=self._args)
        act = Action(ans=ans, topo=self.topology)
        if 'pd_servers' in self._diff:
            act.deploy_component(component='pd', pattern='pd_servers')
            act.deploy_component(component='tikv', pattern='tikv_servers')
            act.deploy_component(component='tidb', pattern='tidb_servers')
            act.deploy_component(component='pump', pattern='pump_servers')
            act.deploy_component(component='drainer',
                                 pattern='drainer_servers')

        # self.deploy.deploy_component(component='prometheus', pattern='monitoring_server', ans=ans)
        # self.reload.do(component='prometheus', pattern='monitoring_server')

        term.notice('Finished scaling in.')

    def __delete_component(self, config=None, component=None, pattern=None, uuid=None):
        if component == 'pd':
            try:
                self._cluster.del_pd(uuid)
            except exceptions.TiOPSException as e:
                term.fatal(
                    'Unable to delete PD node from cluster: {}'.format(e)
                )
                exit(1)

        if component == 'tikv':
            _tikv_info = ''
            for _tikv_node in config[pattern]:
                if _tikv_node['uuid'] != uuid:
                    continue
                if _tikv_node['offline']:
                    return
                _tikv_info = _tikv_node
            for ctikv in self._tikv_stores['stores']:
                # check if node in cluster
                if '{}:{}'.format(_tikv_info['ip'], _tikv_info['port']) == ctikv['store']['address']:
                    _store_id = ctikv['store']['id']

                    # delete store through api
                    try:
                        self._cluster.del_store(_store_id)
                    except exceptions.TiOPSException as e:
                        term.fatal(
                            'Unable to delete store: {}'.format(e)
                        )
                        exit(1)

        if component == 'drainer':
            _binlog = modules.BinlogAPI(topology=self.topology)
            _binlog.delete_drainer(node_id=uuid)

        if component == 'pump':
            _binlog = modules.BinlogAPI(topology=self.topology)
            _binlog.delete_pump(node_id=uuid)
