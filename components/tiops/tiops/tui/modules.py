# coding: utf-8

import json
import logging
import os

from tiops import modules as tcapi
from tiops import utils

from .terminal import TUIDisplay


class TUIModuleBase(TUIDisplay):
    def __init__(self, inventory=None, module=None):
        super(TUIModuleBase, self).__init__(inventory)

        try:
            self.hosts = self.inventory.servers(module)
        except AttributeError:
            pass


class TUIModule(TUIModuleBase):
    def __init__(self, inventory=None, module=None, args=None, status=False):
        super(TUIModule, self).__init__(inventory, module)
        self.args = args
        self.status = status

    def display(self, listing=False):
        if listing:
            self._list_clusters()
        else:
            self._display_cluster()

    def _list_clusters(self):
        profile_dir = utils.profile_path('clusters')
        _srv_list = [['Cluster', 'Version']]
        for _file in utils.list_dir(profile_dir):
            try:
                for x in ['downloads', 'host_vars', 'tiops.log']:
                    if x in _file:
                        raise RuntimeError
            except RuntimeError:
                continue
            if not os.path.isdir(_file):
                continue

            _cluster_name = os.path.split(_file)[1]
            try:
                _meta = utils.read_yaml(os.path.join(
                    profile_dir, _file, 'meta.yaml'))
            except EnvironmentError as e:
                import errno
                # only pass when the error is file not found
                if e.errno != errno.ENOENT:
                    raise
                self.term.warn(
                    'Metadata file of cluster {} not found, did the deploy process finished?'.format(_cluster_name))
                # skip this cluster
                continue
            try:
                _version = _meta['tidb_version']
            except KeyError:
                _version = '-'
            _srv_list.append([_cluster_name, _version])

        self.term.info('Available TiDB clusters:')
        for row in self.format_columns(_srv_list):
            self.term.normal(row)

    def _display_cluster(self):
        try:
            _status = self.args.show_status
        except AttributeError:
            _status = self.status

        cluster_home = utils.profile_path(
            'clusters/{}'.format(self.args.cluster_name))
        _profile_path = utils.profile_path(cluster_home, 'meta.yaml')
        if os.path.exists(cluster_home) and os.path.exists(_profile_path):
            cluster_info = utils.read_yaml(_profile_path)
            info = 'TiDB cluster {}, version {}\nNode list:'.format(
                self.args.cluster_name, cluster_info['tidb_version'])
            self.term.info(info)

        display_info = self._format_cluster(_status)
        for section in display_info:
            for row in self.format_columns(section):
                self.term.normal(row)

    def _format_cluster(self, show_status=False):
        try:
            _role_filter = self.args.role.lower()
        except AttributeError:
            _role_filter = None
        try:
            _node_filter = self.args.node_id.lower()
        except AttributeError:
            _node_filter = None
        try:
            _ip_filter = self.args.ip_addr
        except AttributeError:
            _ip_filter = None

        result = []

        if show_status:
            _title = ['ID', 'Role', 'Host', 'Ports',
                      'Status', 'Data Dir', 'Deploy Dir']
        else:
            _title = ['ID', 'Role', 'Host', 'Ports', 'Data Dir', 'Deploy Dir']
        srv_list = [_title]
        for srv in self.hosts:
            _host = srv['ip']
            _role = srv['role']
            _uuid = srv['uuid']

            # apply filters
            if _role_filter and _role.lower() not in _role_filter:
                continue
            if _node_filter and _uuid.lower() not in _node_filter:
                continue
            if _ip_filter and _host not in _ip_filter:
                continue

            try:
                _port = '{}/{}'.format(srv['client_port'], srv['peer_port'])
            except KeyError:
                try:
                    _port = '{}/{}'.format(srv['port'], srv['status_port'])
                except KeyError:
                    try:
                        if srv.has_key('prometheus_port'):
                            _port = '{}/{}'.format(srv['prometheus_port'],
                                                   srv['pushgateway_port'])
                        elif srv.has_key('node_exporter_port'):
                            _port = '{}/{}'.format(srv['node_exporter_port'],
                                                   srv['blackbox_exporter_port'])
                        elif srv.has_key('web_port'):
                            _port = '{}/{}'.format(srv['web_port'],
                                                   srv['cluster_port'])
                        else:
                            _port = '{}'.format(srv['port'])
                    except:
                        pass

            # query node status
            if show_status:
                _status = self.__get_status(srv)

            _deploy = srv['full_deploy_dir']
            try:
                _data = srv['full_data_dir']
                if srv['role'].lower() == 'tidb':
                    _data = '-'
            except KeyError:
                _data = "-"

            if show_status:
                srv_list.append(
                    [_uuid, _role, _host, _port, _status, _data, _deploy])
            else:
                srv_list.append([_uuid, _role, _host, _port, _data, _deploy])

        result.append(srv_list)
        return result

    def __get_status(self, srv=None):
        _status = '-'
        if not srv:
            return _status

        _host = srv['ip']
        _role = srv['role']
        _uuid = srv['uuid']
        if 'pd' == _role.lower():
            _port = srv['client_port']
            _api = tcapi.PDAPI(_host, _port)
            try:
                _resp = _api.status()
                for _pd in json.loads(_resp):
                    if _pd['name'] != _uuid:
                        continue
                    if _pd['health']:
                        _status = 'Health'
                    else:
                        _status = 'Unhealth'
            except:
                return 'Down'
            try:
                _leader = _api.leader()
                if _leader == _uuid:
                    _status = '{}|L'.format(_status)
            except:
                return _status
        elif 'tidb' == _role.lower():
            _port = srv['status_port']
            _api = tcapi.TiDBAPI(_host, _port)
            if _api.ok():
                _status = 'Up'
            else:
                _status = 'Down'
        elif 'tikv' == _role.lower():
            _port = srv['port']
            _api = tcapi.ClusterAPI(self.inventory)
            if _api.tikv_stores():
                for store in _api.tikv_stores()['stores']:
                    if '{}:{}'.format(_host, _port) == store['store']['address']:
                        return store['store']['state_name']
            if _api.tikv_tombstone():
                for store in _api.tikv_tombstone()['stores']:
                    if '{}:{}'.format(_host, _port) == store['store']['address']:
                        return store['store']['state_name']
            if _status == '-':
                return 'Down'
        elif 'pump' == _role.lower():
            _uuid = srv['uuid']
            _api = tcapi.BinlogAPI(self.inventory)
            if _api.pump_status:
                for _pump_id, _pump_info in _api.pump_status['status'].iteritems():
                    if _pump_info['nodeId'] == _uuid:
                        _status = _pump_info['state']
                        break
            else:
                _status = 'Down'

        elif 'drainer' == _role.lower():
            _uuid = srv['uuid']
            _api = tcapi.BinlogAPI(self.inventory)
            if _api.drainer_status:
                for _drainer in _api.drainer_status:
                    if _uuid == _drainer['nodeId']:
                        _status = _drainer['state']
                        break
            else:
                _status = 'Down'

        return _status
