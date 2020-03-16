# coding: utf-8


import re
import json
import logging

from collections import namedtuple
from os.path import join, abspath

from ansible import constants as C
from ansible.playbook.play import Play
from ansible.errors import AnsibleError
from ansible.utils.path import unfrackpath
from ansible.parsing.dataloader import DataLoader
from ansible.plugins.callback import CallbackBase
from ansible.vars.manager import VariableManager
from ansible.inventory.manager import InventoryManager
from ansible.executor.task_queue_manager import TaskQueueManager
from ansible.executor.playbook_executor import PlaybookExecutor

from tiops import exceptions
from tiops import utils
from tiops.tui import term

try:
    from __main__ import display
except ImportError:
    from ansible.utils.display import Display

    display = Display()


class ModelResultsCollector(CallbackBase):
    def __init__(self, *args, **kwargs):
        super(ModelResultsCollector, self).__init__(*args, **kwargs)
        self.host_ok = {}
        self.host_unreachable = {}
        self.host_failed = {}

    def v2_runner_on_unreachable(self, result):
        self.host_unreachable[result._host.get_name()] = result

    def v2_runner_on_ok(self, result, *args, **kwargs):
        self.host_ok[result._host.get_name()] = result

    def v2_runner_on_failed(self, result, *args, **kwargs):
        self.host_failed[result._host.get_name()] = result


class PlayBookResultsCollector(CallbackBase):
    CALLBACK_VERSION = 2.0

    def __init__(self, *args, **kwargs):
        super(PlayBookResultsCollector, self).__init__(*args, **kwargs)
        self.task_ok = {}
        self.task_skipped = {}
        self.task_failed = {}
        self.task_status = {}
        self.task_unreachable = {}

    def v2_runner_on_ok(self, result, *args, **kwargs):
        self.task_ok[result._host.get_name()] = result

    def v2_runner_on_failed(self, result, *args, **kwargs):
        self.task_failed[result._host.get_name()] = result

    def v2_runner_on_unreachable(self, result):
        self.task_unreachable[result._host.get_name()] = result

    def v2_runner_on_skipped(self, result):
        self.task_ok[result._host.get_name()] = result

    def v2_playbook_on_stats(self, stats):
        hosts = sorted(stats.processed.keys())
        for h in hosts:
            t = stats.summarize(h)
            self.task_status[h] = {
                "ok": t['ok'],
                "changed": t['changed'],
                "unreachable": t['unreachable'],
                "skipped": t['skipped'],
                "failed": t['failures']
            }


class InvManager(InventoryManager):
    def _enumerate_matches(self, pattern):
        """
        Returns a list of host names matching the given pattern according to the
        rules explained above in _match_one_pattern.
        """

        results = []
        # check if pattern matches group
        matching_groups = self._match_list(self._inventory.groups, pattern)
        if matching_groups:
            for groupname in matching_groups:
                results.extend(self._inventory.groups[groupname].get_hosts())

        # check hosts if no groups matched or it is a regex/glob pattern
        if not matching_groups or pattern.startswith('~') or any(
                special in pattern for special in ('.', '?', '*', '[')):
            # pattern might match host
            matching_hosts = self._match_list(self._inventory.hosts, pattern)
            if matching_hosts:
                for hostname in matching_hosts:
                    results.append(self._inventory.hosts[hostname])

        if not results and pattern in C.LOCALHOST:
            # get_host autocreates implicit when needed
            implicit = self._inventory.get_host(pattern)
            if implicit:
                results.append(implicit)

        # Display warning if specified host pattern did not match any groups or hosts
        if not results and not matching_groups and pattern != 'all':
            logging.info('No {} or {} is null, ignoring.'.format(pattern, pattern))

        if not results and matching_groups and pattern != 'all':
            logging.info('No hosts in {}, ignoring.'.format(pattern))

        return results

    def parse_sources(self, cache=False):
        ''' iterate over inventory sources and parse each one to populate it'''

        self._setup_inventory_plugins()

        parsed = False
        # allow for multiple inventory parsing
        for source in self._sources:

            if source:
                if ',' not in source:
                    source = unfrackpath(source, follow=False)
                parse = self.parse_source(source, cache=cache)
                if parse and not parsed:
                    parsed = True

        if parsed:
            # do post processing
            self._inventory.reconcile_inventory()
        else:
            if C.INVENTORY_UNPARSED_IS_FAILED:
                raise AnsibleError(
                    "No inventory was parsed, please check your configuration and options.")
            # else:
            #     display.warning("No inventory was parsed, only implicit localhost is available")

        self._inventory_plugins = []


class ANSRunner(object):
    """
    This is a General object for parallel execute modules.
    """

    def __init__(self, ips=None, user='tidb', topology=None, inventory=None, tiargs=None, passwd=None,
                 *args, **kwargs):
        self.ips = ips
        self.list_ip_check()
        self.user = user
        self.topology = topology
        self.inventory = inventory
        self.variable_manager = None
        self.loader = None
        self.options = None
        self.password = passwd
        self.callback = None
        try:
            self.forks = tiargs.forks
        except AttributeError:
            self.forks = 5
        self.results_raw = {}
        try:
            self.cluster_name = tiargs.cluster_name
        except AttributeError:
            pass
        self.__initializeData()

    def list_ip_check(self):
        if not self.ips or len(self.ips) < 1:
            return
        iplist = []
        if ',' in self.ips:
            for ip in self.ips.split(','):
                if not ip or not utils.is_valid_ip(ip):
                    continue
                iplist.append(ip)
        else:
            iplist.append(self.ips)
        ipsstr = ','.join(iplist)
        if len(iplist) == 1:
            ipsstr += ','
        return ipsstr

    def __initializeData(self):
        """ 初始化ansible """

        C.DEFAULT_FILTER_PLUGIN_PATH.append('/usr/lib/python2.7/site-packages/tiops/ansibleapi/plugins/filter')
        C.HOST_KEY_CHECKING = False
        C.ANSIBLE_SSH_ARGS = '-C -o ControlMaster=auto -o ControlPersist=1d'
        C.PIPELINING = True
        C.CACHE_PLUGIN = 'jsonfile'
        C.CACHE_PLUGIN_CONNECTION = '~/.ansible/ansible_fact_cache'
        C.CACHE_PLUGIN_TIMEOUT = 86400
        C.DEFAULT_GATHER_TIMEOUT = 120

        Options = namedtuple('Options',
                             ['connection',
                              'module_path',
                              'forks',
                              'timeout',
                              'remote_user',
                              'ask_pass',
                              'private_key_file',
                              'ssh_common_args',
                              'ssh_extra_args',
                              'sftp_extra_args',
                              'scp_extra_args',
                              'become',
                              'become_method',
                              'become_user',
                              'ask_value_pass',
                              'verbosity',
                              'check',
                              'listhosts',
                              'listtasks',
                              'listtags',
                              'syntax',
                              'diff'])

        self.options = Options(connection='smart',
                               module_path=None,
                               forks=self.forks,
                               timeout=60,
                               remote_user=self.user,
                               ask_pass=False,
                               private_key_file=None,
                               ssh_common_args=None,
                               ssh_extra_args=None,
                               sftp_extra_args=None,
                               scp_extra_args=None,
                               become=None,
                               become_method='sudo',
                               become_user='root',
                               ask_value_pass=False,
                               verbosity=None,
                               check=False,
                               listhosts=False,
                               listtasks=False,
                               listtags=False,
                               syntax=False,
                               diff=True)

        # generate an Ansible inventory object from the topology
        def inventory(topology):
            for grp, srvs in topology.items():
                if grp not in self.inventory.groups:
                    self.inventory.add_group(grp)
                for srv in srvs:
                    self.inventory.add_host(
                        srv.get('uuid', srv['ip']), group=grp, port=srv['ssh_port'])

            self.variable_manager = VariableManager(
                loader=DataLoader(), inventory=self.inventory)
            for group in self.inventory.get_groups_dict().iterkeys():
                for items in self.inventory.get_groups_dict()[group]:
                    host = self.inventory.get_host(hostname=str(items))
                    self.variable_manager.set_host_variable(
                        host=host, varname='ansible_user', value=self.user)

                    for vars in topology[group]:
                        if vars.get('uuid', vars['ip']) != items:
                            continue
                        for key, value in vars.items():
                            if key == 'uuid' or key == 'ssh_port':
                                continue
                            if key == 'ip':
                                self.variable_manager.set_host_variable(
                                    host=host, varname='ansible_host', value=value)
                            else:
                                self.variable_manager.set_host_variable(
                                    host=host, varname=key, value=value)
                        self.variable_manager.set_host_variable(host=host, varname='numa_node',
                                                                value=vars['numa_node'])
                        if group == 'tidb_servers':
                            service_name = 'tidb-{}'.format(vars['port'])
                            self.variable_manager.set_host_variable(host=host, varname='service_name',
                                                                    value=service_name)
                        elif group == 'pd_servers':
                            service_name = 'pd-{}'.format(vars['client_port'])
                            self.variable_manager.set_host_variable(host=host, varname='service_name',
                                                                    value=service_name)
                        elif group == 'tikv_servers':
                            service_name = 'tikv-{}'.format(vars['port'])
                            self.variable_manager.set_host_variable(host=host, varname='service_name',
                                                                    value=service_name)
                        elif group == 'grafana_server':
                            service_name = 'grafana-{}'.format(vars['port'])
                            self.variable_manager.set_host_variable(host=host, varname='service_name',
                                                                    value=service_name)
                        elif group == 'pump_servers':
                            service_name = 'pump-{}'.format(vars['port'])
                            self.variable_manager.set_host_variable(host=host, varname='service_name',
                                                                    value=service_name)
                        elif group == 'drainer_servers':
                            service_name = 'drainer-{}'.format(vars['port'])
                            self.variable_manager.set_host_variable(host=host, varname='service_name',
                                                                    value=service_name)
                        elif group == 'alertmanager_server':
                            service_name = 'alertmanager-{}'.format(vars['web_port'])
                            self.variable_manager.set_host_variable(host=host, varname='service_name',
                                                                    value=service_name)

            return inventory

        self.password = dict(conn_pass=self.password)
        self.loader = DataLoader()
        self.ips = self.list_ip_check()
        if self.ips:
            self.inventory = InvManager(loader=self.loader,
                                        sources=self.ips)
            self.variable_manager = VariableManager(
                loader=DataLoader(), inventory=self.inventory)
        else:
            if self.topology:
                self.inventory = InvManager(loader=self.loader,
                                            sources=self.inventory)
                inventory(self.topology)

    def run_model(self, module_name, module_args, become=False, register=None, with_items=None, group='*',
                  extra_vars=None, node=None):
        """
        run module from andible ad-hoc.
        module_name: ansible module_name
        module_args: ansible module args
        """
        if self.topology:
            service_names = {'node_exporter': ['monitored_servers', 'node_exporter_port'],
                             'blackbox_exporter': ['monitored_servers', 'blackbox_exporter_port'],
                             'prometheus': ['monitoring_server', 'prometheus_port'],
                             'pushgateway': ['monitoring_server', 'pushgateway_port']}

            if extra_vars in service_names and service_names[extra_vars][0] in self.inventory.get_groups_dict():
                for host in self.inventory.get_groups_dict()[service_names[extra_vars][0]]:
                    hostname = self.inventory.get_host(hostname=host)
                    service_name = '{}-{}'.format(extra_vars, self.variable_manager.get_vars(
                        host=hostname)[service_names[extra_vars][1]])
                    self.variable_manager.set_host_variable(
                        host=hostname, varname='service_name', value=service_name)

            if self.cluster_name and extra_vars:
                self.variable_manager.extra_vars = {
                    'cluster_name': self.cluster_name, 'service': extra_vars}
            else:
                self.variable_manager.extra_vars = {
                    'cluster_name': self.cluster_name}

        if register and with_items:
            task = [dict(action=dict(module=module_name,
                                     args=module_args),
                         become=become,
                         register=register,
                         with_items=with_items)]
        elif register is None and with_items:
            task = [dict(action=dict(module=module_name,
                                     args=module_args),
                         become=become,
                         with_items=with_items)]
        elif register and with_items is None:
            task = [dict(action=dict(module=module_name,
                                     args=module_args),
                         become=become,
                         register=register)]
        else:
            task = [dict(action=dict(module=module_name,
                                     args=module_args),
                         become=become)]

        if node:
            node_list = node.split(',')
            if len(node_list) == 1:
                node_str = '{},'.format(node)
            else:
                node_str = ','.join(node_list)

        play_source = dict(
            name="Ansible Play",
            hosts=self.ips if self.ips else (node_str if node else group),
            gather_facts='no',
            tasks=task
        )

        play = Play().load(play_source, variable_manager=self.variable_manager, loader=self.loader)
        tqm = None
        self.callback = ModelResultsCollector()
        import traceback
        try:
            tqm = TaskQueueManager(
                inventory=self.inventory,
                variable_manager=self.variable_manager,
                loader=self.loader,
                options=self.options,
                passwords=self.password,
                stdout_callback="minimal",
            )
            tqm._stdout_callback = self.callback
            tqm.run(play)
        except Exception as e:
            term.warn(str(e))
            term.debug(traceback.print_exc())
        finally:
            if tqm is not None:
                tqm.cleanup()

        result = self.get_model_result()
        failed = {}
        unreachable = {}

        offline_list = []

        if self.topology:
            for grp in ['drainer_servers', 'pump_servers', 'tikv_servers']:
                if not self.topology.has_key(grp) or not self.topology[grp]:
                    continue
                for _node in self.topology[grp]:
                    if _node['offline']:
                        offline_list.append(_node['uuid'])

        if result['success']:
            for _uuid, _info in result['success'].iteritems():
                _ip = _info['ansible_host']
                if _info.has_key('stderr') and _info['stderr']:
                    try:
                        failed[_uuid][_ip].append(_info['stderr'])
                    except:
                        if not failed.has_key(_uuid):
                            failed[_uuid] = {}
                        failed[_uuid][_ip] = [_info['stderr']]

        if result['failed']:
            for _uuid, _info in result['failed'].iteritems():
                _ip = _info['ansible_host']
                if _info.has_key('stderr') and _info['stderr']:
                    try:
                        failed[_uuid][_ip].append(_info['stderr'])
                    except:
                        if not failed.has_key(_uuid):
                            failed[_uuid] = {}
                        failed[_uuid][_ip] = [_info['stderr']]
                if _info.has_key('stdout') and _info['stdout']:
                    try:
                        failed[_uuid][_ip].append(_info['stdout'])
                    except:
                        if not failed.has_key(_uuid):
                            failed[_uuid] = {}
                        failed[_uuid][_ip] = [_info['stdout']]
                if _info.has_key('msg') and \
                        _info['msg'] and \
                        "'full_data_dir' is undefined" not in _info['msg'] and \
                        not re.search(r'Could not find.*firewalld', _info['msg']):
                    if _uuid in offline_list and re.search(r'the.*port.*is not up', _info['msg']):
                        continue
                    try:
                        failed[_uuid][_ip].append(_info['msg'])
                    except:
                        if not failed.has_key(_uuid):
                            failed[_uuid] = {}
                        failed[_uuid][_ip] = [_info['msg']]

        if result['unreachable']:
            for _uuid, _info in result['unreachable'].iteritems():
                _ip = _info['ansible_host']
                if _info.has_key('stderr') and _info['stderr']:
                    try:
                        unreachable[_uuid][_ip].append(_info['stderr'])
                    except:
                        if not unreachable.has_key(_uuid):
                            unreachable[_uuid] = {}
                        unreachable[_uuid][_ip] = [_info['stderr']]
                if _info.has_key('stdout') and _info['stdout']:
                    try:
                        unreachable[_uuid][_ip].append(_info['stdout'])
                    except:
                        if not unreachable.has_key(_uuid):
                            unreachable[_uuid] = {}
                        unreachable[_uuid][_ip] = [_info['stdout']]
                if _info.has_key('msg') and _info['msg']:
                    try:
                        unreachable[_uuid][_ip].append(_info['msg'])
                    except:
                        if not unreachable.has_key(_uuid):
                            unreachable[_uuid] = {}
                        unreachable[_uuid][_ip] = [_info['msg']]

        if not failed and not unreachable:
            return result

        msg = {}
        msg['failed'] = failed
        msg['unreachable'] = unreachable
        raise exceptions.TiOPSRuntimeError(msg, result, tp='ansible')

    def run_playbook(self, playbook_path, extra_vars=None):
        """
        运行playbook
        """
        try:
            self.callback = PlayBookResultsCollector()
            if extra_vars:
                self.variable_manager.extra_vars = extra_vars
            executor = PlaybookExecutor(
                playbooks=[playbook_path],
                inventory=self.inventory,
                variable_manager=self.variable_manager,
                loader=self.loader,
                options=self.options,
                passwords=self.password,
            )
            executor._tqm._stdout_callback = self.callback
            executor.run()
        except Exception as e:
            term.warn(str(e))
            return False

    def __ansible_host(self, hostname=None):
        _ansible_host = self.variable_manager.get_vars(host=hostname)[
                'ansible_host'] if 'ansible_host' in self.variable_manager.get_vars(host=hostname) else \
            self.variable_manager.get_vars(host=hostname)['inventory_hostname']
        return _ansible_host

    def get_model_result(self):
        self.results_raw = {
            'success': {},
            'failed': {},
            'unreachable': {}
        }
        for host, result in self.callback.host_ok.items():
            self.results_raw['success'][host] = result._result
            hostname = self.inventory.get_host(hostname=str(host))
            self.results_raw['success'][host]['ansible_host'] = self.__ansible_host(hostname)

        for host, result in self.callback.host_failed.items():
            self.results_raw['failed'][host] = result._result
            hostname = self.inventory.get_host(hostname=str(host))
            self.results_raw['failed'][host]['ansible_host'] = self.__ansible_host(hostname)

        for host, result in self.callback.host_unreachable.items():
            self.results_raw['unreachable'][host] = result._result
            hostname = self.inventory.get_host(hostname=str(host))
            self.results_raw['unreachable'][host]['ansible_host'] = self.__ansible_host(hostname)

        return self.results_raw

    def get_playbook_result(self):
        self.results_raw = {
            'skipped': {},
            'failed': {},
            'ok': {},
            "status": {},
            'unreachable': {},
            "changed": {}
        }
        for host, result in self.callback.task_ok.items():
            self.results_raw['ok'][host] = result._result
            hostname = self.inventory.get_host(hostname=str(host))
            self.results_raw['ok'][host]['ansible_host'] = self.__ansible_host(hostname)

        for host, result in self.callback.task_failed.items():
            self.results_raw['failed'][host] = result._result
            hostname = self.inventory.get_host(hostname=str(host))
            self.results_raw['failed'][host]['ansible_host'] = self.__ansible_host(hostname)

        for host, result in self.callback.task_status.items():
            self.results_raw['status'][host] = result
            hostname = self.inventory.get_host(hostname=str(host))
            self.results_raw['status'][host]['ansible_host'] = self.__ansible_host(hostname)

        for host, result in self.callback.task_skipped.items():
            self.results_raw['skipped'][host] = result._result
            hostname = self.inventory.get_host(hostname=str(host))
            self.results_raw['skipped'][host]['ansible_host'] = self.__ansible_host(hostname)

        for host, result in self.callback.task_unreachable.items():
            self.results_raw['unreachable'][host] = result._result
            hostname = self.inventory.get_host(hostname=str(host))
            self.results_raw['unreachable'][host]['ansible_host'] = self.__ansible_host(hostname)
        return self.results_raw


if __name__ == '__main__':
    a = "192.168.111.137,127.0.0.1"
    rbt = ANSRunner(a)
    rbt.run_model('shell', 'uptime')
    result = json.dumps(rbt.get_model_result(), indent=4)
    print(result)
