# coding: utf-8

from tiops.operations import OperationBase
from tiops.tui import term
from tiops.operations.action import Action


class OprStop(OperationBase):
    def __init__(self, args=None, topology=None):
        super(OprStop, self).__init__(args, topology)
        self.act = Action(ans=self.ans, topo=self.topology)

    def _process(self, component=None, pattern=None, node=None, role=None):
        if node:
            term.notice('Stop specified node in cluster.')
        elif role:
            term.notice('Stop specified role in cluster.')
        else:
            term.notice('Stop TiDB cluster.')
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

    def _post(self, component=None, pattern=None, node=None, role=None):
        term.notice('Finished stop.')
