# coding: utf-8

import sys
import os
import getpass
import argparse
from tiops import utils


class TiOPSParser(object):
    def __init__(self):
        self.parser = argparse.ArgumentParser()
        self.parser.add_argument("-v", "--verbose", action="store_true", default=False,
                                 help="Print verbose output.")

        subparsers = self.parser.add_subparsers(
            dest="action", help='tidb cluster commands')

        localhost_parser = subparsers.add_parser('bootstrap-local',
                                                 help='init localhost')
        ssh_parser = subparsers.add_parser('bootstrap-ssh',
                                           help='init ssh for cluster hosts')
        host_parser = subparsers.add_parser('bootstrap-host',
                                            help='init environment for cluster hosts ')
        cluster_deploy_parser = subparsers.add_parser('deploy',
                                                      help='create tidb cluster')
        cluster_start_parser = subparsers.add_parser('start',
                                                     help='start tidb cluster')
        cluster_stop_parser = subparsers.add_parser('stop',
                                                    help='stop tidb cluster')
        cluster_restart_parser = subparsers.add_parser('restart',
                                                       help='restart tidb cluster')
        cluster_upgrade_parser = subparsers.add_parser('upgrade',
                                                       help='upgrade tidb cluster')
        cluster_display_parser = subparsers.add_parser('display',
                                                       help='display tidb cluster')
        cluster_destroy_parser = subparsers.add_parser('destroy',
                                                       help='destroy tidb cluster')
        cluster_reload_parser = subparsers.add_parser('reload',
                                                      help='reload tidb cluster')
        # cluster_import_parser = tidb_cluster_subparsers.add_parser('import',
        #                                                                 help='import cluster info from tidb-ansible')
        cluster_econfig_parser = subparsers.add_parser('edit-config',
                                                       help='edit tidb cluster configuration')
        cluster_scaleout_parser = subparsers.add_parser('scale-out',
                                                        help='scale out tidb cluster')
        cluster_scalein_parser = subparsers.add_parser('scale-in',
                                                       help='scale in tidb cluster')
        cluster_exec_parser = subparsers.add_parser('exec',
                                                    help='run shell command on host in the tidb cluster')
        quickdeploy_parser = subparsers.add_parser('quickdeploy',
                                                   help='deploy a tidb cluster in demo mode')
        show_version_parser = subparsers.add_parser(
            'version', help='show TiOPS version and exit')

        self._bootstrap_args_ssh(ssh_parser)
        self._bootstrap_args_host(host_parser)
        self._tiops_args_display(cluster_display_parser)
        self._tiops_args_deploy(cluster_deploy_parser)
        self._tiops_args_start(cluster_start_parser)
        self._tiops_args_stop(cluster_stop_parser)
        self._tiops_args_restart(cluster_restart_parser)
        self._tiops_args_upgrade(cluster_upgrade_parser)
        self._tiops_args_destroy(cluster_destroy_parser)
        self._tiops_args_reload(cluster_reload_parser)
        self._tiops_args_edit_config(cluster_econfig_parser)
        self._tiops_args_scaleout(cluster_scaleout_parser)
        self._tiops_args_scalein(cluster_scalein_parser)
        self._tiops_args_qdeploy(quickdeploy_parser)
        self._tiops_args_exec(cluster_exec_parser)

    def __call__(self):
        return self.parser.parse_args()

    def __get_last_subcmd(self, parser):
        return parser.prog.split()[-1]

    def __check_parser(self, parser, value):
        subcmd = self.__get_last_subcmd(parser)
        return subcmd, subcmd == value

    def __common_args_bootstrap(self, bootstrap_parser):
        bootstrap_parser.add_argument('-H', '--host', dest='hosts', required=True,
                                      help='Initialized host list: file or ip list (example: 10.0.1.10,10.0.1.11,10.0.1.12)')
        bootstrap_parser.add_argument('-s', '--ssh-port', dest='port', default=22,
                                      help='the port for ssh. (default: 22)')
        bootstrap_parser.add_argument('-f', '--forks', dest='forks', default=5, type=int,
                                      help='Concurrency number (default: 5)')
        bootstrap_parser.add_argument('--private-key', dest='private_key',
                                      default='{}id_rsa'.format(utils.profile_path('.ssh')),
                                      help='Specify the private key, usually no need to specify and the default private key will be used')

    def _bootstrap_args_ssh(self, ssh_parser):
        if not self.__check_parser(ssh_parser, 'bootstrap-ssh')[1]:
            return
        self.__common_args_bootstrap(ssh_parser)
        ssh_parser.add_argument('-u', '--user', dest='user', default='root', required=True,
                                help='the tidb user to connect remote machine. (default: root)')
        ssh_parser.add_argument('-p', '--password', dest='password', default=None,
                                help='the password for connection user. (default: None)')
        ssh_parser.add_argument('-d', '--deploy-user', dest='deploy_user', default=getpass.getuser(),
                                help='the tidb user to create. (default: {})'.format(getpass.getuser()))

    def _bootstrap_args_host(self, host_parser):
        if not self.__check_parser(host_parser, 'bootstrap-host')[1]:
            return
        self.__common_args_bootstrap(host_parser)
        host_parser.add_argument('-d', '--deploy-user', dest='deploy_user', default=getpass.getuser(),
                                 help='the ssh user(create by init_network) to connect remote machine. '
                                      '(default: {})'.format(getpass.getuser()))
        host_parser.add_argument('--enable-check-ntp', dest='enable_check_ntp', default=False, action='store_true',
                                 help='Check if NTP is running to synchronize the time (default: disable)')
        host_parser.add_argument('--ntp-server', dest='ntp_server', default=None,
                                 help='Set ntp server config by specified ntp server '
                                      '(example: 10.0.0.10 or 10.0.0.10,10.0.0.11). (default: None)')
        host_parser.add_argument('--timezone', dest='timezone', default='Asia/Shanghai',
                                 help='Set timezone for cluster (default: Asia/Shanghai)')
        host_parser.add_argument('--enable-swap', dest='enable_swap', default=False, action='store_true',
                                 help='Enable swap (default: disable)')
        host_parser.add_argument('--disable-irqbalance', dest='disable_irqbalance', default=False, action='store_true',
                                 help='Disable irqbalance (default: enable)')
        host_parser.add_argument('--enable-checks', dest='enable_checks', default=False, action='store_true',
                                 help='Enable preflight checks (default: disable)')

    # common arguments added to every sub parser
    def __common_args_cluster(self, tidb_parser):
        _subcmd = self.__get_last_subcmd(tidb_parser)
        if _subcmd == 'display':
            tidb_parser.add_argument(
                '-c', '--cluster-name', dest='cluster_name', required=False, help='Cluster name')
        else:
            tidb_parser.add_argument(
                '-c', '--cluster-name', dest='cluster_name', required=True, help='Cluster name')
            tidb_parser.add_argument('--private-key', dest='private_key',
                                     default='{}id_rsa'.format(utils.profile_path('.ssh')),
                                     help='Specify the private key, usually no need to specify and the default private key will be used')

        if _subcmd in ['deploy', 'upgrade', 'quickdeploy']:
            tidb_parser.add_argument('-t', '--tidb-version', dest='tidb_version', default='3.0.9',
                                     help='TiDB cluster version (default: 3.0.9)')
            tidb_parser.add_argument('--enable-check-config', dest='enable_check_config', default=False,
                                     action='store_true',
                                     help='Check if the configuration of the tidb component is reasonable (default: disable)')
        if _subcmd in ['deploy', 'upgrade', 'scale-out', 'quickdeploy']:
            tidb_parser.add_argument('--local-pkg', dest='local_pkg', default=None,
                                     help='Specify a local path to pre-download packages instead of download them during the process.')
        if _subcmd not in ['edit-config', 'display']:
            tidb_parser.add_argument('-f', '--forks', dest='forks', default=5, type=int,
                                     help='Concurrency when deploy TiDB cluster (default: 5)')
        if _subcmd in ['start', 'stop', 'restart', 'reload', 'upgrade', 'display', 'scale-in']:
            tidb_parser.add_argument('-n', '--node-id', dest='node_id', default=None,
                                     help='specified node id')
        if _subcmd in ['start', 'stop', 'restart', 'reload', 'upgrade', 'display']:
            tidb_parser.add_argument('-r', '--role', dest='role', default=None,
                                     help='specified roles (items: ["pd", "tikv", "pump", "tidb", "drainer", "monitoring", "monitored", "grafana", "alertmanager"])')

    # common arguments added to deploy related sub parser (deploy and upgrade)
    def __common_args_deploy(self, tidb_parser):
        _subcmd = self.__get_last_subcmd(tidb_parser)

        tidb_parser.add_argument('-T', '--topology', dest='topology', default=None,
                                 help='Cluster topology file (example: "{}/tiops/templates/topology.yaml.example")'.format(
                                     os.environ['TIUP_COMPONENT_INSTALL_DIR']))
        tidb_parser.add_argument('--enable-check-cpu', dest='enable_check_cpu', default=False, action='store_true',
                                 help='Check cpu vcores number (default: disable)')
        tidb_parser.add_argument('--enable-check-mem', dest='enable_check_mem', default=False, action='store_true',
                                 help='Check memory size (default: disable)')
        tidb_parser.add_argument('--enable-check-disk', dest='enable_check_disk', default=False, action='store_true',
                                 help='Check available disk space (default: disable)')
        tidb_parser.add_argument('--enable-check-iops', dest='enable_check_iops', default=False, action='store_true',
                                 help='Check iops and latency for TiKV data directory (default: disable)')
        tidb_parser.add_argument('--enable-check-all', dest='enable_check_all', default=False, action='store_true',
                                 help='Check all environmental information (default: disable)')

    # Parse tidb arguments
    def _tiops_args_display(self, tidb_parser):
        if not self.__check_parser(tidb_parser, 'display')[1]:
            return
        self.__common_args_cluster(tidb_parser)
        tidb_parser.add_argument('--ip', dest='ip_addr', default=None,
                                 help='Show only node(s) with specified IP')
        tidb_parser.add_argument('--status', dest='show_status', default=False, action='store_true',
                                 help='Show node status in output, this might take some time.')

    def _tiops_args_edit_config(self, tidb_parser):
        if not self.__check_parser(tidb_parser, 'edit-config')[1]:
            return
        self.__common_args_cluster(tidb_parser)

    def _tiops_args_deploy(self, tidb_parser):
        if not self.__check_parser(tidb_parser, 'deploy')[1]:
            return
        self.__common_args_cluster(tidb_parser)
        self.__common_args_deploy(tidb_parser)
        tidb_parser.add_argument('-d', '--deploy-user', dest='deploy_user', default=getpass.getuser(),
                                 help='The user to connect remote machine (default: {})'.
                                 format(getpass.getuser()))
        tidb_parser.add_argument('--enable-firewall', dest='firewall', default=False, action='store_true',
                                 help='Set and start firewalld; False: stop firewalld (default: disable)')
        # tidb_parser.add_argument('--enable-numa', dest='numa', default=False, action='store_true',
        #                          help='Set numa for tidb-server when machine have installed numactl (default: False)')

    def _tiops_args_upgrade(self, tidb_parser):
        if not self.__check_parser(tidb_parser, 'upgrade')[1]:
            return
        self.__common_args_cluster(tidb_parser)
        tidb_parser.add_argument('--force', dest='force', default=False, action='store_true',
                                 help='Upgrade without transfer leader, fast but affects stability (default: disabled)')

    def _tiops_args_start(self, tidb_parser):
        if not self.__check_parser(tidb_parser, 'start')[1]:
            return
        self.__common_args_cluster(tidb_parser)

    def _tiops_args_stop(self, tidb_parser):
        if not self.__check_parser(tidb_parser, 'stop')[1]:
            return
        self.__common_args_cluster(tidb_parser)

    def _tiops_args_restart(self, tidb_parser):
        if not self.__check_parser(tidb_parser, 'restart')[1]:
            return
        self.__common_args_cluster(tidb_parser)

    def _tiops_args_reload(self, tidb_parser):
        if not self.__check_parser(tidb_parser, 'reload')[1]:
            return
        self.__common_args_cluster(tidb_parser)

    def _tiops_args_destroy(self, tidb_parser):
        if not self.__check_parser(tidb_parser, 'destroy')[1]:
            return
        self.__common_args_cluster(tidb_parser)

    def _tiops_args_scaleout(self, tidb_parser):
        if not self.__check_parser(tidb_parser, 'scale-out')[1]:
            return
        self.__common_args_cluster(tidb_parser)
        self.__common_args_deploy(tidb_parser)

    def _tiops_args_scalein(self, tidb_parser):
        if not self.__check_parser(tidb_parser, 'scale-in')[1]:
            return
        self.__common_args_cluster(tidb_parser)

    def _tiops_args_qdeploy(self, tidb_parser):
        if not self.__check_parser(tidb_parser, 'quickdeploy')[1]:
            return
        self.__common_args_cluster(tidb_parser)
        self.__common_args_deploy(tidb_parser)
        tidb_parser.add_argument('-s', '--ssh-port', dest='port', default=22,
                                 help='the port for ssh. (default: 22)')
        tidb_parser.add_argument('-u', '--user', dest='user', default='root',
                                 help='the tidb user to connect remote machine. (default: root)')
        tidb_parser.add_argument('-p', '--password', dest='password', default=None,
                                 help='the password for connection user. (default: None)')
        tidb_parser.add_argument('-d', '--deploy-user', dest='deploy_user', default=getpass.getuser(),
                                 help='the tidb user to run tidb components. (default: {})'.format(getpass.getuser()))

    def _tiops_args_exec(self, tidb_parser):
        if not self.__check_parser(tidb_parser, 'exec')[1]:
            return
        self.__common_args_cluster(tidb_parser)
        tidb_parser.add_argument('--root', dest='root', action='store_true', default=False,
                                 help='try to run the command with root priviledge')
        tidb_parser.add_argument(
            'cmd', nargs="*", help='shell command to execute')
