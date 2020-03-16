# coding: utf-8

import json
import logging

from abc import abstractmethod

from tiops import utils
from tiops import exceptions
from tiops.tui import TUI


class ModuleAPIBase(object):
    host = None  # hostname or IP address
    port = None  # port of control / status service
    uris = None  # dict of API URIs
    scheme = 'http'

    @abstractmethod
    def __init__(self):
        raise NotImplementedError

    @abstractmethod
    def status(self):
        raise NotImplementedError

    @abstractmethod
    def ok(self):
        raise NotImplementedError


class PDAPI(ModuleAPIBase):
    _default_pd_client_port = 2379
    uris = {
        'health': 'pd/health',
        'leader': 'pd/api/v1/leader',
    }

    def __init__(self, host=None, port=None, tls=False):
        if not host:
            raise exceptions.TiOPSArgumentError('Host must be set.')
        else:
            self.host = host
        self.port = port if port else self._default_pd_client_port
        self.scheme = 'https' if tls else 'http'

    def status(self):
        url = '{}://{}:{}/{}'.format(self.scheme,
                                     self.host, self.port, self.uris['health'])
        resp, code = utils.read_url(url, timeout=2)
        if not code or not (code >= 200 and code < 400):
            raise exceptions.TiOPSRequestError(url, code, resp)
        return resp

    def ok(self):
        try:
            resp = self.status()
        except exceptions.TiOPSRequestError:
            return False
        try:
            _status = json.loads(resp)
            _health = True
            for _pd in _status:
                _health = _health and _pd['health']
            return _health
        except KeyError:
            return False

    def leader(self):
        url = '{}://{}:{}/{}'.format(self.scheme,
                                     self.host, self.port, self.uris['leader'])
        resp, code = utils.read_url(url, timeout=2)
        if not code or not (code >= 200 and code < 400):
            raise exceptions.TiOPSRequestError(url, code, resp)
        _data = json.loads(resp)
        return _data['name']


class TiDBAPI(ModuleAPIBase):
    _default_tidb_status_port = 10080
    uris = {
        'status': 'status',
    }

    def __init__(self, host=None, port=None):
        if not host:
            raise exceptions.TiOPSArgumentError('Host must be set.')
        else:
            self.host = host
        self.port = port if port else self._default_tidb_status_port

    def status(self):
        url = '{}://{}:{}/{}'.format(self.scheme,
                                     self.host, self.port, self.uris['status'])
        resp, code = utils.read_url(url, timeout=2)
        if not code or not (code >= 200 and code < 400):
            raise exceptions.TiOPSRequestError(url, code, resp)
        return resp

    def ok(self):
        try:
            resp = self.status()
        except exceptions.TiOPSRequestError:
            return False
        if not resp:
            return False
        return True


class TiKVAPI(ModuleAPIBase):
    _default_tikv_status_port = 20180
    uris = {
        'status': 'status',
    }

    def __init__(self, host=None, port=None):
        if not host:
            raise exceptions.TiOPSArgumentError('Host must be set.')
        else:
            self.host = host
        self.port = port if port else self._default_tikv_status_port

    def status(self):
        url = '{}://{}:{}/{}'.format(self.scheme,
                                     self.host, self.port, self.uris['status'])
        resp, code = utils.read_url(url, timeout=2)
        if not code or not (code >= 200 and code < 400):
            raise exceptions.TiOPSRequestError(url, code, resp)
        return resp

    def ok(self):
        try:
            self.status()
        except exceptions.TiOPSRequestError:
            return False
        return True


class ClusterAPI(PDAPI):
    uris = {
        'health': 'pd/health',
        'members': 'pd/api/v1/members',
        'store': 'pd/api/v1/store',
        'stores': 'pd/api/v1/stores',
        'tombstone': '/pd/api/v1/stores?state=2',
        'config': 'pd/api/v1/config',
        'cluster': 'pd/api/v1/cluster',
        'schedulers': 'pd/api/v1/schedulers',
        'leader': 'pd/api/v1/leader',
        'transfer': 'pd/api/v1/leader/transfer',
    }

    def __init__(self, topology=None):
        if not topology:
            raise exceptions.TiOPSAttributeError(
                'Topology is empty, can not get cluster info.')
        else:
            self.config = topology()

        try:
            pd_node = self.config['pd_servers'][0]
        except KeyError:
            raise exceptions.TiOPSArgumentError(
                'Topolgoy does not contain any PD node.')
        try:
            tls = topology.enable_tls
        except AttributeError:
            tls = False

        super(ClusterAPI, self).__init__(
            pd_node['ip'], pd_node['client_port'], tls)

    def __url(self, uri=None):
        if not uri:
            uri = self.uris['health']
        return '{}://{}:{}/{}'.format(self.scheme, self.host, self.port, uri)

    def __req(self, url=None, method='GET'):
        if not url:
            raise exceptions.TiOPSArgumentError(
                'No URL specified to request.')
        resp, code = utils.request_url(url, method=method)
        if not code or not (code >= 200 and code < 400):
            raise exceptions.TiOPSRequestError(url, code, resp)
        return resp

    def status(self):
        _url = self.__url(self.uris['health'])
        _data = self.__req(_url)
        return json.loads(_data)

    def pd_leader(self):
        _url = self.__url(self.uris['leader'])
        while True:
            try:
                _info = json.loads(self.__req(_url))
                # this will trow KeyError or AttributeError if it's not leader
                uuid = _info['name']
                break
            except:
                continue
            utils.wait(5)

        return uuid

    # transfer leader until leader is not current node
    def evict_pd_leader(self, uuid=None):
        if not uuid:
            raise exceptions.TiOPSArgumentError(
                'UUID is not set, can not transfer leader from None.')
        _members = self.__req(self.__url(self.uris['members']))
        if len(json.loads(_members)['members']) == 1:
            # force continue when there is only one PD
            logging.warning('Only 1 PD node, skip leader transfer.')
            return
        while True:
            curr_leader = self.pd_leader()
            if uuid != curr_leader:
                return
            _url = '{}/resign'.format(self.__url(self.uris['leader']))
            try:
                utils.request_url(_url, method='POST')
            except Exception as e:
                logging.warning('Failed to transfer PD leader: {}'.format(e))
                pass
            utils.wait(5)

    def tikv_stores(self):
        try:
            _url = self.__url(self.uris['stores'])
            _tikv_stores = self.__req(_url)
        except:
            return None
        return json.loads(_tikv_stores)

    def tikv_tombstone(self):
        try:
            _url = self.__url(self.uris['tombstone'])
            _tikv_tombstones = self.__req(_url)
        except:
            return None
        return json.loads(_tikv_tombstones)

    def current_store_info(self, host=None, port=None):
        _stores = self.tikv_stores()
        if not _stores:
            return None, None
        _current_address = '{}:{}'.format(host, port)
        try:
            _store_info = [x for x in _stores['stores']
                           if x['store']['address'] == _current_address][0]
            _store_id = _store_info['store']['id']
        except IndexError:
            return None, None
        try:
            _store_leader_count = _store_info['status']['leader_count']
        except KeyError:
            _store_leader_count = None
        return _store_id, _store_leader_count

    def evict_store_leaders(self, host=None, port=None):
        _store_id = self.current_store_info(host=host, port=port)[0]
        if not _store_id:
            return
        # _current_schedulers = self.display_scheduler()
        # if 'evict-leader-scheduler-{}'.format(_store_id) not in _current_schedulers:
        #     _headers = {'Content-Type': 'application/json'}
        #     _url = self.__url(self.uris['schedulers'])
        #     _scheduler = {'name': 'evict-leader-scheduler',
        #                   'store_id': _store_id}
        #     utils.request_url(
        #         _url, data=json.dumps(_scheduler), method='POST', headers=_headers)
        _headers = {'Content-Type': 'application/json'}
        _url = self.__url(self.uris['schedulers'])
        _scheduler = {'name': 'evict-leader-scheduler',
                      'store_id': _store_id}
        utils.request_url(
            _url, data=json.dumps(_scheduler), method='POST', headers=_headers)
        while True:
            _store_leader_count = self.current_store_info(
                host=host, port=port)[1]
            if not _store_leader_count:
                return
            utils.wait(5)

    def remove_evict(self, host, port):
        _store_id, _store_leader_count = self.current_store_info(
            host=host, port=port)
        # _current_schedulers = self.display_scheduler()
        # if 'evict-leader-scheduler-{}'.format(_store_id) in _current_schedulers:
        #     _remove_evict_url = '{}/{}'.format(self.__url(
        #         self.uris['schedulers']), 'evict-leader-scheduler-{}'.format(_store_id))
        #     utils.request_url(_remove_evict_url, method='DELETE')
        _remove_evict_url = '{}/{}'.format(self.__url(
            self.uris['schedulers']), 'evict-leader-scheduler-{}'.format(_store_id))
        utils.request_url(_remove_evict_url, method='DELETE')

    def display_scheduler(self):
        _url = self.__url(self.uris['schedulers'])
        while True:
            _schedulers = self.__req(_url)
            _data = json.loads(_schedulers)
            if _data and isinstance(_data, list):
                return _data
            utils.wait(5)

    def del_pd(self, uuid=None):
        if not uuid:
            raise exceptions.TiOPSArgumentError(
                'UUID is not set, can not delete PD node.')
        _url = '{}/name/{}'.format(self.__url(self.uris['members']), uuid)
        self.__req(_url, method='DELETE')

    def del_store(self, id=None):
        if not id:
            raise exceptions.TiOPSArgumentError(
                'Store ID is not set, can not delete store.')
        _url = '{}/{}'.format(self.__url(self.uris['store']), id)
        self.__req(_url, method='DELETE')

    def pd_label(self, method='GET', label=None):
        _url = self.__url(self.uris['config'])
        if method == 'GET':
            _config = self.__req(_url)
            _data = json.loads(_config)
            _label = _data['replication']['location-labels']
            return _label
        elif method == 'POST':
            _headers = {'Content-Type': 'application/json'}
            _label = {'location-labels': label}
            while True:
                _result = utils.request_url(_url, data=json.dumps(_label), method='POST', headers=_headers)
                if _result[1] == 200:
                    break

    def tikv_label(self, method='GET', label=None, host=None, port=None):
        _store_id = self.current_store_info(host=host, port=port)[0]
        if method == 'GET':
            _url = '{}/{}'.format(self.__url(self.uris['store']), _store_id)
            _store_info = self.__req(_url)
            _data = json.loads(_store_info)
            if 'labels' not in _data['store']:
                return False
            elif _data['store']['labels']:
                return True
            else:
                return False
        elif method == 'POST':
            _url = '{}/{}/label'.format(self.__url(self.uris['store']), _store_id)
            _headers = {'Content-Type': 'application/json'}
            while True:
                _result = utils.request_url(_url, data=label, method='POST', headers=_headers)
                if _result[1] == 200:
                    break


class BinlogAPI(object):
    uris = {
        'pump_status': '/status',
        'drainer_status': '/drainers',
        'action': '/state'
    }

    def __init__(self, topology):
        if not topology:
            raise exceptions.TiOPSAttributeError(
                'Topology is empty, can not get cluster info.')
        self.config = topology()

        try:
            tls = topology.enable_tls
        except AttributeError:
            tls = False

        self.scheme = 'https' if tls else 'http'
        self.pump_ip, self.pump_port, self.pump_status, self.drainer_status = self._avialable_node()

    def _avialable_node(self):
        _num = len(self.config['pump_servers'])
        if not _num:
            return None, None, None, None
        _count = 0
        for _pump_node in self.config['pump_servers']:
            _count += 1
            _ip = _pump_node['ip']
            _port = _pump_node['port']
            _pump_content, _pump_code = utils.request_url(
                '{}://{}:{}{}'.format(self.scheme, _ip, _port, self.uris['pump_status']))
            if _pump_code != 200:
                if _count == _num:
                    return None, None, None, None
                continue
            _drainer_content, _drainer_code = utils.request_url(
                '{}://{}:{}{}'.format(self.scheme, _ip, _port, self.uris['drainer_status']))
            return _ip, _port, json.loads(_pump_content), json.loads(_drainer_content)

    def delete_pump(self, node_id=None):
        _pump_info = [x for x in self.config['pump_servers'] if x['uuid'] == node_id][0]
        _url = '{}://{}:{}{}/{}/close'.format(
            self.scheme, _pump_info['ip'], _pump_info['port'], self.uris['action'], node_id)
        _content, _code = utils.request_url(url=_url, method='PUT')
        if _code == 200 and 'success' in json.loads(_content)['message']:
            return True
        return False

    def delete_drainer(self, node_id=None):
        _drainer_info = [x for x in self.config['drainer_servers'] if x['uuid'] == node_id][0]
        _url = '{}://{}:{}{}/{}/close'.format(
            self.scheme, _drainer_info['ip'], _drainer_info['port'], self.uris['action'], node_id)
        _content, _code = utils.request_url(url=_url, method='PUT')
        if _code == 200 and 'success' in json.loads(_content)['message']:
            return True
        return False
