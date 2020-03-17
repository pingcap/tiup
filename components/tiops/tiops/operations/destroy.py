# coding: utf-8

import logging

from tiops import exceptions
from tiops import utils
from tiops.operations import OperationBase
from tiops.tui import term
from tiops.operations.action import Action


class OprDestroy(OperationBase):
    def __init__(self, args=None, topology=None):
        super(OprDestroy, self).__init__(args, topology)
        self.act = Action(ans=self.ans, topo=self.topology)

    def _prepare(self, component=None, pattern=None, node=None, role=None):
        term.warn('The TiDB cluster {} ({}) is going to be destroyed.'.format(
            self.topology.cluster_name, self.topology.version))
        rm_promt = 'This operation will ' + term.warn_red('remove') \
                   + ' the TiDB cluster ' + term.highlight_red(self.topology.cluster_name) \
                   + '. It can NOT be undone. ' + term.yes_no() + ':'
        notice = term.input(rm_promt)
        if notice.lower() not in ['y', 'yes']:
            term.notice('Terminate the destroy operation.')
            raise exceptions.TiOPSRuntimeError('Operation cancelled by user.')

    def _process(self, component=None, pattern=None, node=None, role=None):
        term.info('Check ssh connection.')
        self.act.check_ssh_connection()
        term.info('Stopping TiDB cluster.')
        for service in self.topology.service_group[::-1]:
            component, pattern = self.check_exist(
                service, config=self.topology())
            if not component and not pattern:
                continue
            try:
                self.act.stop_component(
                    component=component, pattern=pattern, node=node)
            except exceptions.TiOPSWarning as e:
                term.debug(str(e))
                pass

        for service in self.topology.service_group[::-1]:
            component, pattern = self.check_exist(
                service, config=self.topology())
            if not component and not pattern:
                continue
            term.normal('{} is being destroyed.'.format(component))
            try:
                self.act.destroy_component(
                    component=component, pattern=pattern, node=node)
            except exceptions.TiOPSWarning as e:
                term.debug(str(e))
                pass

        # remove deploy dir
        self.ans.run_model('shell',
                           'rm -rf {{ full_deploy_dir | cluster_dir }}',
                           become=True,
                           group='*')

        self.ans.run_model('shell',
                           'rm -rf {{ full_data_dir | cluster_dir }}',
                           become=True,
                           group='*')

    def _post(self, component=None, pattern=None, node=None, role=None):
        try:
            utils.remove_dir(utils.profile_path(self.topology.cluster_dir))
        except Exception as e:
            logging.warning(e)

        term.notice('TiDB cluster destroyed.')
