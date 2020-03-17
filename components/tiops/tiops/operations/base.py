# coding: utf-8

import os

from abc import abstractmethod

from tiops import exceptions
from tiops import utils
from tiops.ansibleapi import ansibleapi
from tiops.modules.api import ClusterAPI, BinlogAPI
from tiops.operations.action import Action
from tiops.tui import term


class OperationBase(object):
    def __init__(self, args=None, topology=None, demo=False, action=None):
        try:
            self._lock_profile()
        except NotImplementedError:
            pass
        except exceptions.TiOPSException:  # TODO: add more specific handlers
            raise

        self.topology = topology
        self._args = args

        if not demo and os.path.exists(self.topology.topology_file) and not action:
            term.warn('Check TiDB cluster {} status, it may take a few minutes.'.format(
                self.topology.cluster_name))
            self.check_tombstone()

        self.ans = ansibleapi.ANSRunner(
            user=self.topology.user, topology=self.topology(), tiargs=self._args)

    def __del__(self):
        try:
            self._unlock_profile()
        except NotImplementedError:
            pass

    def __call__(self):
        self.do()

    def _lock_profile(self):
        raise NotImplementedError

    def _unlock_profile(self):
        raise NotImplementedError

    @abstractmethod
    def _prepare(self, component=None, pattern=None, node=None, role=None):
        # implemente any preparations before the actual operation here
        raise NotImplementedError

    @abstractmethod
    def _process(self, component=None, pattern=None, node=None, role=None):
        # implemente any process needed to take the operation here
        raise NotImplementedError

    @abstractmethod
    def _post(self, component=None, pattern=None, node=None, role=None):
        # implemente any post cleanups, promts or restult handlings after the operation here
        raise NotImplementedError

    def do(self, component=None, pattern=None, node=None, role=None, ans=None):
        try:
            self._prepare(component, pattern, node, role)
        except NotImplementedError:
            pass
        try:
            self._process(component, pattern, node, role)
        except NotImplementedError:
            pass
        try:
            self._post(component, pattern, node, role)
        except NotImplementedError:
            pass

    def check_exist(self, service, config=None):
        if not config:
            config = self.topology()
        for item in service.items():
            component, pattern = item
        if not pattern in config.keys() or not config[pattern]:
            return False, False
        return component, pattern

    def check_tombstone(self, topology=None, args=None):
        if not topology:
            topology = self.topology
        if not args:
            args = self._args
        _remove_uuid = []
        _cluster = ClusterAPI(topology)
        _binlog = BinlogAPI(topology)

        if _cluster.tikv_stores() and _cluster.tikv_tombstone():
            # get tombstone tikv node
            for _node in topology()['tikv_servers']:
                _tombstone = False
                if not _node['offline']:
                    continue

                # online tikv node list
                _online_list = [x['store']['address']
                                for x in _cluster.tikv_stores()['stores']]
                # tombstone status tikv list
                _tombstone_list = [x['store']['address']
                                   for x in _cluster.tikv_tombstone()['stores']]

                _address = '{}:{}'.format(_node['ip'], _node['port'])

                # if node is online, skip it
                if _address in _online_list:
                    continue
                # if node is tombstone, will delete it from topology
                elif _address in _tombstone_list:
                    _remove_uuid.append(_node['uuid'])

        if _binlog.pump_status:
            # get tombstone pump node
            for _node in topology()['pump_servers']:
                _tombstone = False
                if not _node['offline']:
                    continue

                _online_list = [x['nodeId'] for x in _binlog.pump_status['status'].itervalues() if
                                x['state'] != 'offline']
                _tombstone_list = [x['nodeId'] for x in _binlog.pump_status['status'].itervalues() if
                                   x['state'] == 'offline']

                if _node['uuid'] in _online_list:
                    continue
                elif _node['uuid'] in _tombstone_list:
                    _remove_uuid.append(_node['uuid'])

            for _node in topology()['drainer_servers']:
                _tombstone = False
                if not _node['offline']:
                    continue

                _online_list = [x['nodeId'] for x in _binlog.drainer_status if
                                x['state'] != 'offline']
                _tombstone_list = [x['nodeId'] for x in _binlog.drainer_status if
                                   x['state'] == 'offline']

                if _node['uuid'] in _online_list:
                    continue
                elif _node['uuid'] in _tombstone_list:
                    _remove_uuid.append(_node['uuid'])

        if not _remove_uuid:
            return

        _new_topo, _diff = topology.remove(
            ','.join(_remove_uuid), delete=True)
        ans = ansibleapi.ANSRunner(
            user=topology.user, topology=_diff, tiargs=args)
        act = Action(ans=ans, topo=topology)
        for service in [{'drainer': 'drainer_servers'}, {'pump': 'pump_servers'}, {'tikv': 'tikv_servers'}]:
            component, pattern = self.check_exist(service, _diff)
            if not component and not pattern:
                continue
            act.stop_component(component=component, pattern=pattern)
            act.destroy_component(component=component, pattern=pattern)

        topology.replace(_new_topo)
