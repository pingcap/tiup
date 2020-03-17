# coding: utf-8


import os
import re
import sys
import json

from tiops import exceptions
from tiops import utils
from shutil import copy, rmtree
from tiops.tui import term
from tiops.utils import ticontinue, read_yaml, profile_path
from tiops.modules.api import ClusterAPI
from tiops.ansibleapi import ansibleapi

# packages url
component_tpls = {'tidb': 'https://download.pingcap.org/tidb-{}-linux-amd64.tar.gz',
                  'tidb-toolkit': 'https://download.pingcap.org/tidb-toolkit-{}-linux-amd64.tar.gz',
                  'fio': 'https://download.pingcap.org/fio-3.8.tar.gz',
                  'tidb-insight': 'https://download.pingcap.org/tidb-insight-v0.2.5-1-g99b8fea.tar.gz',
                  'prometheus': 'https://download.pingcap.org/prometheus-2.8.1.linux-amd64.tar.gz',
                  'alertmanager': 'https://download.pingcap.org/alertmanager-0.17.0.linux-amd64.tar.gz',
                  'node_exporter': 'https://download.pingcap.org/node_exporter-0.17.0.linux-amd64.tar.gz',
                  'blackbox_exporter': 'https://download.pingcap.org/blackbox_exporter-0.12.0.linux-amd64.tar.gz',
                  'pushgateway': 'https://download.pingcap.org/pushgateway-0.7.0.linux-amd64.tar.gz',
                  'grafana': 'https://download.pingcap.org/grafana-6.1.6.linux-amd64.tar.gz',
                  'spark': 'https://download.pingcap.org/spark-2.4.3-bin-hadoop2.7.tgz',
                  'tispark': 'https://download.pingcap.org/tispark-assembly-2.2.0.jar',
                  'tispark_sampla_data': 'https://download.pingcap.org/tispark-sample-data.tar.gz',
                  }


def do_download(components, version,
                cfg_dir, dash_dir, pkg_dir, res_dir):
    # download packages and untar
    for component, url in components.items():
        fname = url.split('/')[-1]
        if os.path.isfile(fname):
            continue
        term.normal('Downloading {}'.format(url))

        code = os.popen('curl -s --connect-timeout 10 www.baidu.com 2>/dev/null >/dev/null; echo $?').read().strip('\n')
        if int(code):
            raise exceptions.TiOPSRuntimeError(msg='The Control Machine have not access to the internet network.',
                                               operation='download')

        _data, _code = utils.read_url(url, timeout=5)
        if not (_code >= 200 and _code < 300):
            _msg = 'Failed downloading {}.'.format(fname)
            raise exceptions.TiOPSRequestError(url, _code, _msg)
        utils.write_file(fname, _data, 'wb')
        if component in ['grafana', 'tidb-insight', 'spark', 'tispark']:
            copy(os.path.join(pkg_dir, fname), res_dir)
        else:
            try:
                utils.untar(fname)
            except Exception as e:
                term.error('Failed uncompressing {}, {}.'.format(fname, e))

    # copy binary and remove directory
    for curfile in utils.list_dir('.'):
        if re.search(r'tidb-insight', curfile):
            copy(curfile, res_dir)
        if not os.path.isdir(curfile):
            continue

        monitors = ['alertmanager',
                    'prometheus',
                    'node_exporter',
                    'pushgateway',
                    'blackbox_exporter']

        monitor_name = None
        for monitor in monitors:
            if monitor in curfile:
                monitor_name = monitor
                break

        if monitor_name:
            copy(os.path.join(curfile, monitor_name), res_dir)
            rmtree(curfile)
            continue

        for root, dirs, files in os.walk(curfile):
            if re.search(r'tispark-sample-data', root):
                utils.create_dir(os.path.join(res_dir, root))
                for file in files:
                    copy(os.path.join(root, file),
                         os.path.join(res_dir, root))
            else:
                if not files:
                    continue

                for file in files:
                    copy(os.path.join(root, file), res_dir)
        rmtree(curfile)

    # download file from github
    def _save_file(url, path, file_name):
        _data, _code = utils.read_url(url, timeout=5)
        if not (_code >= 200 and _code < 300):
            _msg = 'Failed downloading {}.'.format(file_name)
            raise exceptions.TiOPSRequestError(url, _code, _msg)

        utils.write_file(os.path.join(path, file_name), _data, 'a')

    # configuration url
    toml_config_url = {
        'pd': 'https://raw.githubusercontent.com/pingcap/pd/{}/conf/config.toml'.format(version),
        'tidb': 'https://raw.githubusercontent.com/pingcap/tidb/{}/config/config.toml.example'.format(version),
        'tikv': 'https://raw.githubusercontent.com/tikv/tikv/{}/etc/config-template.toml'.format(version),
        'pump': 'https://raw.githubusercontent.com/pingcap/tidb-binlog/{}/cmd/pump/pump.toml'.format(version),
        'drainer': 'https://raw.githubusercontent.com/pingcap/tidb-binlog/{}/cmd/drainer/drainer.toml'.format(version),
        'lightning': 'https://raw.githubusercontent.com/pingcap/tidb-lightning/{}/tidb-lightning.toml'.format(version),
        'importer': 'https://raw.githubusercontent.com/tikv/importer/{}/etc/tikv-importer.toml'.format(version),
    }
    yaml_config_url = {
        'alertmanager': 'https://raw.githubusercontent.com/pingcap/tidb-ansible/master/conf/alertmanager.yml',
        'blackbox': 'https://raw.githubusercontent.com/pingcap/tidb-ansible/master/roles/blackbox_exporter/templates/blackbox.yml.j2',
        'binlog.rules': 'https://raw.githubusercontent.com/pingcap/tidb-ansible/{}/roles/prometheus/files/binlog.rules.yml'.format(
            version),
        'blacker.rules': 'https://raw.githubusercontent.com/pingcap/tidb-ansible/{}/roles/prometheus/files/blacker.rules.yml'.format(
            version),
        'bypass.rules': 'https://raw.githubusercontent.com/pingcap/tidb-ansible/{}/roles/prometheus/files/bypass.rules.yml'.format(
            version),
        'kafka.rules': 'https://raw.githubusercontent.com/pingcap/tidb-ansible/{}/roles/prometheus/files/kafka.rules.yml'.format(
            version),
        'lightning.rules': 'https://raw.githubusercontent.com/pingcap/tidb-ansible/{}/roles/prometheus/files/lightning.rules.yml'.format(
            version),
        'node.rules': 'https://raw.githubusercontent.com/pingcap/tidb-ansible/{}/roles/prometheus/files/node.rules.yml'.format(
            version),
        'tidb.rules': 'https://raw.githubusercontent.com/pingcap/tidb-ansible/{}/roles/prometheus/files/tidb.rules.yml'.format(
            version),
        'tikv.rules': 'https://raw.githubusercontent.com/pingcap/tidb-ansible/{}/roles/prometheus/files/tikv.rules.yml'.format(
            version),
        'pd.rules': 'https://raw.githubusercontent.com/pingcap/tidb-ansible/{}/roles/prometheus/files/pd.rules.yml'.format(
            version),
    }
    if version == 'latest' or version[1] > '3':
        yaml_config_url[
            'tikv.accelerate.rules'] = 'https://raw.githubusercontent.com/pingcap/tidb-ansible/{}/roles/prometheus/files/tikv.accelerate.rules.yml'.format(
            version)

    # download configs
    for component, url in toml_config_url.iteritems():
        url = url.format(version)
        if not os.path.isfile(os.path.join(cfg_dir, component + '.toml')):
            _save_file(url, cfg_dir, component + '.toml')
        utils.processTidbConfig(
            os.path.join(cfg_dir, component + '.toml'), component)

    for component, url in yaml_config_url.iteritems():
        if not os.path.isfile(os.path.join(cfg_dir, component + '.yml')):
            _save_file(url, cfg_dir, component + '.yml')

    # dashboard url
    dashboards = ['pd.json', 'tidb.json', 'tidb_summary.json', 'tikv_summary.json', 'tikv_details.json',
                  'tikv_trouble_shooting.json', 'performance_read.json', 'performance_write.json', 'binlog.json',
                  'lightning.json', 'overview.json', 'node.json', 'kafka.json', 'blackbox_exporter.json',
                  'disk_performance.json']
    url = 'https://raw.githubusercontent.com/pingcap/tidb-ansible/{}/scripts/'.format(
        version)

    # download dashboards
    for dashboard in dashboards:
        if not os.path.isfile(os.path.join(dash_dir, dashboard)):
            _save_file(url + dashboard,
                       dash_dir, dashboard)


class Action(object):

    def __init__(self, ans=None, topo=None):
        self.ans = ans
        self.topo = topo

        if self.topo.version != 'latest':
            self._tidb_version = 'v{}'.format(self.topo.version.lstrip('v'))
        self.components = self.__update_components(self._tidb_version)

        self.binary_name = {'blackbox_exporter': 'blackbox_exporter',
                            'node_exporter': 'node_exporter',
                            'prometheus': 'prometheus',
                            'alertmanager': 'alertmanager',
                            'tidb': 'tidb-server',
                            'tikv': 'tikv-server',
                            'pd': 'pd-server',
                            'pump': 'pump',
                            'drainer': 'drainer',
                            'br': 'br'}

        self.alert_rule = ['binlog.rules.yml',
                           'blacker.rules.yml',
                           'bypass.rules.yml',
                           'kafka.rules.yml',
                           'lightning.rules.yml',
                           'node.rules.yml',
                           'tidb.rules.yml',
                           'tikv.rules.yml',
                           'pd.rules.yml']

        if self._tidb_version == 'latest' or self._tidb_version[1] > '3':
            self.alert_rule.append('tikv.accelerate.rules.yml')

        self.config_name = {'blackbox_exporter': 'blackbox.yml',
                            'prometheus': 'prometheus.yml.j2',
                            'alertmanager': 'alertmanager.yml',
                            'tidb': 'tidb.toml',
                            'tikv': 'tikv.toml',
                            'pd': 'pd.toml',
                            'pump': 'pump.toml',
                            'drainer': 'drainer.toml'}

        self.tool_name = {'binlogctl': 'binlogctl',
                          'etcdctl': 'etcdctl',
                          'mydumper': 'mydumper',
                          'pd-ctl': 'pd-ctl',
                          'pd-recover': 'pd-recover',
                          'reparo': 'reparo',
                          'sync_diff_inspector': 'sync_diff_inspector',
                          'tidb-ctl': 'tidb-ctl',
                          'tidb-lightning-ctl': 'tidb-lightning-ctl',
                          'tikv-ctl': 'tikv-ctl'}

    def __update_components(self, version):
        version = 'v{}'.format(version.lstrip('v'))
        _components = component_tpls.copy()
        _components['tidb'] = _components['tidb'].format(version)
        _components['tidb-toolkit'] = _components['tidb-toolkit'].format(version)
        return _components

    def check_exist(self, service, config=None):
        if not config:
            config = self.topo()
        for item in service.items():
            component, pattern = item
        if not pattern in config.keys() or not config[pattern]:
            return False, False
        return component, pattern

    def check_machine_config(self, diff=None):
        _servers = [{'pd': 'pd_servers'},
                    {'tikv': 'tikv_servers'},
                    {'tidb': 'tidb_servers'},
                    {'prometheus': 'monitoring_server'}]

        # loop groups which need to check
        if diff:
            config = diff
        else:
            config = self.topo()

        def check_cpu():
            _min_cpu = {'pd': int(self.topo.pd_min_cpu),
                        'tikv': int(self.topo.tikv_min_cpu),
                        'tidb': int(self.topo.tidb_min_cpu),
                        'prometheus': int(self.topo.monitor_min_cpu)}
            _lower_cpu = {}
            for _service in _servers:
                _component, _pattern = self.check_exist(
                    _service, config=config)
                if not _component and not _pattern:
                    continue
                # loop host node for check cpu num
                for node in config[_pattern]:
                    _ip = node['ip']
                    _host_vars_path = os.path.join(
                        profile_path('host_vars'), '{}.yaml'.format(_ip))
                    _current_cpu_num = read_yaml(_host_vars_path)[
                        'ansible_facts']['ansible_processor_vcpus']
                    # if cpu vcores num less than min, record it in list.
                    if int(_current_cpu_num) < _min_cpu[_component]:
                        # if group not in list, will add group and node. if group in list, add node.
                        if _component not in _lower_cpu:
                            _lower_cpu[_component] = [[_ip, _current_cpu_num]]
                        else:
                            _lower_cpu[_component].append(
                                [_ip, _current_cpu_num])

            if _lower_cpu:
                for _component, _nodes in _lower_cpu.iteritems():
                    term.warn(
                        'The list of machines running {} with less than {} of CPU Vcores:'.format(_component,
                                                                                                  _min_cpu[_component]))
                    _length = max(max([len(x[0]) for x in _nodes]), len('IP'))
                    term.normal('IP'.ljust(_length + 2) + 'Vcores')
                    for _node in _nodes:
                        term.normal('{}{}'.format(
                            _node[0].ljust(_length + 2), _node[1]))
                term.warn('Performance of cluster using this configuration may be lower.')
                if not ticontinue():
                    exit(1)

        def check_mem():
            _min_mem = {'pd': int(self.topo.pd_min_ram),
                        'tikv': int(self.topo.tikv_min_ram),
                        'tidb': int(self.topo.tidb_min_ram),
                        'prometheus': int(self.topo.monitor_min_ram)}
            _lower_mem = {}
            for _service in _servers:
                _component, _pattern = self.check_exist(
                    _service, config=config)
                if not _component and not _pattern:
                    continue

                # loop host node for check memory size
                for node in config[_pattern]:
                    _ip = node['ip']
                    _host_vars_path = os.path.join(
                        profile_path('host_vars'), '{}.yaml'.format(_ip))
                    _current_memory_size = read_yaml(_host_vars_path)[
                        'ansible_facts']['ansible_memtotal_mb']
                    # if memory size less than min, record it in list.
                    if int(_current_memory_size) < _min_mem[_component]:
                        if _component not in _lower_mem:
                            _lower_mem[_component] = [
                                [_ip, _current_memory_size]]
                        else:
                            _lower_mem[_component].append(
                                [_ip, _current_memory_size])

            if _lower_mem:
                for _component, _nodes in _lower_mem.iteritems():
                    term.warn(
                        'The list of machines running {} with less than {} MB of memory:'.format(_min_mem[_component],
                                                                                                 _component))
                    _length = max(max([len(x[0]) for x in _nodes]), len('IP'))
                    term.normal('IP'.ljust(_length + 2) + 'Memory(MB)')
                    for _node in _nodes:
                        term.normal('{}{}'.format(
                            _node[0].ljust(_length + 2), _node[1]))
                term.warn('Performance of cluster using this configuration may be lower.')
                if not ticontinue():
                    exit(1)

        def check_disk():
            _min_disk = {'pd': int(self.topo.pd_min_disk),
                         'tikv': int(self.topo.tikv_min_disk),
                         'prometheus': int(self.topo.monitor_min_disk)}
            _lower_disk = {}
            for _service in _servers:
                _component, _pattern = self.check_exist(
                    _service, config=config)
                if not _component and not _pattern:
                    continue

                if diff:
                    _uuid = ','.join([x['uuid'] for x in config[_pattern]])
                else:
                    _uuid = None

                if _component != 'tidb':
                    _result = self.ans.run_model('shell',
                                                 "df {{ full_data_dir }} | tail -n1 | awk '{print $NF}'",
                                                 group=_pattern,
                                                 node=_uuid)
                else:
                    continue

                # loop host node for check disk space
                for node in config[_pattern]:
                    _ip = node['ip']
                    _data_dir = node['full_data_dir']
                    _host_vars_path = os.path.join(
                        profile_path('host_vars'), '{}.yaml'.format(_ip))
                    # begin check disk available space
                    if _component == 'tidb':
                        continue

                    _result_node = {}
                    # loop to check disk space
                    for _node, _info in _result['success'].iteritems():
                        if _info['ansible_host'] == _ip and _data_dir in _info['cmd']:
                            _result_node = _info
                            break

                    _mount_dir = _result_node['stdout']
                    _current_disk_space = \
                        [x['size_available'] for x in read_yaml(_host_vars_path)['ansible_facts']['ansible_mounts'] if
                         x['mount'] == _mount_dir][0]
                    # if available disk space less than min size, record it in list
                    if int(_current_disk_space) < _min_disk[_component]:
                        if _component not in _lower_disk:
                            _lower_disk[_component] = [
                                [_ip, _data_dir, _current_disk_space / 1024 / 1024 / 1024]]
                        else:
                            _lower_disk[_component].append(
                                [_ip, _data_dir, _current_disk_space / 1024 / 1024 / 1024])

            if _lower_disk:
                for _component, _nodes in _lower_disk.iteritems():
                    _length1 = max(max([len(x[0]) for x in _nodes]), len('IP'))
                    _length2 = max(max([len(x[1])
                                        for x in _nodes]), len('Data_dir'))
                    term.warn(
                        'The list of machines running {} with less than {} GB of available disk space size:'.format(
                            _component, _min_disk[_component] / 1000000000))
                    term.normal('IP'.ljust(
                        _length1 + 2) + 'Data_dir'.ljust(_length2 + 2) + 'Available_size(GB)')
                    for _node in _nodes:
                        term.normal(
                            '{}{}{}'.format(_node[0].ljust(_length1 + 2), _node[1].ljust(_length2 + 2), _node[2]))
                term.warn('Data disk available space of some nodes is small.')
                if not ticontinue():
                    exit(1)

        def check_tikv_disk():
            _filesystem = []
            _lower_randread_iops = []
            _lower_mix_iops = []
            _greater_mix_latency = []
            _component, _pattern = self.check_exist(
                {'tikv': 'tikv_servers'}, config=config)
            if not _component and not _pattern:
                return

            if diff:
                _uuid = ','.join([x['uuid'] for x in config[_pattern]])
            else:
                _uuid = None

            self.ans.run_model('file',
                               'name={{ full_data_dir }}/check '
                               'state=directory',
                               group=_pattern,
                               node=_uuid)
            self.ans.run_model('copy',
                               'src={{ item }} '
                               'dest={{ full_data_dir }}/check '
                               'mode=0755',
                               with_items=['{}/check_disk.py'.format(self.topo.ticheck_dir),
                                           '{}/fio'.format(self.topo.resource_dir)],
                               group=_pattern,
                               node=_uuid)

            _fstype_result = self.ans.run_model('shell',
                                                'cd {{ full_data_dir }}/check && '
                                                './check_disk.py --filesystem',
                                                group=_pattern,
                                                node=_uuid)

            for node in config[_pattern]:
                _ip = node['ip']
                _data_dir = node['full_data_dir']

                _fstype = {}
                for _node, _info in _fstype_result['success'].iteritems():
                    if _info['ansible_host'] == _ip and _data_dir in _info['cmd']:
                        _fstype = eval(_info['stdout'])
                        break

                if 'successful' not in _fstype['filesystem']:
                    _filesystem.append([_ip, _data_dir, _fstype['filesystem']])

            if _filesystem:
                _length1 = max(max([len(x[0])
                                    for x in _filesystem]), len('IP'))
                _length2 = max(max([len(x[1])
                                    for x in _filesystem]), len('Data_dir'))
                term.fatal('File system check error:')
                term.normal('IP'.ljust(_length1 + 2) +
                            'Data_dir'.ljust(_length2 + 2) + 'Msg')
                for _node in _filesystem:
                    term.normal('{}{}{}'.format(_node[0].ljust(
                        _length1 + 2), _node[1].ljust(_length2 + 2), _node[2]))
                    exit(1)

            _disk_result = self.ans.run_model('shell',
                                              'cd {{ full_data_dir }}/check && '
                                              './check_disk.py --read-iops --write-iops --latency',
                                              group=_pattern,
                                              node=_uuid)

            # loop host node for check tikv data dir filesystem
            for node in config[_pattern]:
                _ip = node['ip']
                _data_dir = node['full_data_dir']

                _fio = {}
                for _node, _info in _disk_result['success'].iteritems():
                    if _info['ansible_host'] == _ip and _data_dir in _info['cmd']:
                        _fio = eval(_info['stdout'])
                        break

                if _fio['read_iops'] < self.topo.randread_iops:
                    _lower_randread_iops.append(
                        [_ip, _data_dir, _fio['read_iops']])
                if _fio['mix_read_iops'] < self.topo.mix_read_iops or \
                        _fio['mix_write_iops'] < self.topo.mix_write_iops:
                    _lower_mix_iops.append([_ip, _data_dir, _fio['mix_read_iops'], _fio['mix_write_iops']])
                if _fio['mix_read_latency'] > self.topo.mix_read_lat or \
                        _fio['mix_write_latency'] > self.topo.mix_write_lat:
                    _greater_mix_latency.append([_ip, _data_dir, _fio['mix_read_latency'], _fio['mix_write_latency']])

            self.ans.run_model('file',
                               'name={{ full_data_dir }}/check '
                               'state=absent',
                               group=_pattern,
                               node=_uuid)

            if _lower_randread_iops:
                term.warn(
                    'The list of machines running TiKV with less than {} of randread iops:'.format(
                        self.topo.randread_iops))
                _ri_length1 = max(
                    max([len(x[0]) for x in _lower_randread_iops]), len('IP'))
                _ri_length2 = max(
                    max([len(x[1]) for x in _lower_randread_iops]), len('Data_dir'))
                term.normal('IP'.ljust(_ri_length1 + 2) +
                            'Data_dir'.ljust(_ri_length2 + 2) + 'IOPS')
                for _node in _lower_randread_iops:
                    term.normal(
                        '{}{}{}'.format(_node[0].ljust(_ri_length1 + 2), _node[1].ljust(_ri_length2 + 2), _node[2]))
                if not ticontinue():
                    exit(1)
            if _lower_mix_iops:
                term.warn(
                    'The list of machines running TiKV with less than {} of randread iops or with less than {} of write iops for mix test:'.format(
                        self.topo.mix_read_iops, self.topo.mix_write_iops))
                _mi_length1 = max(max([len(x[0])
                                       for x in _lower_mix_iops]), len('IP'))
                _mi_length2 = max(
                    max([len(x[1]) for x in _lower_mix_iops]), len('Data_dir'))
                _mi_length3 = max(
                    max([len(str(x[2])) for x in _lower_mix_iops]), len('Read_IOPS'))
                term.normal('IP'.ljust(_mi_length1 + 2) + 'Data_dir'.ljust(_mi_length2 + 2) + 'Read_IOPS'.ljust(
                    _mi_length3 + 2) + 'Write_IOPS')
                for _node in _lower_mix_iops:
                    term.normal('{}{}{}{}'.format(_node[0].ljust(_mi_length1 + 2), _node[1].ljust(_mi_length2 + 2),
                                                  str(_node[2]).ljust(_mi_length3 + 2), _node[3]))
                if not ticontinue():
                    exit(1)
            if _greater_mix_latency:
                term.warn(
                    'The list of machines running TiKV with greater than {}ms of randread latency or with greater than {}ms of write latency for mix test.'.format(
                        self.topo.mix_read_lat, self.topo.mix_write_lat))
                _ml_length1 = max(max([len(x[0]) for x in _greater_mix_latency]), len('IP'))
                _ml_length2 = max(max([len(x[1]) for x in _greater_mix_latency]), len('Data_dir'))
                _ml_length3 = max(max([len(str(x[2])) for x in _greater_mix_latency]), len('Read_LAT(ms)'))
                term.normal('IP'.ljust(_ml_length1 + 2) + 'Data_dir'.ljust(_ml_length2 + 2) + 'Read_LAT(ms)'.ljust(
                    _ml_length3 + 2) + 'Write_LAT(ms)')
                for _node in _greater_mix_latency:
                    term.normal('{}{}{}{}'.format(_node[0].ljust(_ml_length1 + 2), _node[1].ljust(_ml_length2 + 2),
                                                  str(_node[2]).ljust(_ml_length3 + 2), _node[3]))
                if not ticontinue():
                    exit(1)

        if self.topo.enable_check_all or self.topo.enable_check_cpu:
            term.info('Check if the number of CPU vcores meets the requirements.')
            check_cpu()
        if self.topo.enable_check_all or self.topo.enable_check_mem:
            term.info('Check if the size of memory meets the requirements.')
            check_mem()
        if self.topo.enable_check_all or self.topo.enable_check_disk:
            term.info(
                'Check if the available size of data disk meets the requirements.')
            check_disk()
        if self.topo.enable_check_all or self.topo.enable_check_iops:
            term.info(
                'Check if the file system type and performance of data disk meets the requirements.')
            check_tikv_disk()

    def check_ssh_connection(self):
        try:
            self.ans.run_model('ping',
                               'data="pong"',
                               group='*')
        except exceptions.TiOPSRuntimeError as e:
            raise exceptions.TiOPSRuntimeError(msg=e.msg, operation='check_ssh', tp='ansible')

    # Download binary/config/dashboard
    def download(self, version=None, local_pkg=None):
        utils.create_dir(self.topo.ticache_dir)
        utils.create_dir(self.topo.tidown_dir)
        utils.create_dir(self.topo.tiversion_dir)
        utils.create_dir(self.topo.cluster_dir)

        # create download dirs
        for dname in [self.topo.package_dir, self.topo.resource_dir, self.topo.config_dir,
                      self.topo.dashboard_dir]:
            utils.create_dir(dname)

        # packages url
        dl_tidb_version = 'v{}'.format(version) if version else self._tidb_version
        components = self.__update_components(dl_tidb_version)

        if local_pkg:
            term.notice('Using pre-downloaded packages in {}.'.format(local_pkg))
            utils.copy_dir(local_pkg, self.topo.ticache_dir)
        else:
            _cwd = utils.cwd()
            utils.chdir(self.topo.package_dir)
            do_download(components, dl_tidb_version,
                        self.topo.config_dir,
                        self.topo.dashboard_dir,
                        self.topo.package_dir,
                        self.topo.resource_dir
                        )
            utils.chdir(_cwd)

        utils.create_dir(self.topo.cluster_configs)

        for service in ['tidb', 'tikv', 'pd', 'pump', 'drainer', 'alertmanager']:
            _group = [x.values()[0] for x in self.topo.service_group if service in x][0]
            if _group not in self.topo() or not self.topo()[_group]:
                continue

            if service == 'alertmanager':
                if os.path.exists(self.topo.meta_file) and os.path.exists(
                        os.path.join(self.topo.cluster_configs, self.config_name['alertmanager'])):
                    pass
                else:
                    copy(
                        os.path.join(self.topo.titemplate_dir,
                                     'common_config', 'alertmanager.yml'),
                        self.topo.cluster_configs)
            elif service == 'drainer':
                for _drainer in self.topo()[_group]:
                    _file_name = re.sub(r'\.', '_{}_{}.'.format(_drainer['ip'], _drainer['port']),
                                        self.config_name[service])
                    if os.path.exists(self.topo.meta_file) and os.path.exists(
                            os.path.join(self.topo.cluster_configs, _file_name)):
                        pass
                    else:
                        copy(os.path.join(self.topo.config_dir, self.config_name[service]),
                             os.path.join(self.topo.cluster_configs, _file_name))
            else:
                if os.path.exists(self.topo.meta_file) and os.path.exists(
                        os.path.join(self.topo.cluster_configs, self.config_name[service])):
                    pass
                else:
                    copy(os.path.join(self.topo.config_dir,
                                      self.config_name[service]), self.topo.cluster_configs)

    def _editfile(self):
        path = self.topo.cluster_configs
        file_list = os.listdir(path)
        flist = {}
        for _fname in file_list:
            if re.search(r'(.*).toml', _fname):
                flist[re.search(r'(.*).toml', _fname).group(1)] = os.path.join(path, _fname)
            else:
                flist[re.search(r'(.*).yml', _fname).group(1)] = os.path.join(path, _fname)

        info = 'Config list: {}'.format(flist.keys())
        hinfo = 'Enter ' + term.highlight_green('the config name') + ' to modify the configuration or ' \
                + term.highlight_green('q') + ' to exit configuration setting or ' \
                + term.highlight_green('p') + ' to get the list of modifiable services or ' \
                + term.highlight_green('h') + ' for help.'
        str_input = 'Please choice one config: '
        while True:
            term.normal(info)
            term.normal(hinfo)
            while True:
                opt = term.input(term.highlight_cyan(str_input))
                if opt in flist.keys():
                    utils.editfile(flist[opt])
                elif opt.lower() == 'q':
                    break
                elif opt.lower() == 'p':
                    term.normal(info)
                elif opt.lower() == 'h':
                    term.normal(hinfo)
                elif not opt or (opt not in flist.keys()):
                    term.fatal('Invalid input, please retry.')
                    continue
            break

    def edit_file(self):
        term.notice('Start edit configuration.')
        self._editfile()
        term.notice('Configuration file saved.')

    def create_directory(self, component=None, pattern=None, node=None):
        self.ans.run_model('file',
                           'name="{{ full_deploy_dir }}" '
                           'state=directory '
                           'mode="0755" '
                           'owner="{{ ansible_user }}" '
                           'group="{{ ansible_user }}"',
                           become=True,
                           group=pattern,
                           node=node)

        if component in ['pd', 'tikv', 'pump', 'drainer', 'prometheus']:
            self.ans.run_model('file',
                               'name="{{ full_data_dir }}" '
                               'state=directory '
                               'mode="0755" '
                               'owner="{{ ansible_user }}" '
                               'group="{{ ansible_user }}"',
                               become=True,
                               group=pattern,
                               node=node)

        for dir_name in ['bin', 'scripts', 'log', 'status', 'conf']:
            self.ans.run_model('file',
                               'name="{{ full_deploy_dir }}/%s" '
                               'state=directory '
                               'mode="0755" '
                               'owner="{{ ansible_user }}" '
                               'group="{{ ansible_user }}"' % (
                                   dir_name),
                               become=True,
                               group=pattern,
                               node=node)

    def deploy_component(self, component=None, pattern=None, node=None, template_dir=None):
        if template_dir:
            _template_dir = template_dir
        else:
            _template_dir = self.topo.titemplate_dir

        if component == 'grafana':
            # distribute grafana package
            self.ans.run_model('unarchive',
                               'src=%s/%s '
                               'dest={{ full_deploy_dir }}' % (
                                   self.topo.resource_dir, os.path.split(self.components['grafana'])[1]),
                               group=pattern,
                               node=node)

            # create necessary directory
            self.ans.run_model('file',
                               'path="{{ full_deploy_dir }}/grafana-6.1.6/{{ item }}" '
                               'state=directory '
                               'mode=0755',
                               with_items=['provisioning/datasources', 'provisioning/dashboards',
                                           'dashboards/' + self.topo.cluster_name, 'plugins'],
                               group=pattern,
                               node=node)

            # gen grafana config
            self.ans.run_model('template',
                               'src=%s/%s '
                               'dest={{ full_deploy_dir }}/grafana-6.1.6/conf/grafana.ini '
                               'mode=0644' % (
                                   os.path.join(_template_dir, 'common_config'), 'grafana.ini.j2'),
                               group=pattern,
                               node=node)

            self.ans.run_model('template',
                               'src=%s/%s '
                               'dest={{ full_deploy_dir }}/grafana-6.1.6/provisioning/datasources/{{ cluster_name }}.yml '
                               'mode=0644' % (
                                   os.path.join(
                                       _template_dir, 'common_config'),
                                   'datasource.yml.j2'),
                               group=pattern,
                               node=node)

            self.ans.run_model('template',
                               'src=%s/%s '
                               'dest={{ full_deploy_dir }}/grafana-6.1.6/provisioning/dashboards/{{ cluster_name }}.yml '
                               'mode=0644' % (
                                   os.path.join(
                                       _template_dir, 'common_config'),
                                   'dashboard.yml.j2'),
                               group=pattern,
                               node=node)

            # distribute dashboard
            dashboard_list = ["node.json", "pd.json", "tidb.json", "tidb_summary.json",
                              "tikv_summary.json", "tikv_details.json", "tikv_trouble_shooting.json", "binlog.json",
                              "overview.json", "disk_performance.json", "blackbox_exporter.json",
                              "kafka.json", "lightning.json"]

            self.ans.run_model('copy',
                               'src=%s/{{ item }} '
                               'dest={{ full_deploy_dir }}/grafana-6.1.6/dashboards/{{ cluster_name }}' % os.path.join(
                                   self.topo.dashboard_dir),
                               with_items=dashboard_list,
                               group=pattern,
                               node=node)

            self.ans.run_model('replace',
                               'path={{ full_deploy_dir }}/grafana-6.1.6/dashboards/{{ cluster_name }}/{{ item }} '
                               'regexp="^  \"title\": .*," '
                               'replace="  \"title\": \"{{ cluster_name }}-{{ item.split(\'.\')[0] | replace(\'_\', \'-\') | title | replace(\'Tidb\', \'TiDB\') | replace(\'TiKV\', \'TiKV\') | replace(\'Pd\', \'PD\')}}\","',
                               with_items=dashboard_list,
                               group=pattern,
                               node=node)

            self.ans.run_model('replace',
                               'path={{ full_deploy_dir }}/grafana-6.1.6/dashboards/{{ cluster_name }}/{{ item }} '
                               'regexp="^  \"uid\": .*," '
                               'replace="  \"uid\": \"{{ cluster_name }}-{{ item.split(\'.\')[0] }}\","',
                               with_items=dashboard_list,
                               group=pattern,
                               node=node)

            self.ans.run_model('replace',
                               'path={{ full_deploy_dir }}/grafana-6.1.6/dashboards/{{ cluster_name }}/{{ item }} '
                               'regexp="\"datasource\": .*," '
                               'replace="\"datasource\": \"{{ cluster_name }}\","',
                               with_items=dashboard_list,
                               group=pattern,
                               node=node)
        else:
            # distribute binary
            self.ans.run_model('copy',
                               'src=%s/%s '
                               'dest={{ full_deploy_dir }}/bin/ '
                               'mode=0755' % (
                                   self.topo.resource_dir, self.binary_name[component]),
                               group=pattern,
                               node=node)

            # set jurisdiction for blackbox_exporter binary
            if component == 'blackbox_exporter':
                self.ans.run_model('command',
                                   'setcap cap_net_raw+ep "{{ full_deploy_dir }}/bin/blackbox_exporter"',
                                   become=True,
                                   group=pattern,
                                   node=node)

            # distribute config
            if component in self.config_name:
                if component == 'prometheus':
                    self.ans.run_model('template',
                                       'src=%s/%s '
                                       'dest={{ full_deploy_dir }}/conf/prometheus.yml '
                                       'mode=0644 '
                                       'backup=yes' % (
                                           os.path.join(
                                               _template_dir, 'common_config'),
                                           self.config_name[component]),
                                       group=pattern,
                                       node=node)

                    self.ans.run_model('copy',
                                       'src=%s/{{ item }} '
                                       'dest={{ full_deploy_dir }}/conf/{{ item }} '
                                       'mode=0644' % (
                                           os.path.join(self.topo.config_dir)),
                                       with_items=self.alert_rule,
                                       group=pattern,
                                       node=node)
                elif component in ['blackbox_exporter', 'alertmanager']:
                    self.ans.run_model('copy',
                                       'src=%s/%s '
                                       'dest={{ full_deploy_dir }}/conf/ '
                                       'mode=0644 '
                                       'backup=yes' % (
                                           os.path.join(
                                               _template_dir, 'common_config'),
                                           self.config_name[component]),
                                       group=pattern,
                                       node=node)
                elif component == 'drainer':
                    self.ans.run_model('copy',
                                       'src=%s/%s_{{ ansible_host }}_{{ port }}.%s '
                                       'dest={{ full_deploy_dir }}/conf/%s '
                                       'mode=0644 '
                                       'backup=yes' % (
                                           self.topo.cluster_configs,
                                           self.config_name[component].split('.')[0],
                                           self.config_name[component].split('.')[1],
                                           self.config_name[component]),
                                       group=pattern,
                                       node=node)
                else:
                    self.ans.run_model('copy',
                                       'src=%s/%s '
                                       'dest={{ full_deploy_dir }}/conf/ '
                                       'mode=0644 '
                                       'backup=yes' % (
                                           self.topo.cluster_configs, self.config_name[component]),
                                       group=pattern,
                                       node=node)

        # distribute script
        self.ans.run_model('template',
                           'src=%s/run_%s.sh.j2 '
                           'dest={{ full_deploy_dir }}/scripts/run_%s.sh '
                           'mode=0755 '
                           'backup=yes' % (
                               os.path.join(_template_dir, 'scripts/'), component, component),
                           group=pattern,
                           node=node)

        self.ans.run_model('template',
                           'src=%s/system.service.j2 '
                           'dest=/etc/systemd/system/{{ service_name }}.service' % (
                               os.path.join(_template_dir, 'systemd/')),
                           become=True,
                           group=pattern,
                           extra_vars=component,
                           node=node)

        self.ans.run_model('template',
                           'src=%s/{{ item }}_role.sh.j2 '
                           'dest={{ full_deploy_dir }}/scripts/{{ item }}_%s.sh '
                           'mode=0755 '
                           'backup=yes' % (os.path.join(_template_dir, 'scripts/'),
                                           component),
                           with_items=['start', 'stop'],
                           group=pattern,
                           extra_vars=component,
                           node=node)

        self.ans.run_model('systemd',
                           'daemon_reload=yes',
                           become=True,
                           group=pattern,
                           node=node)

    def deploy_tool(self):
        tools_dir = os.path.join(self.topo.user_home, 'tidb-tools')
        latest_version = ''
        tmp_version = []
        release_version = []
        all_version = os.listdir(utils.profile_path('downloads'))
        if 'latest' in all_version:
            latest_version = 'latest'
        else:
            for _version in all_version:
                if re.search(r'[a-z|A-Z]', _version.lstrip('v')):
                    tmp_version.append(_version)
                else:
                    release_version.append(_version)

            if len(release_version) == 0:
                latest_version = max(tmp_version)
            elif len(tmp_version) == 0:
                latest_version = max(release_version)
            else:
                if max(release_version) in max(tmp_version):
                    latest_version = max(release_version)
                else:
                    latest_version = max([max(release_version), max(tmp_version)])

        if not os.path.exists(tools_dir):
            os.mkdir(tools_dir)

        for tool in self.tool_name:
            binary_path = os.path.join(utils.profile_path('downloads/{}/resources'.format(latest_version)),
                                       self.tool_name[tool])
            if os.path.exists(binary_path):
                copy(binary_path, tools_dir)

    def configCheck(self, component=None, pattern=None, node=None):
        self.ans.run_model('file',
                           'name=/tmp/tidb_check '
                           'state=directory',
                           group=pattern,
                           node=node)

        self.ans.run_model('copy',
                           'src=%s/%s '
                           'dest=/tmp/tidb_check '
                           'mode=0755' % (
                               self.topo.resource_dir, self.binary_name[component]),
                           group=pattern,
                           node=node)

        self.ans.run_model('copy',
                           'src=%s/%s '
                           'dest=/tmp/tidb_check '
                           'mode=0644 '
                           'backup=yes' % (
                               self.topo.cluster_configs, self.config_name[component]),
                           group=pattern,
                           node=node)

        if component == 'pd':
            self.ans.run_model('shell',
                               'cd /tmp/tidb_check && ./pd-server -config ./pd.toml -config-check',
                               group=pattern,
                               node=node)
        elif component == 'tikv':
            self.ans.run_model('shell',
                               'cd /tmp/tidb_check && ./tikv-server --config ./tikv.toml '
                               '--config-check --pd-endpoints 127.0.0.1:2379',
                               group=pattern,
                               node=node)
        else:
            self.ans.run_model('shell',
                               'cd /tmp/tidb_check && ./tidb-server -config ./tidb.toml -config-check',
                               group=pattern,
                               node=node)

        self.ans.run_model('file',
                           'name=/tmp/tidb_check '
                           'state=absent',
                           group=pattern,
                           node=node)

    def deploy_firewall(self, component=None, pattern=None, node=None):
        if self.topo.firewall:
            self.ans.run_model('systemd',
                               'name=firewalld '
                               'enabled=yes '
                               'state=started',
                               become=True,
                               group=pattern,
                               node=node)
            if component in ['pump', 'drainer', 'grafana']:
                self.ans.run_model('firewalld',
                                   'port={{ port }}/tcp '
                                   'state=enabled '
                                   'permanent=yes '
                                   'immediate=yes',
                                   become=True,
                                   group=pattern,
                                   node=node)
            elif component == 'prometheus':
                self.ans.run_model('firewalld',
                                   'port={{ item }}/tcp '
                                   'state=enabled '
                                   'permanent=yes '
                                   'immediate=yes',
                                   become=True,
                                   with_items=[
                                       "{{ prometheus_port }}", "{{ pushgateway_port }}"],
                                   group=pattern,
                                   node=node)
            elif component in ['blackbox_exporter', 'node_exporter']:
                self.ans.run_model('firewalld',
                                   'port={{ %s_port }}/tcp '
                                   'state=enabled '
                                   'permanent=yes '
                                   'immediate=yes' % component,
                                   become=True,
                                   group=pattern,
                                   node=node)
            elif component in ['tikv', 'tidb']:
                self.ans.run_model('firewalld',
                                   'port={{ item }}/tcp '
                                   'state=enabled '
                                   'permanent=yes '
                                   'immediate=yes',
                                   become=True,
                                   with_items=["{{ port }}",
                                               "{{ status_port }}"],
                                   group=pattern,
                                   node=node)
            elif component == 'pd':
                self.ans.run_model('firewalld',
                                   'port={{ item }}/tcp '
                                   'state=enabled '
                                   'permanent=yes '
                                   'immediate=yes',
                                   become=True,
                                   with_items=[
                                       "{{ client_port }}", "{{ peer_port }}"],
                                   group=pattern,
                                   node=node)
        else:
            self.ans.run_model('systemd',
                               'name=firewalld '
                               'enabled=no '
                               'state=stopped',
                               become=True,
                               group=pattern,
                               node=node)

    def start_component(self, component=None, pattern=None, node=None):
        # start services
        self.ans.run_model('shell',
                           '{{ full_deploy_dir }}/scripts/start_%s.sh' % (
                               component),
                           group=pattern,
                           node=node)

        # status check
        if component in ['pump', 'drainer', 'grafana', 'tidb', 'tikv']:
            self.ans.run_model('wait_for',
                               'host={{ ansible_host }} '
                               'port={{ port }} '
                               'state=started '
                               'timeout=60 '
                               'msg="the %s port {{ port }} is not up"' % (
                                   component),
                               group=pattern,
                               node=node)
        elif component in ['node_exporter', 'blackbox_exporter', 'prometheus', 'pushagateway']:
            self.ans.run_model('wait_for',
                               'host={{ ansible_host }} '
                               'port={{ %s_port }} '
                               'state=started '
                               'timeout=60 '
                               'msg="the %s port {{ %s_port }} is not up"' % (
                                   component, component, component),
                               group=pattern,
                               node=node)
        elif component in ['alertmanager']:
            self.ans.run_model('wait_for',
                               'host={{ ansible_host }} '
                               'port={{ web_port }} '
                               'state=started '
                               'timeout=60 '
                               'msg="the %s port {{ web_port }} is not up"' % (
                                   component),
                               group=pattern,
                               node=node)
        elif component in ['pd']:
            self.ans.run_model('wait_for',
                               'host={{ ansible_host }} '
                               'port={{ client_port }} '
                               'state=started '
                               'timeout=60 '
                               'msg="the %s port {{ client_port }} is not up"' % (
                                   component),
                               group=pattern,
                               node=node)

            count = 0
            _fail_status = []
            while count < 12:
                self.ans.run_model('uri',
                                   'url="http://{{ ansible_host }}:{{ client_port }}/health" '
                                   'return_content=yes',
                                   group=pattern,
                                   node=node)

                for _uuid, _status in self.ans.get_model_result()['success'].iteritems():
                    if str(_status['status']) != '200' or 'true' not in json.dumps(_status['content']):
                        _fail_status.append(_uuid)

                if not _fail_status:
                    break

                utils.wait()
                count += 1

            if _fail_status:
                raise exceptions.TiOPSRuntimeError('Some pd nodes are in false state, node list: {}'
                                                   .format(','.join(_fail_status)), operation='start')

        if 'tikv_servers' not in self.topo():
            return
        if not self.topo()['tikv_servers'][0]['label']:
            return
        # get pd api
        _cluster_api = ClusterAPI(self.topo)
        _pd_label_url = '{}://{}:{}/pd/api/v1/config'.format(_cluster_api.scheme, _cluster_api.host, _cluster_api.port)
        # _tikv_label_url = '{}://{}:{}/pd/api/v1/store'
        # Add label for pd
        if component == 'pd' and not _cluster_api.pd_label():
            _pd_label = utils.format_labels(self.topo()['tikv_servers'][0]['label'])[0]
            _cluster_api.pd_label(method='POST', label=_pd_label)

        # Add label for tikv
        if component == 'tikv':
            for _tikv_node in self.topo()['tikv_servers']:
                _tikv_label = utils.format_labels(_tikv_node['label'])[1]
                _ip = _tikv_node['ip']
                _port = _tikv_node['port']
                if not _cluster_api.tikv_label(host=_ip, port=_port):
                    _cluster_api.tikv_label(method='POST', label=_tikv_label, host=_ip, port=_port)

    def stop_component(self, component=None, pattern=None, node=None):
        # stop services
        try:
            self.ans.run_model('shell',
                               '{{ full_deploy_dir }}/scripts/stop_%s.sh' % (
                                   component),
                               group=pattern,
                               node=node)
        except Exception as e:
            raise exceptions.TiOPSWarning(str(e))

        # check status
        if component in ['pump', 'drainer', 'grafana', 'tidb', 'tikv']:
            self.ans.run_model('wait_for',
                               'host={{ ansible_host }} '
                               'port={{ port }} '
                               'state=stopped '
                               'timeout=60 '
                               'msg="the %s port {{ port }} may still be up"' % (
                                   component),
                               group=pattern,
                               node=node)
        elif component in ['node_exporter', 'blackbox_exporter', 'prometheus', 'pushagateway']:
            self.ans.run_model('wait_for',
                               'host={{ ansible_host }} '
                               'port={{ %s_port }} '
                               'state=stopped '
                               'timeout=60 '
                               'msg="the %s port {{ %s_port }} may still be up"' % (
                                   component, component, component),
                               group=pattern,
                               node=node)
        elif component in ['alertmanager']:
            self.ans.run_model('wait_for',
                               'host={{ ansible_host }} '
                               'port={{ web_port }} '
                               'state=stopped '
                               'timeout=60 '
                               'msg="the %s port {{ web_port }} may still be up"' % (
                                   component),
                               group=pattern,
                               node=node)
        elif component in ['pd']:
            self.ans.run_model('wait_for',
                               'host={{ ansible_host }} '
                               'port={{ client_port }} '
                               'state=stopped '
                               'timeout=60 '
                               'msg="the %s port {{ client_port }} may still be up"' % (
                                   component),
                               group=pattern,
                               node=node)

    def destroy_component(self, component=None, pattern=None, node=None):
        try:
            self.ans.run_model('file',
                               'path=/etc/systemd/system/{{ service_name }}.service '
                               'state=absent',
                               become=True,
                               group=pattern,
                               extra_vars=component,
                               node=node)
        except Exception as e:
            raise exceptions.TiOPSWarning(str(e))

        if component != 'blackbox_exporter':
            self.ans.run_model('shell',
                               'rm -rf {{ full_deploy_dir }}',
                               become=True,
                               group=pattern,
                               node=node)

        if component in ['pd', 'tikv', 'pump', 'drainer']:
            self.ans.run_model('shell',
                               'rm -rf {{ full_data_dir }}',
                               become=True,
                               group=pattern,
                               node=node)

        if self.topo.firewall:
            self.ans.run_model('systemd',
                               'name=firewalld '
                               'enabled=yes '
                               'state=started',
                               become=True,
                               group=pattern,
                               node=node)
            if component in ['pump', 'drainer']:
                self.ans.run_model('firewalld',
                                   'port={{ port }}/tcp '
                                   'state=disabled '
                                   'permanent=yes '
                                   'immediate=yes',
                                   become=True,
                                   group=pattern,
                                   node=node)
            elif component == 'prometheus':
                self.ans.run_model('firewalld',
                                   'port={{ item }}/tcp '
                                   'state=disabled '
                                   'permanent=yes '
                                   'immediate=yes',
                                   become=True,
                                   with_items=[
                                       "{{ prometheus_port }}", "{{ pushgateway_port }}"],
                                   group=pattern,
                                   node=node)
            elif component in ['blackbox_exporter', 'node_exporter']:
                self.ans.run_model('firewalld',
                                   'port={{ %s_port }}/tcp '
                                   'state=disabled '
                                   'permanent=yes '
                                   'immediate=yes' % component,
                                   become=True,
                                   group=pattern,
                                   node=node)
            elif component in ['tikv', 'tidb']:
                self.ans.run_model('firewalld',
                                   'port={{ item }}/tcp '
                                   'state=disabled '
                                   'permanent=yes '
                                   'immediate=yes',
                                   become=True,
                                   with_items=[
                                       "{{ port }}", "{{ status_port }}"],
                                   group=pattern,
                                   node=node)
            elif component == 'pd':
                self.ans.run_model('firewalld',
                                   'port={{ item }}/tcp '
                                   'state=disabled '
                                   'permanent=yes '
                                   'immediate=yes',
                                   become=True,
                                   with_items=[
                                       "{{ client_port }}", "{{ peer_port }}"],
                                   group=pattern,
                                   node=node)

    def run_shell(self, pattern=None, node=None, sudo=False, cmd=None):
        if not cmd:
            return None

        return self.ans.run_model('shell',
                                  'cmd="{}"'.format(cmd),
                                  group=pattern,
                                  node=node,
                                  become=sudo)
