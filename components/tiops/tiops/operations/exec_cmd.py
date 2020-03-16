# coding: utf-8


from tiops import exceptions
from tiops import modules
from tiops.ansibleapi import ansibleapi
from tiops.operations import OperationBase
from tiops.operations.action import Action
from tiops.tui import term


class OprExec(OperationBase):
    def __init__(self, args=None, topology=None):
        super(OprExec, self).__init__(args, topology)
        self.act = Action(ans=self.ans, topo=self.topology)
        self._result = {
            'failed': {},
            'success': {},
            'unreachable': {},
        }

    def _prepare(self, component=None, pattern=None, node=None, role=None):
        try:
            self.cmd = ' '.join(self._args.cmd)
        except AttributeError:
            raise exceptions.TiOPSArgumentError(
                'No command specified, do nothing.')
        term.notice('Run raw shell command on {} cluster.'.format(
            self.topology.cluster_name))
        term.normal('{}'.format(self.cmd))

    def _process(self, component=None, pattern=None, node=None, role=None):
        if node:
            term.notice('Running command on specified node in cluster.')
        elif role:
            term.notice('Running command on specified role in cluster.')
        else:
            term.notice('Running command on all node in cluster.')

        _topology = self.topology.role_node(roles=role, nodes=node)

        try:
            _sudo = self._args.root
        except AttributeError:
            _sudo = False

        term.info('Check ssh connection.')
        self.act.check_ssh_connection()

        for service in self.topology.service_group:
            component, pattern = self.check_exist(
                service, config=_topology)
            if not component and not pattern:
                continue
            if not node:
                term.info('Running command on {}.'.format(component))
                self.__run(pattern=pattern, sudo=_sudo, cmd=self.cmd)
            else:
                _uuid = [x['uuid'] for x in _topology[pattern]]
                term.info('Running command on {}, node list: {}.'.format(
                    component, ','.join(_uuid)))
                self.__run(pattern=pattern, node=','.join(
                    _uuid), sudo=_sudo, cmd=self.cmd)

    def _post(self, component=None, pattern=None, node=None, role=None):
        term.notice('Finished reload config for {} cluster.'.format(
            self.topology.version))

        print(term.bold_cyan('Success:'))
        for host, out in self._result['success'].items():
            _output = 'stdout: {}'.format(out['stdout'])
            if len(out['stderr']) > 0:
                _output += '\nstderr: {}'.format(out['stderr'])
            print(term.plain_green('{}:'.format(host)))
            print(term.plain(_output))

        if len(self._result['unreachable']) > 0:
            print(term.bold_yellow('Unreachable:'))
            for host, out in self._result['unreachable'].items():
                _output = 'stdout: {}'.format(out['stdout'])
                if len(out['stderr']) > 0:
                    _output += '\nstderr: {}'.format(out['stderr'])
                print(term.plain_yellow('{}:'.format(host)))
                print(term.plain(_output))

        if len(self._result['failed']) > 0:
            print(term.bold_red('Failed:'))
            for host, out in self._result['failed'].items():
                _output = 'stdout: {}'.format(out['stdout'])
                if len(out['stderr']) > 0:
                    _output += '\nstderr: {}'.format(out['stderr'])
                print(term.plain_red('{}:'.format(host)))
                print(term.plain(_output))

    def __run(self, pattern=None, node=None, sudo=False, cmd=None):
        try:
            _result = self.act.run_shell(
                pattern=pattern, node=node, sudo=sudo, cmd=cmd)
        except exceptions.TiOPSRuntimeError as e:
            term.error('Error execute command: {}'.format(e))
            _result = e.ctx

        for host, out in _result['success'].items():
            if not host in self._result['success'].keys():
                self._result['success'][host] = out
        for host, out in _result['failed'].items():
            if not host in self._result['failed'].keys():
                self._result['failed'][host] = out
        for host, out in _result['unreachable'].items():
            if not host in self._result['unreachable'].keys():
                self._result['unreachable'][host] = out
