# coding: utf-8

from tiops.operations import OperationBase
from tiops.tui import term
from tiops.operations.action import Action


class OprRestart(OperationBase):
    def __init__(self, args=None, topology=None):
        super(OprRestart, self).__init__(args, topology)
        self.act = Action(ans=self.ans, topo=self.topology)

    def _process(self, component=None, pattern=None, node=None, role=None):
        if node:
            term.notice('Restart specified node in cluster.')
        elif role:
            term.notice('Restart specified role in cluster.')
        else:
            term.notice('Restart TiDB cluster.')
        _topology = self.topology.role_node(roles=role, nodes=node)

        term.info('Check ssh connection.')
        self.act.check_ssh_connection()

        for service in self.topology.service_group[::-1]:
            component, pattern = self.check_exist(
                service, config=_topology)
            if not component and not pattern:
                continue
            if not node:
                term.normal('Stopping {}.'.format(component))
                self.act.stop_component(component, pattern)
            else:
                _uuid = [x['uuid'] for x in _topology[pattern]]
                term.normal('Stopping {}, node list: {}.'.format(
                    component, ','.join(_uuid)))
                self.act.stop_component(component, pattern, ','.join(_uuid))

        for service in self.topology.service_group:
            component, pattern = self.check_exist(
                service, config=_topology)
            if not component and not pattern:
                continue
            if not node:
                term.normal('Starting {}.'.format(component))
                self.act.start_component(component, pattern)
            else:
                _uuid = [x['uuid'] for x in _topology[pattern]]
                term.normal('Starting {}, node list: {}.'.format(
                    component, ','.join(_uuid)))
                self.act.start_component(component, pattern, ','.join(_uuid))

    def _post(self, component=None, pattern=None, node=None, role=None):
        term.notice('Finished restart.')
