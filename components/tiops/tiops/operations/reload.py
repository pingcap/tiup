# coding: utf-8


from tiops import exceptions
from tiops import modules
from tiops.operations import OperationBase
from tiops.tui import term
from tiops.operations.action import Action


class OprReload(OperationBase):
    def __init__(self, args=None, topology=None):
        super(OprReload, self).__init__(args, topology)
        self.act = Action(ans=self.ans, topo=self.topology)

    def _process(self, component=None, pattern=None, node=None, role=None):
        if node:
            term.notice('Reload specified node in cluster.')
        elif role:
            term.notice('Reload specified role in cluster.')
        else:
            term.notice('Reload TiDB cluster.')
        _topology = self.topology.role_node(roles=role, nodes=node)

        _cluster = modules.ClusterAPI(topology=self.topology)
        _unhealth_node = []
        for _pd_node in _cluster.status():
            if not _pd_node['health']:
                _unhealth_node.append(_pd_node['name'])
                msg = 'Some pd node is unhealthy, maybe server stoppd or network unreachable, unhealthy node list: {}'.format(
                    ','.join(_unhealth_node))
                term.fatal(msg)
                raise exceptions.TiOPSRuntimeError(msg, operation='reload')

        term.info('Check ssh connection.')
        self.act.check_ssh_connection()
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
                    term.normal('Reload {}, node id: {}.'.format(
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
                if not node:
                    term.normal('Reload {}.'.format(component))
                    self.act.deploy_component(
                        component=component, pattern=pattern)
                    self.act.stop_component(
                        component=component, pattern=pattern)
                    self.act.start_component(
                        component=component, pattern=pattern)
                else:
                    _uuid = [x['uuid'] for x in _topology[pattern]]
                    term.normal('Reload {}, node list: {}.'.format(
                        component, ','.join(_uuid)))
                    self.act.deploy_component(
                        component=component, pattern=pattern, node=','.join(_uuid))
                    self.act.stop_component(
                        component=component, pattern=pattern, node=','.join(_uuid))
                    self.act.start_component(
                        component=component, pattern=pattern, node=','.join(_uuid))
                continue

            for _node in _topology[pattern]:
                _uuid = _node['uuid']
                _host = _node['ip']
                term.normal('Reload {}, node id: {}.'.format(component, _uuid))
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
        term.notice('Finished reload config for {} cluster.'.format(
            self.topology.version))
