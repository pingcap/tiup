# coding: utf-8

import logging
import os
import copy

from abc import abstractmethod
from argparse import Namespace
from collections import Counter

from tiops import utils
from tiops import exceptions
from tiops.tui import term


# the base config of topology config
class TopologyBase(object):
    cluster_name = 'tidb-cluster'
    enable_tls = False
    ssh_port = 22
    server = {}
    version = None
    firewall = False
    _default_user = 'tidb'
    _default_version = 'v3.0.9'

    def __call__(self):
        raise NotImplementedError

    def __init__(self, args=None):
        if not args:
            raise exceptions.TiOPSArgumentError("Argument must be set.")

        try:
            if args.cluster_name and len(args.cluster_name) > 0:
                self.cluster_name = args.cluster_name
        except AttributeError:
            logging.info('Cluster name not set, using {} as default.'.format(
                self.cluster_name))
            pass

        try:
            self.ntp_server = args.ntp_server
        except AttributeError:
            pass

        try:
            self.forks = args.forks
        except AttributeError:
            pass

        try:
            self.enable_check_cpu = args.enable_check_cpu
        except AttributeError:
            pass

        try:
            self.enable_check_mem = args.enable_check_mem
        except AttributeError:
            pass

        try:
            self.enable_check_disk = args.enable_check_disk
        except AttributeError:
            pass

        try:
            self.enable_check_iops = args.enable_check_iops
        except AttributeError:
            pass

        try:
            self.enable_check_all = args.enable_check_all
        except AttributeError:
            pass

        self.cluster_dir = utils.profile_path(
            'clusters/{}'.format(self.cluster_name))
        # try to read metadata from profile directory
        self.meta_file = utils.profile_path(self.cluster_dir, 'meta.yaml')
        try:
            _meta = utils.read_yaml(self.meta_file)
        except EnvironmentError as e:
            import errno
            # only pass when the error is file not found
            if e.errno != errno.ENOENT:
                raise
            _meta = None
            pass

        # try to read and set metadata
        try:
            _meta_version = _meta['tidb_version'] if _meta else None
        except KeyError:
            _meta_version = None
        try:
            _meta_user = _meta['user'] if _meta else None
        except KeyError:
            _meta_user = None

        try:
            self.enable_tls = _meta['enable_tls'] if _meta else args.enable_tls
        except AttributeError:
            self.enable_tls = False

        try:
            self.firewall = _meta['firewall'] if _meta else args.firewall
        except AttributeError:
            self.firewall = False

        # There is already a default version in parameter parse. If the format is wrong,
        # it is a user-specified problem. In this case, the error should be returned to the
        # user to specify the correct format, instead of installing a default version that is
        # different from what the user expects.
        if _meta_version:
            self.version = _meta_version
        else:
            try:
                self.version = args.tidb_version.lstrip('v')
            except AttributeError:
                self.version = None

        # Same as above
        try:
            if _meta_user:
                self.user = _meta_user
            else:
                self.user = args.deploy_user
        except AttributeError:
            logging.info('Deploy user is not set, using {} as default.'.format(
                self._default_user))
            self.user = self._default_user

        self._base_deploy_dir = '/home/{}/deploy'.format(self.user)
        try:
            self._data_dir = args.data_dir
        except AttributeError:
            # TODO: use true $HOME of deploy machine, or ensure the target user has permission to this path
            self._data_dir = '/home/{}/data'.format(self.user)

        self.othvars()

    def othvars(self):
        # machine cpu limit
        self.tidb_min_cpu = 8
        self.tikv_min_cpu = 8
        self.pd_min_cpu = 4
        self.monitor_min_cpu = 4

        # machine mem limit
        self.tidb_min_ram = 16000
        self.tikv_min_ram = 16000
        self.pd_min_ram = 8000
        self.monitor_min_ram = 8000

        # machine disk space limit
        self.tikv_min_disk = 500000000000
        self.pd_min_disk = 200000000000
        self.monitor_min_disk = 500000000000

        self.randread_iops = 40000
        self.mix_read_iops = 10000
        self.mix_write_iops = 10000
        self.mix_read_lat = 250000
        self.mix_write_lat = 30000

        self.user_home = utils.home()
        self.ticache_dir = utils.profile_dir()
        self.tidown_dir = utils.profile_path('downloads')
        self.titemplate_dir = '{}/tiops/templates'.format(os.environ['TIUP_COMPONENT_INSTALL_DIR'])
        try:
            self.cache_template_dir = utils.profile_path(
                self.cluster_dir, 'templates')
        except Exception as e:
            pass
        self.ticheck_dir = '{}/tiops/check/'.format(
            os.environ['TIUP_COMPONENT_INSTALL_DIR'])
        self.ticluster_dir = utils.profile_path('clusters')
        self.cluster_configs = utils.profile_path(self.cluster_dir, 'configs')
        self.tiversion_dir = utils.profile_path(
            self.tidown_dir, 'v{}'.format(self.version))

        # cache dir path
        if self.version:
            self.resource_dir = utils.profile_path(
                'downloads', 'v{}/resources'.format(self.version))
            self.dashboard_dir = utils.profile_path(
                'downloads', 'v{}/dashboards'.format(self.version))
            self.package_dir = utils.profile_path(
                'downloads', 'v{}/packages'.format(self.version))
            self.config_dir = utils.profile_path(
                'downloads', 'v{}/configs'.format(self.version))

        # service and group list
        self.service_group = [{'pd': 'pd_servers'},
                              {'tikv': 'tikv_servers'},
                              {'pump': 'pump_servers'},
                              {'tidb': 'tidb_servers'},
                              {'drainer': 'drainer_servers'},
                              {'prometheus': 'monitoring_server'},
                              {'node_exporter': 'monitored_servers'},
                              {'blackbox_exporter': 'monitored_servers'},
                              {'grafana': 'grafana_server'},
                              {'alertmanager': 'alertmanager_server'}]

    # create the class object directly from config
    @classmethod
    def new(cls, cluster_name=None, user=None, data_dir=None, server=None, ssh_port=None):
        _args = Namespace(cluster_name=cluster_name,
                          user=user,
                          data_dir=data_dir)
        _cls = cls(_args)
        if server:
            _cls.server = server
        if ssh_port:
            _cls.ssh_port = ssh_port
        return _cls

    # format config to a server object
    @abstractmethod
    def format(self, config, cluster_name=None):
        raise NotImplementedError

    # return port(s) of the server
    @abstractmethod
    def port(self, new_port=None):
        raise NotImplementedError

    # validate ports of the server for conflictions
    @abstractmethod
    def _validate_ports(self):
        raise NotImplementedError

    # return the IP address of server
    def ip(self):
        try:
            return self.server['ip']
        except (AttributeError, KeyError):
            # it's very unlikely to occur, the 'ip' section should always be set
            return '0.0.0.0'

    # merge partial config to current one
    def merge(self, srv):
        for k, v in srv.items():
            self.server[k] = v

    # build the full data_dir path for a node
    def _full_data_dir(self, role, uuid, base=None):
        if base:
            return os.path.join(base, self.cluster_name,
                                '{}-{}'.format(role, uuid))
        return os.path.join(self._data_dir, self.cluster_name,
                            '{}-{}'.format(role, uuid))

    # build the full deploy_dir path for a node
    def _full_deploy_dir(self, role, uuid, base=None):
        if base:
            return os.path.join(base, self.cluster_name,
                                '{}-{}'.format(role, uuid))
        return os.path.join(self._base_deploy_dir, self.cluster_name,
                            '{}-{}'.format(role, uuid))


# topology config for a single PD server
class TopologyPDServer(TopologyBase):
    client_port = 2379
    peer_port = 2380
    deploy_dir = None
    data_dir = None
    #_global = {}

    def __call__(self):
        return self.server

    def __init__(self, args=None, global_vars=None):
        super(TopologyPDServer, self).__init__(args)

        # apply global variables
        _glv = global_vars if global_vars else {}
        for k, v in _glv.items():
            if k == 'client_port':
                self.client_port = int(v)
            elif k == 'peer_port':
                self.peer_port = int(v)
            elif k == 'deploy_dir':
                self.deploy_dir = v
            elif k == 'data_dir':
                self.data_dir = v
            elif k == 'ssh_port':
                self.ssh_port = int(v)
            # else: # remaining vars
            #    self._global[k] = v

    def format(self, srv, cluster_name=None):
        try:
            ip = srv['ip']
        except KeyError:
            term.fatal(
                'Config format error in "pd_servers": ip can not be empty.')
            raise
        try:
            client_port = srv['client_port']
        except KeyError:
            client_port = self.client_port
        try:
            peer_port = srv['peer_port']
        except KeyError:
            peer_port = self.peer_port
        try:
            uuid = srv['uuid']
        except KeyError:
            uuid = utils.gen_uuid(
                '{}:{}:{}'.format(ip, client_port, peer_port))
        try:
            data_dir = srv['full_data_dir']
        except KeyError:
            try:
                data_dir = self._full_data_dir('pd', uuid, srv['data_dir'])
            except KeyError:
                if self.data_dir:
                    data_dir = self._full_data_dir('pd', uuid, self.data_dir)
                else:
                    data_dir = self._full_data_dir('pd', uuid)

        try:
            deploy_dir = srv['full_deploy_dir']
        except KeyError:
            try:
                deploy_dir = self._full_deploy_dir(
                    'pd', uuid, srv['deploy_dir'])
            except KeyError:
                if self.deploy_dir:
                    deploy_dir = self._full_deploy_dir(
                        'pd', uuid, self.deploy_dir)
                else:
                    deploy_dir = self._full_deploy_dir('pd', uuid)
        try:
            ssh_port = srv['ssh_port']
        except KeyError:
            ssh_port = self.ssh_port
        try:
            numa_node = srv['numa_node']
        except KeyError:
            numa_node = False
        self.server = {
            'ip': ip,
            'client_port': int(client_port),
            'peer_port': int(peer_port),
            'full_data_dir': data_dir,
            'full_deploy_dir': deploy_dir,
            'uuid': uuid,
            'ssh_port': int(ssh_port),
            'numa_node': numa_node
        }
        return self

    def port(self, new_port=None):
        if new_port:
            if new_port[0] == new_port[1]:
                raise exceptions.TiOPSConfigError(new_port,
                                                  'Same port number {} used for multiple purpose'.format(new_port[0]))
            self.server['client_port'], self.server['peer_port'] = new_port[0], new_port[1]
        return self.server['client_port'], self.server['peer_port']

    def _validate_ports(self):
        return self.server['client_port'] != self.server['peer_port']


# topology config for a single TiDB server
class TopologyTiDBServer(TopologyBase):
    tidb_port = 4000
    status_port = 10080
    deploy_dir = None

    def __call__(self):
        return self.server

    def __init__(self, args=None, global_vars=None):
        super(TopologyTiDBServer, self).__init__(args)

        # apply global variables
        _glv = global_vars if global_vars else {}
        for k, v in _glv.items():
            if k == 'port':
                self.tidb_port = int(v)
            elif k == 'status_port':
                self.status_port = int(v)
            elif k == 'deploy_dir':
                self.deploy_dir = v
            elif k == 'ssh_port':
                self.ssh_port = int(v)

    def format(self, srv, cluster_name=None):
        try:
            ip = srv['ip']
        except KeyError:
            term.fatal(
                'Config format error in "tidb_servers": ip can not be empty.')
            raise
        try:
            port = srv['port']
        except KeyError:
            port = self.tidb_port
        try:
            status_port = srv['status_port']
        except KeyError:
            status_port = self.status_port
        try:
            uuid = srv['uuid']
        except KeyError:
            uuid = utils.gen_uuid('{}:{}:{}'.format(ip, port, status_port))
        try:
            deploy_dir = srv['full_deploy_dir']
        except KeyError:
            try:
                deploy_dir = self._full_deploy_dir(
                    'tidb', uuid, srv['deploy_dir'])
            except KeyError:
                if self.deploy_dir:
                    deploy_dir = self._full_deploy_dir(
                        'tidb', uuid, self.deploy_dir)
                else:
                    deploy_dir = self._full_deploy_dir('tidb', uuid)
        try:
            ssh_port = srv['ssh_port']
        except KeyError:
            ssh_port = self.ssh_port
        try:
            numa_node = srv['numa_node']
        except KeyError:
            numa_node = False
        self.server = {
            'ip': ip,
            'port': int(port),
            'status_port': int(status_port),
            'full_deploy_dir': deploy_dir,
            'uuid': uuid,
            'ssh_port': int(ssh_port),
            'numa_node': numa_node
        }
        return self

    def port(self, new_port=None):
        if new_port:
            if new_port[0] == new_port[1]:
                raise exceptions.TiOPSConfigError(new_port,
                                                  'Same port number {} used for multiple purpose'.format(new_port[0]))
            self.server['port'], self.server['status_port'] = new_port[0], new_port[1]
        return self.server['port'], self.server['status_port']

    def _validate_ports(self):
        return self.server['port'] != self.server['status_port']


# topology config for a single TiKV server
class TopologyTiKVServer(TopologyBase):
    tikv_port = 20160
    status_port = 20180
    deploy_dir = None
    data_dir = None

    def __call__(self):
        return self.server

    def __init__(self, args=None, global_vars=None):
        super(TopologyTiKVServer, self).__init__(args)

        # apply global variables
        _glv = global_vars if global_vars else {}
        for k, v in _glv.items():
            if k == 'port':
                self.tikv_port = int(v)
            elif k == 'status_port':
                self.status_port = int(v)
            elif k == 'deploy_dir':
                self.deploy_dir = v
            elif k == 'data_dir':
                self.data_dir = v
            elif k == 'ssh_port':
                self.ssh_port = int(v)

    def format(self, srv, cluster_name=None):
        try:
            ip = srv['ip']
        except KeyError:
            term.fatal(
                'Config format error in "tikv_servers": ip can not be empty.')
            raise
        try:
            port = srv['port']
        except KeyError:
            port = self.tikv_port
        try:
            status_port = srv['status_port']
        except KeyError:
            status_port = self.status_port
        try:
            uuid = srv['uuid']
        except KeyError:
            uuid = utils.gen_uuid('{}:{}:{}'.format(ip, port, status_port))
        try:
            data_dir = srv['full_data_dir']
        except KeyError:
            try:
                data_dir = self._full_data_dir(
                    'tikv', uuid, srv['data_dir'])
            except KeyError:
                if self.data_dir:
                    data_dir = self._full_data_dir('tikv', uuid, self.data_dir)
                else:
                    data_dir = self._full_data_dir('tikv', uuid)
        try:
            deploy_dir = srv['full_deploy_dir']
        except KeyError:
            try:
                deploy_dir = self._full_deploy_dir(
                    'tikv', uuid, srv['deploy_dir'])
            except KeyError:
                if self.deploy_dir:
                    deploy_dir = self._full_deploy_dir(
                        'tikv', uuid, self.deploy_dir)
                else:
                    deploy_dir = self._full_deploy_dir('tikv', uuid)
        try:
            ssh_port = srv['ssh_port']
        except KeyError:
            ssh_port = self.ssh_port
        try:
            offline = srv['offline']
        except KeyError:
            offline = False
        try:
            numa_node = srv['numa_node']
        except KeyError:
            numa_node = False
        try:
            label = srv['label']
        except KeyError:
            label = False
        self.server = {
            'ip': ip,
            'port': int(port),
            'status_port': int(status_port),
            'full_data_dir': data_dir,
            'full_deploy_dir': deploy_dir,
            'uuid': uuid,
            'ssh_port': int(ssh_port),
            'offline': offline,
            'numa_node': numa_node,
            'label': label
        }
        return self

    def port(self, new_port=None):
        if new_port:
            if new_port[0] == new_port[1]:
                raise exceptions.TiOPSConfigError(new_port,
                                                  'Same port number {} used for multiple purpose'.format(new_port[0]))
            self.server['port'], self.server['status_port'] = new_port[0], new_port[1]
        return self.server['port'], self.server['status_port']

    def _validate_ports(self):
        return self.server['port'] != self.server['status_port']


# topology config for a single Pump server
class TopologyPumpServer(TopologyBase):
    pump_port = 8250
    deploy_dir = None
    data_dir = None

    def __call__(self):
        return self.server

    def __init__(self, args=None, global_vars=None):
        super(TopologyPumpServer, self).__init__(args)

        # apply global variables
        _glv = global_vars if global_vars else {}
        for k, v in _glv.items():
            if k == 'port':
                self.pump_port = int(v)
            elif k == 'deploy_dir':
                self.deploy_dir = v
            elif k == 'data_dir':
                self.data_dir = v
            elif k == 'ssh_port':
                self.ssh_port = int(v)

    def format(self, srv, cluster_name=None):
        try:
            ip = srv['ip']
        except KeyError:
            term.fatal(
                'Config format error in "pump_servers": ip can not be empty.')
            raise
        try:
            port = srv['port']
        except KeyError:
            port = self.pump_port
        try:
            uuid = srv['uuid']
        except KeyError:
            uuid = utils.gen_uuid('{}:{}'.format(ip, port))
        try:
            data_dir = srv['full_data_dir']
        except KeyError:
            try:
                data_dir = self._full_data_dir('pump', uuid, srv['data_dir'])
            except KeyError:
                if self.data_dir:
                    data_dir = self._full_data_dir('pump', uuid, self.data_dir)
                else:
                    data_dir = self._full_data_dir('pump', uuid)
        try:
            deploy_dir = srv['full_deploy_dir']
        except KeyError:
            try:
                deploy_dir = self._full_deploy_dir(
                    'pump', uuid, srv['deploy_dir'])
            except KeyError:
                if self.deploy_dir:
                    deploy_dir = self._full_deploy_dir(
                        'pump', uuid, self.deploy_dir)
                else:
                    deploy_dir = self._full_deploy_dir('pump', uuid)
        try:
            ssh_port = srv['ssh_port']
        except KeyError:
            ssh_port = self.ssh_port
        try:
            offline = srv['offline']
        except KeyError:
            offline = False
        try:
            numa_node = srv['numa_node']
        except KeyError:
            numa_node = False
        self.server = {
            'ip': ip,
            'port': int(port),
            'full_data_dir': data_dir,
            'full_deploy_dir': deploy_dir,
            'uuid': uuid,
            'ssh_port': int(ssh_port),
            'offline': offline,
            'numa_node': numa_node
        }
        return self

    def port(self, new_port=None):
        if new_port:
            self.server['port'] = new_port
        return self.server['port']


# topology config for a single Drainer server
class TopologyDrainerServer(TopologyBase):
    drainer_port = 8249
    deploy_dir = None
    data_dir = None

    def __call__(self):
        return self.server

    def __init__(self, args=None, global_vars=None):
        super(TopologyDrainerServer, self).__init__(args)

        # apply global variables
        _glv = global_vars if global_vars else {}
        for k, v in _glv.items():
            if k == 'port':
                self.drainer_port = int(v)
            elif k == 'deploy_dir':
                self.deploy_dir = v
            elif k == 'data_dir':
                self.data_dir = v
            elif k == 'ssh_port':
                self.ssh_port = int(v)

    def format(self, srv, cluster_name=None):
        try:
            ip = srv['ip']
        except KeyError:
            term.fatal(
                'Config format error in "dariner_server": ip can not be empty.')
            raise
        try:
            commit_ts = srv['commit_ts']
        except KeyError:
            term.fatal(
                'Config format error in "dariner_server": commit_ts can not be empty.')
            raise
        try:
            port = srv['port']
        except KeyError:
            port = self.drainer_port
        try:
            uuid = srv['uuid']
        except KeyError:
            uuid = utils.gen_uuid('{}:{}'.format(ip, port))
        try:
            data_dir = srv['full_data_dir']
        except KeyError:
            try:
                data_dir = self._full_data_dir(
                    'drainer', uuid, srv['data_dir'])
            except KeyError:
                if self.data_dir:
                    data_dir = self._full_data_dir(
                        'drainer', uuid, self.data_dir)
                else:
                    data_dir = self._full_data_dir('drainer', uuid)
        try:
            deploy_dir = srv['full_deploy_dir']
        except KeyError:
            try:
                deploy_dir = self._full_deploy_dir(
                    'drainer', uuid, srv['deploy_dir'])
            except KeyError:
                if self.data_dir:
                    deploy_dir = self._full_deploy_dir(
                        'drainer', uuid, self.deploy_dir)
                else:
                    deploy_dir = self._full_deploy_dir('drainer', uuid)
        try:
            ssh_port = srv['ssh_port']
        except KeyError:
            ssh_port = self.ssh_port
        try:
            offline = srv['offline']
        except KeyError:
            offline = False
        try:
            numa_node = srv['numa_node']
        except KeyError:
            numa_node = False
        self.server = {
            'ip': ip,
            'port': int(port),
            'full_data_dir': data_dir,
            'full_deploy_dir': deploy_dir,
            'uuid': uuid,
            'ssh_port': int(ssh_port),
            'commit_ts': commit_ts,
            'offline': offline,
            'numa_node': numa_node
        }
        return self

    def port(self, new_port=None):
        if new_port:
            self.server['port'] = new_port
        return self.server['port']


# topology config for a single monitoring agents on server
class TopologyMonitorAgentServer(TopologyBase):
    node_exporter_port = 9100
    blackbox_exporter_port = 9115

    def __call__(self):
        return self.server

    def __init__(self, args=None):
        super(TopologyMonitorAgentServer, self).__init__(args)

    def format(self, srv, cluster_name=None):
        try:
            ip = srv['ip']
        except KeyError:
            msg = 'Config format error in "monitored_servers": ip can not be empty.'
            term.fatal(msg)
            raise exceptions.TiOPSConfigError(srv, msg)
        try:
            ssh_port = srv['ssh_port']
        except KeyError:
            ssh_port = self.ssh_port
        try:
            node_exporter_port = srv['node_exporter_port']
        except KeyError:
            node_exporter_port = self.node_exporter_port
        try:
            blackbox_exporter_port = srv['blackbox_exporter_port']
        except KeyError:
            blackbox_exporter_port = self.blackbox_exporter_port
        try:
            uuid = srv['uuid']
        except KeyError:
            uuid = utils.gen_uuid('{}:{}:{}'.format(
                ip, node_exporter_port, blackbox_exporter_port))
        try:
            deploy_dir = srv['full_deploy_dir']
        except KeyError:
            try:
                deploy_dir = self._full_deploy_dir(
                    'monitored', uuid, srv['deploy_dir'])
            except KeyError:
                deploy_dir = self._full_deploy_dir('monitored', uuid)
        try:
            numa_node = srv['numa_node']
        except KeyError:
            numa_node = False
        self.server = {
            'ip': ip,
            'ssh_port': ssh_port,
            'full_deploy_dir': deploy_dir,
            'uuid': uuid,
            'node_exporter_port': node_exporter_port,
            'blackbox_exporter_port': blackbox_exporter_port,
            'numa_node': numa_node
        }
        return self

    def port(self, new_port=None):
        if new_port:
            if new_port[0] == new_port[1]:
                raise exceptions.TiOPSConfigError(new_port,
                                                  'Same port number {} used for multiple purpose'.format(new_port[0]))
            self.server['node_exporter_port'], self.server['blackbox_exporter_port'] = new_port[0], new_port[1]
        return self.server['node_exporter_port'], self.server['blackbox_exporter_port']

    def _validate_ports(self):
        return self.server['node_exporter_port'] != self.server['blackbox_exporter_port']


# topology config for a single monitoring system server
class TopologyMonitorSystemServer(TopologyBase):
    prometheus_port = 9090
    pushgateway_port = 9091

    def __call__(self):
        return self.server

    def __init__(self, args=None):
        super(TopologyMonitorSystemServer, self).__init__(args)

    def format(self, srv, cluster_name=None):
        try:
            ip = srv['ip']
        except KeyError:
            msg = 'Config format error in "monitoring_servers": ip can not be empty.'
            term.fatal(msg)
            raise exceptions.TiOPSConfigError(
                srv, msg)
        try:
            ssh_port = srv['ssh_port']
        except KeyError:
            ssh_port = self.ssh_port
        try:
            prometheus_port = srv['prometheus_port']
        except KeyError:
            prometheus_port = self.prometheus_port
        try:
            pushgateway_port = srv['pushgateway_port']
        except KeyError:
            pushgateway_port = self.pushgateway_port
        try:
            uuid = srv['uuid']
        except KeyError:
            uuid = utils.gen_uuid('{}:{}:{}'.format(
                ip, prometheus_port, pushgateway_port))
        try:
            deploy_dir = srv['full_deploy_dir']
        except KeyError:
            try:
                deploy_dir = self._full_deploy_dir(
                    'monitoring', uuid, srv['deploy_dir'])
            except KeyError:
                deploy_dir = self._full_deploy_dir('monitoring', uuid)
        try:
            data_dir = srv['full_data_dir']
        except KeyError:
            try:
                data_dir = self._full_data_dir(
                    'monitoring', uuid, srv['data_dir'])
            except KeyError:
                data_dir = self._full_data_dir('monitoring', uuid)
        try:
            numa_node = srv['numa_node']
        except KeyError:
            numa_node = False
        self.server = {
            'ip': ip,
            'ssh_port': int(ssh_port),
            'full_deploy_dir': deploy_dir,
            'full_data_dir': data_dir,
            'uuid': uuid,
            'prometheus_port': prometheus_port,
            'pushgateway_port': pushgateway_port,
            'numa_node': numa_node
        }
        return self

    def port(self, new_port=None):
        if new_port:
            if new_port[0] == new_port[1]:
                raise exceptions.TiOPSConfigError(new_port,
                                                  'Same port number {} used for multiple purpose'.format(new_port[0]))
            self.server['prometheus_port'], self.server['pushgateway_port'] = new_port[0], new_port[1]
        return self.server['prometheus_port'], self.server['pushgateway_port']

    def _validate_ports(self):
        return self.server['prometheus_port'] != self.server['pushgateway_port']


# topology config for Grafana server
class TopologyGrafanaServer(TopologyBase):
    grafana_port = 3000

    def __call__(self):
        return self.server

    def __init__(self, args=None):
        super(TopologyGrafanaServer, self).__init__(args)

    def format(self, srv, cluster_name=None):
        try:
            ip = srv['ip']
        except KeyError:
            msg = 'Config format error in "grafana_server": ip can not be empty.'
            term.fatal(msg)
            raise exceptions.TiOPSConfigError(srv, msg)
        try:
            ssh_port = srv['ssh_port']
        except KeyError:
            ssh_port = self.ssh_port
        try:
            port = srv['port']
        except KeyError:
            port = self.grafana_port
        try:
            uuid = srv['uuid']
        except KeyError:
            uuid = utils.gen_uuid('{}:{}'.format(ip, port))
        try:
            deploy_dir = srv['full_deploy_dir']
        except KeyError:
            try:
                deploy_dir = self._full_deploy_dir(
                    'grafana', uuid, srv['deploy_dir'])
            except KeyError:
                deploy_dir = self._full_deploy_dir('grafana', uuid)
        try:
            numa_node = srv['numa_node']
        except KeyError:
            numa_node = False
        self.server = {
            'ip': ip,
            'ssh_port': int(ssh_port),
            'full_deploy_dir': deploy_dir,
            'uuid': uuid,
            'port': port,
            'numa_node': numa_node
        }
        return self

    def port(self, new_port=None):
        if new_port:
            self.server['port'] = new_port
        return self.server['port']


# topology config for Alertmanager server
class TopologyAlertmanagerServer(TopologyBase):
    web_port = 9093
    cluster_port = 9094

    def __call__(self):
        return self.server

    def __init__(self, args=None):
        super(TopologyAlertmanagerServer, self).__init__(args)

    def format(self, srv, cluster_name=None):
        try:
            ip = srv['ip']
        except KeyError:
            msg = 'Config format error in "alertmanager_server": ip can not be empty.'
            term.fatal(msg)
            raise exceptions.TiOPSConfigError(srv, msg)
        try:
            ssh_port = srv['ssh_port']
        except KeyError:
            ssh_port = self.ssh_port
        try:
            web_port = srv['web_port']
        except KeyError:
            web_port = self.web_port
        try:
            cluster_port = srv['cliuster_port']
        except KeyError:
            cluster_port = self.cluster_port
        try:
            uuid = srv['uuid']
        except KeyError:
            uuid = utils.gen_uuid(
                '{}:{}:{}'.format(ip, web_port, cluster_port))
        try:
            deploy_dir = srv['full_deploy_dir']
        except KeyError:
            try:
                deploy_dir = self._full_deploy_dir(
                    'alertmanager', uuid, srv['deploy_dir'])
            except KeyError:
                deploy_dir = self._full_deploy_dir('alertmanager', uuid)
        try:
            data_dir = srv['full_data_dir']
        except KeyError:
            try:
                data_dir = self._full_data_dir(
                    'alertmanager', uuid, srv['data_dir'])
            except KeyError:
                data_dir = self._full_data_dir('alertmanager', uuid)
        try:
            numa_node = srv['numa_node']
        except KeyError:
            numa_node = False
        self.server = {
            'ip': ip,
            'ssh_port': int(ssh_port),
            'full_deploy_dir': deploy_dir,
            'full_data_dir': data_dir,
            'uuid': uuid,
            'web_port': web_port,
            'cluster_port': cluster_port,
            'numa_node': numa_node
        }
        return self

    def port(self, new_port=None):
        if new_port:
            if new_port[0] == new_port[1]:
                raise exceptions.TiOPSConfigError(new_port,
                                                  'Same port number {} used for multiple purpose'.format(new_port[0]))
            self.server['web_port'], self.server['cluster_port'] = new_port[0], new_port[1]
        return self.server['web_port'], self.server['cluster_port']

    def _validate_ports(self):
        return self.server['web_port'] != self.server['cluster_port']


class Topology(TopologyBase):
    def __call__(self):
        return self._topology()

    # load topology from file
    # __init__ will first try to read any exist topology file in the profile directory,
    # then use the new config file inputed in the argument to overwrite it with any
    # values defined in the config.
    # That means, every time a (partial) config file is passed to Topology, a new, updated
    # version of the full topology file will be generated and saved to profile directory.
    def __init__(self, args=None, merge=False):
        try:
            super(Topology, self).__init__(args)
            self.__do_init(args, merge)
        except exceptions.TiOPSException as e:
            term.fatal(str(e))
            exit(1)

    def __do_init(self, args=None, merge=False):
        # flag identifing if current topology should be changed
        _overwrite = False

        self.topology_file = str(utils.profile_path(
            self.cluster_dir, 'topology.yaml'))
        try:
            filepath = args.topology
        except AttributeError:
            filepath = None

        # check before read input config
        if filepath and not os.path.isfile(filepath):
            msg = 'Input topology config {} does not exist, abort.'.format(
                filepath)
            raise exceptions.TiOPSRuntimeError(msg=msg)
        if merge and not filepath:
            msg = 'No new config set to merge, abort.'
            raise exceptions.TiOPSArgumentError(msg=msg)

        # read (partial) topology config from input path
        if filepath:
            try:
                _input_config = self._topology(
                    self._format(utils.read_yaml(filepath), self.cluster_name, args))
                _overwrite = True  # cached topology should be updated
            except Exception as e:
                msg = 'Error reading input topology config: {}'.format(e)
                term.debug('filepath: {}'.format(filepath))
                raise exceptions.TiOPSRuntimeError(msg=msg)
        else:
            _input_config = None

        # try to generate monitored_servers
        if _input_config:
            _input_config['monitored_servers'] = self.__auto_monitored(args, _input_config)

        if not os.path.exists(self.meta_file):
            # read current full topology in profile directory (if exist)
            if not os.path.exists(self.topology_file):
                term.info('No exist config for {}, tring to generate metadata for the new cluster from input.'.format(
                    self.cluster_name))
                _overwrite = False
            elif not os.path.isfile(self.topology_file):
                msg = 'Check profile directory failed, {} is not a file.'.format(
                    self.topology_file)
                raise exceptions.TiOPSRuntimeError(msg=msg)
            else:
                _overwrite = True
                merge = False # force overwrite, do not try to merge
                term.debug('No metadata found, force overwritting topology.')
            try:
                self.topology = self._format(
                    _input_config, self.cluster_name, args)
            except Exception as e:
                msg = 'Can not parse input topology config: {}'.format(e)
                raise exceptions.TiOPSConfigError(_input_config, msg=msg)
        else:
            # read exist topology file from cluster
            try:
                self.topology = self._format(
                    utils.read_yaml(self.topology_file), self.cluster_name, args)
            except Exception as e:
                msg = 'Error loading instance topology: {}'.format(e)
                term.debug('filepath: {}'.format(self.topology_file))
                raise exceptions.TiOPSRuntimeError(msg=msg)

        # do merging, nothing will hapen if _input_config is None
        if _overwrite and merge:
            self._merge_topology(_input_config)

        try:
            # validate the full topology
            self._validate()
            term.debug('Instance topology validation passed.')
        except exceptions.TiOPSConfigError as e:
            msg = 'Instance topology validation failed: {}'.format(e)
            raise exceptions.TiOPSRuntimeError(msg=msg)

        # save topology to file if needed
        self._save_topology(overwrite=_overwrite)

    # try to parse cluster-name from topology, in this way we don't need to store
    # a cluster-name variable in the YAML config.
    def _parse_name(self, config):
        # try to parse cluster name from the first deploy_dir path
        for srv in self._servers(config):
            try:
                self.cluster_name = os.path.split(
                    os.path.split(srv['full_deploy_dir'])[0])[1]
                return
            except KeyError:
                continue

    # save full topology file to user's profile directory
    def _save_topology(self, overwrite=False):
        # make sure the directory exist
        utils.create_dir(utils.profile_path(cluster=self.cluster_dir))
        # save topology to file
        _file = utils.profile_path(
            cluster=self.cluster_dir, subpath='topology.yaml')
        # backup exist file
        if overwrite:
            _old_file = '{}.old'.format(_file)
            try:
                os.rename(_file, _old_file)
                term.debug(
                    'Former version of topology.yaml saved to {}'.format(_old_file))
            except OSError as e:
                term.debug(
                    'Former version of topology.yaml not saved, {}'.format(e))
                pass
            utils.write_yaml(_file, self._topology())
        elif not os.path.isfile(_file):
            utils.write_yaml(_file, self._topology())
            term.debug('Created new topology file {}'.format(_file))
        else:
            term.debug('Skipped topology file overwritting, nothing changed.')

    # set metadata
    def set_meta(self, user=None, version=None, enable_tls=None, firewall=None):
        if user:
            self.user = user
        if version:
            self.version = version.lstrip('v')
        if enable_tls:
            self.enable_tls = enable_tls
        if firewall:
            self.firewall = firewall
        self._save_metadata()

    # save metadata of the cluster to user's profile directory
    def _save_metadata(self):
        # make sure the directory exist
        utils.create_dir(utils.profile_path(cluster=self.cluster_dir))

        _file = utils.profile_path(
            cluster=self.cluster_dir, subpath='meta.yaml')

        # try to read from exist file
        try:
            _meta = utils.read_yaml(_file)
        except EnvironmentError as e:
            import errno
            # only pass when the error is file not found
            if e.errno != errno.ENOENT:
                raise
            _meta = {}

        # write metadata to file
        _meta['user'] = self.user
        _meta['tidb_version'] = self.version
        _meta['enable_tls'] = self.enable_tls
        _meta['firewall'] = self.firewall
        utils.write_yaml(_file, _meta)

    # find a server in the topology's server list by its ip and port
    def __find_server(self, group, host):
        i = 0
        for srv in self.topology[group]:
            _host = '{}:{}'.format(srv.ip(), srv.port())
            if _host == host:
                return i
            i += 1
        return -1

    # find a server in the topology's server list by its uuid
    def __find_uuid(self, config=None, uuid=None):
        if not config:
            config = self.topology
        for grp, servers in config.items():
            i = 0
            for srv in servers:
                if srv()['uuid'] == uuid:
                    return grp, i
                i += 1
        return None, -1

    # create a new server object based on the group
    def __new_server(self, group, config=None):
        if group == 'pd_servers':
            return TopologyPDServer.new(cluster_name=self.cluster_name,
                                        data_dir=self._data_dir,
                                        server=config,
                                        user=self.user)
        if group == 'tidb_servers':
            return TopologyTiDBServer.new(cluster_name=self.cluster_name,
                                          data_dir=self._data_dir,
                                          server=config,
                                          user=self.user)
        if group == 'tikv_servers':
            return TopologyTiKVServer.new(cluster_name=self.cluster_name,
                                          data_dir=self._data_dir,
                                          server=config,
                                          user=self.user)
        if group == 'pump_servers':
            return TopologyPumpServer.new(cluster_name=self.cluster_name,
                                          data_dir=self._data_dir,
                                          server=config,
                                          user=self.user)
        if group == 'drainer_servers':
            return TopologyDrainerServer.new(cluster_name=self.cluster_name,
                                             data_dir=self._data_dir,
                                             server=config,
                                             user=self.user)
        if group == 'monitored_servers':
            return TopologyMonitorAgentServer.new(cluster_name=self.cluster_name,
                                                  data_dir=self._data_dir,
                                                  server=config,
                                                  user=self.user)
        if group == 'monitoring_server':
            return TopologyMonitorSystemServer.new(cluster_name=self.cluster_name,
                                                   data_dir=self._data_dir,
                                                   server=config,
                                                   user=self.user)
        if group == 'grafana_server':
            return TopologyGrafanaServer.new(cluster_name=self.cluster_name,
                                             data_dir=self._data_dir,
                                             server=config,
                                             user=self.user)
        if group == 'alertmanager_server':
            return TopologyAlertmanagerServer.new(cluster_name=self.cluster_name,
                                                  data_dir=self._data_dir,
                                                  server=config,
                                                  user=self.user)
        raise NotImplementedError

    # add all server IPs to the 'monitored_servers' group, if there is already servers
    # set in the input file, then use the input list and ignore auto generation
    def __auto_monitored(self, args, config=None):
        if not config:
            raise exceptions.TiOPSArgumentError('config must not be None')
        if not isinstance(config, dict):
            raise exceptions.TiOPSArgumentError('config is in unsupported format')
        if 'monitored_servers' in config.items() \
            and len(config['monitored_servers']) > 0:
            return config['monitored_servers']

        _monitored = set()
        for _grp, _srvs in config.items():
            if not isinstance(_srvs, list):
                continue
            for _srv in _srvs:
                try:
                    _monitored.add(_srv['ip'])
                except KeyError:
                    pass
        term.debug('Automatically generated monitored_servers from input: {}'.format(_monitored))

        _result = []
        for _ip in _monitored:
            _srv = TopologyMonitorAgentServer(args).format({'ip': _ip})
            _result.append(_srv())

        return _result

    # overwrite topology with the config passed in, this only work for pre-exist servers
    def _merge_topology(self, config=None):
        if not config:
            return

        for grp, servers in config.items():
            if not grp in ['pd_servers',
                           'tidb_servers',
                           'tikv_servers',
                           'pump_servers',
                           'drainer_servers',
                           'monitored_servers',
                           'monitoring_server',
                           'grafana_server',
                           'alertmanager_server',
                           ]:
                term.debug(
                    'config merging for {} not supported yet'.format(grp))
                continue
            if not servers:
                continue
            for srv in servers:
                # When trying to merge, the input (partial) config must at least have
                # the same IP address and port(s) as an exist server in the topology.
                # If port(s) are default values, they can be omitted.
                _srv = self.__new_server(grp, srv)
                try:
                    if not _srv._validate_ports():
                        raise exceptions.TiOPSConfigError(_srv,
                                                          'The same port number {}:{} used for multiple purpose'.format(
                                                              _srv.ip(), _srv.port()[0]))
                except NotImplementedError:
                    pass
                try:
                    _host_new = '{}:{}'.format(_srv.ip(), _srv.port())
                except NotImplementedError:
                    continue

                # try to find the same ip:port in exist topology
                _iter = self.__find_server(grp, _host_new)
                if _iter < 0:
                    msg = 'Host {} not found in exist topology, abort config merging.'.format(
                        _host_new)
                    term.debug(msg)
                    raise exceptions.TiOPSAttributeError(msg)

                # actually overwrite the values
                # although we built a _srv object above, we still use the input plain
                # dictionary to get values, as some missing values may be overwritten
                # to default values during the init process.
                # this ensure only settings specified in the input config is overwritten
                # to the current topology, key by key.
                for k, v in srv.items():
                    self.topology[grp][_iter].server[k] = v

        # try to validate the config after merging
        self._validate()

    # build a dict with server_ip:port as key, and see if there's any confliction
    def _servers(self, config):
        full_lst = []

        role_group = {'PD': 'pd_servers', 'TiDB': 'tidb_servers', 'TiKV': 'tikv_servers', 'Pump': 'pump_servers',
                      'Drainer': 'drainer_servers', 'Monitoring': 'monitoring_server', 'Monitored': 'monitored_servers',
                      'Grafana': 'grafana_server', 'Alertmanager': 'alertmanager_server'}

        for group in role_group.keys():
            _grp_name = role_group[group]
            _srv_lst = []
            try:
                for srv in config[_grp_name]:
                    _srv = srv()
                    _srv['role'] = group
                    _srv_lst.append(_srv)
            except KeyError:
                pass
            full_lst += _srv_lst
        return full_lst

    # format the topology config, set missing keys to default values
    def _format(self, config=None, cluster_name=None, args=None):
        if not config:
            config = self.topology
        full_config = {
            'pd_servers': [],
            'tidb_servers': [],
            'tikv_servers': [],
            'pump_servers': [],
            'drainer_servers': [],
            'monitored_servers': [],
            'monitoring_server': [],
            'grafana_server': [],
            'alertmanager_server': []
        }

        # set cluster_name if specified
        if cluster_name:
            self.cluster_name = cluster_name
        else:
            # try to parse cluster_name from the config
            self._parse_name(config)

        # PD configs
        if 'pd_servers' in config and config['pd_servers']:
            try:
                _glv = config['global']['pd_servers']
            except KeyError:
                _glv = None
            for srv in config['pd_servers']:
                full_config['pd_servers'].append(
                    TopologyPDServer(args, global_vars=_glv).format(srv))

        # TiDB configs
        if 'tidb_servers' in config and config['tidb_servers']:
            try:
                _glv = config['global']['tidb_servers']
            except KeyError:
                _glv = None
            for srv in config['tidb_servers']:
                full_config['tidb_servers'].append(
                    TopologyTiDBServer(args, global_vars=_glv).format(srv))

        # TiKV configs
        if 'tikv_servers' in config and config['tikv_servers']:
            try:
                _glv = config['global']['tikv_servers']
            except KeyError:
                _glv = None
            for srv in config['tikv_servers']:
                full_config['tikv_servers'].append(
                    TopologyTiKVServer(args, global_vars=_glv).format(srv))

        # Pump configs
        if 'pump_servers' in config and config['pump_servers']:
            try:
                _glv = config['global']['pump_servers']
            except KeyError:
                _glv = None
            for srv in config['pump_servers']:
                full_config['pump_servers'].append(
                    TopologyPumpServer(args, global_vars=_glv).format(srv))

        # Drainer configs
        if 'drainer_servers' in config and config['drainer_servers']:
            try:
                _glv = config['global']['drainer_servers']
            except KeyError:
                _glv = None
            for srv in config['drainer_servers']:
                full_config['drainer_servers'].append(
                    TopologyDrainerServer(args, global_vars=_glv).format(srv))

        if 'monitored_servers' in config and config['monitored_servers']:
            for srv in config['monitored_servers']:
                full_config['monitored_servers'].append(
                    TopologyMonitorAgentServer(args).format(srv))

        if 'monitoring_server' in config and config['monitoring_server']:
            for srv in config['monitoring_server']:
                full_config['monitoring_server'].append(
                    TopologyMonitorSystemServer(args).format(srv))

        if 'alertmanager_server' in config and config['alertmanager_server']:
            for srv in config['alertmanager_server']:
                full_config['alertmanager_server'].append(
                    TopologyAlertmanagerServer(args).format(srv))

        if 'grafana_server' in config and config['grafana_server']:
            for srv in config['grafana_server']:
                full_config['grafana_server'].append(
                    TopologyGrafanaServer(args).format(srv))
        else:
            # Grafana is optional
            term.warn(
                'Grafana is not set, no monitoring dashboard will be deployed.')

        return full_config

    def _validate(self, config=None):
        topo = self.topology
        if config:
            topo = config

        _srv_list = []
        for _grp, _srvs in topo.items():
            _srv_list += [_srv for _srv in _srvs]

        _ports = []
        _paths = []
        _labels = []
        for srv in _srv_list:
            try:
                if not srv._validate_ports():
                    # check for simplest port confliction in one same server
                    raise exceptions.TiOPSConfigError(srv,
                                                      'Port confliction in config: server {}({}) has defined the same port {} for different usages.'.format(
                                                          srv.ip(), srv.server['uuid'], srv.port()))
                for _p in srv.port():
                    _key = '{}:{}'.format(srv.ip(), _p)
                    _ports.append(_key)
            except AttributeError:
                # when there is only one port
                _key = '{}:{}'.format(srv.ip(), srv.port())
                _ports.append(_key)
            except NotImplementedError:
                pass

            try:
                # data_dir is not necessary for all components
                _paths.append(srv()['full_data_dir'])
            except KeyError:
                pass
            _paths.append(srv()['full_deploy_dir'])
            try:
                # labels is only setting in tikv
                _labels.append(utils.format_labels(srv()['label'])[0])
            except KeyError:
                pass

        # check server:port duplicates in config
        cnt = Counter(_ports)
        dups = [k for k, v in cnt.items() if v > 1]
        if len(dups) > 0:
            msg = []
            for k in dups:
                msg.append('{}({} times)'.format(k, cnt[k]))
            raise exceptions.TiOPSConfigError(dups,
                                              'Port confliction in config: server ports defined multiple times, {}'.format(
                                                  ', '.join(msg)))

        # check dir path duplicaties in config, this also cover UUID duplicates
        cnt = Counter(_paths)
        dups = [k for k, v in cnt.items() if v > 1]
        if len(dups) > 0:
            raise exceptions.TiOPSConfigError(dups,
                                              'Path confliction in config: {} the same directory used for multiple nodes'.format(
                                                  dups))

        if len(set(_labels)) > 1:
            msg = list(set(_labels))
            raise exceptions.TiOPSConfigError(msg,
                                              'Label structure in multiple tikvs are inconsistent, inconsistent label are: {}'.format(
                                                  ', '.join(msg)))

    # return the topology in a plain  format
    def _topology(self, config=None):
        if not config:
            config = self.topology
        result = {}
        for grp, servers in config.items():
            result[grp] = [srv() for srv in servers]
        return result

    def servers(self, group=None):
        if not group:
            return self._servers(self.topology)
        elif group in ['pd_servers',
                       'tidb_servers',
                       'tikv_servers',
                       'pump_servers',
                       'drainer_servers',
                       ]:
            return [srv() for srv in self.topology[group]]
        else:
            raise NotImplementedError

    # add some (partial) configs to the current topology
    # NOTE: add() doesn't apply the new topology, replace() should be called after any operation
    # is done with the new servers
    def add(self, config=None):
        if not config:
            raise exceptions.TiOPSArgumentError(
                'Nothing to add.')

        _new_topology = self.topology
        _diff = {}

        for grp, servers in config.items():
            if grp not in ['pd_servers',
                           'tidb_servers',
                           'tikv_servers',
                           'pump_servers',
                           'drainer_servers',
                           'monitored_servers',
                           'monitoring_server',
                           'grafana_server',
                           'alertmanager_server',
                           ]:
                term.debug(
                    'config adding for {} not supported yet'.format(grp))
                continue
            if not servers:
                continue
            for srv in servers:
                try:
                    # fill server with default values (if anything is missing)
                    _srv = self.__new_server(grp, srv).format(
                        srv, self.cluster_name)
                    if not _srv._validate_ports():
                        raise exceptions.TiOPSConfigError(srv,
                                                          'The same port number {}:{} used for multiple purpose'.format(
                                                              srv.ip(), srv.port()[0]))
                except NotImplementedError:
                    pass

                _keys = []
                try:
                    for _p in _srv.port():
                        _host_new = '{}:{}'.format(_srv.ip(), _p)
                        _keys.append(_host_new)
                except (AttributeError, TypeError):
                    # when there is only one port
                    _host_new = '{}:{}'.format(_srv.ip(), _srv.port())
                    _keys.append(_host_new)
                except NotImplementedError:
                    term.debug('Skip unsupported server {}'.format(srv))
                    continue

                # try to find the same ip:port in exist topology
                for _key in _keys:
                    _iter = self.__find_server(grp, _host_new)
                    if _iter >= 0:
                        msg = 'Host {} already exist in exist topology, abort adding.'.format(
                            _host_new)
                        term.debug(msg)
                        raise exceptions.TiOPSConfigError(msg)

                # add server to the new full config
                try:
                    _diff[grp].append(_srv())
                except KeyError:
                    _diff[grp] = [_srv()]
                try:
                    _new_topology[grp].append(_srv)
                except KeyError:
                    _new_topology[grp] = [_srv]

        # validate the new full config
        self._validate(_new_topology)
        return _new_topology, _diff

    # delete servers from the current topology
    # NOTE: remove() doesn't apply the new topology, replace() should be called after any operation
    # is done with the deleted server
    def remove(self, uuid_list=None, delete=False):
        if not uuid_list:
            return self.topology, None

        _new_topology = copy.deepcopy(self.topology)
        _srv = {}

        for uuid in uuid_list.split(','):
            _grp, _i = self.__find_uuid(config=_new_topology, uuid=uuid)
            if not _grp or _i < 0:
                raise exceptions.TiOPSElementError(
                    uuid, 'Server {} not found in topology.'.format(uuid))

            try:
                _srv[_grp].append(self.topology[_grp][_i]())
            except KeyError:
                _srv[_grp] = [self.topology[_grp][_i]()]
            if _grp not in ['tikv_servers', 'pump_servers', 'drainer_servers']:
                del (_new_topology[_grp][_i])
            elif not delete:
                _new_topology[_grp][_i].server['offline'] = True
            elif delete:
                del (_new_topology[_grp][_i])
        if len(_srv) < 1:
            term.notice('There are no node to delete.')
        return _new_topology, _srv

    # directly apply a new topology file to replace the current one
    def replace(self, config=None, write=True):
        if not config:
            raise exceptions.TiOPSArgumentError(
                'Can not replace topology with empty config.')

        # validate the new full config
        self._validate(config)
        self.topology = config
        term.debug('Replaced topology with new config.')
        if write:
            self._save_topology(overwrite=True)
            term.debug('Saved new topology to file.')

    # get nodes by uuid, but in the format of a full topology dictionary
    def node(self, topology=None, uuid_list=None):
        if not uuid_list:
            return None

        if not topology:
            topology = self.topology
        _srv = {}
        for uuid in uuid_list.split(','):
            _grp, _i = self.__find_uuid(config=topology, uuid=uuid)
            if not _grp or _i < 0:
                # raise exceptions.TiOPSElementError(
                #     uuid, 'Server {} not found in topology.'.format(uuid))
                continue
            try:
                _srv[_grp].append(self.topology[_grp][_i])
            except KeyError:
                _srv[_grp] = [self.topology[_grp][_i]]

        return _srv

    def role(self, topology=None, roles=None):
        if not roles:
            return None

        if not topology:
            topology = self.topology
        _roles = ["pd", "tikv", "pump", "tidb", "drainer",
                  "monitoring", "monitored", "grafana", "alertmanager"]

        _srv = {}
        _service_group = {}
        for _service in self.service_group:
            _key = _service.keys()[0]
            _value = _service.values()[0]
            _service_group[_key] = _value

        for role in roles.split(','):
            if role not in _roles:
                msg = 'Unknown role {}, available roles are: {}.'.format(
                    role, _roles)
                raise exceptions.TiOPSArgumentError(msg)
            if role == 'monitoring':
                _group = 'monitoring_server'
            elif role == 'monitored':
                _group = 'monitored_servers'
            else:
                _group = _service_group[role]

            _srv[_group] = topology[_group]

        return _srv

    def role_node(self, roles=None, nodes=None):
        if roles and nodes:
            _topology_roles = self.role(topology=self.topology, roles=roles)
            _topology_roles_node = self.node(
                topology=_topology_roles, uuid_list=nodes)
            return self._topology(_topology_roles_node)
        elif roles:
            return self._topology(self.role(roles=roles))
        elif nodes:
            return self._topology(self.node(uuid_list=nodes))
        else:
            return self._topology(config=self.topology)
