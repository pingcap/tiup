# coding: utf-8

import logging
import os
import getpass
import json
import copy

from tiops import utils
from tiops.ansibleapi import ansibleapi
from tiops.tui import term
from tiops import exceptions


class Init(object):
    cluster_name = None
    port = 22
    user = 'tidb'
    version = None

    def __init__(self, args=None):
        if not args:
            logging.error("Argument can't be empty.")
            raise ValueError
        self._args = args

        try:
            self.version = args.version
        except AttributeError:
            pass
        try:
            self.port = args.port
        except AttributeError:
            pass
        try:
            self.cluster_name = args.cluster_name
        except AttributeError:
            pass
        try:
            self.init_user = args.user
        except AttributeError:
            pass
        try:
            self.user = args.deploy_user
        except AttributeError:
            pass
        try:
            self.enable_checks = args.enable_checks
        except AttributeError:
            self.enable_checks = False

        # try to read hosts file from topology if the `-H` is not set
        # this doesn't effect manual `bootstrap-*` commands
        # as they don't have the `-T` argument
        if 'hosts' in args or 'topology' in args:
            try:
                _hosts_file = args.hosts
            except AttributeError:
                try:
                    _hosts_file = args.topology
                except AttributeError:
                    _host_file = None
            if os.path.isfile(_hosts_file):
                self.__read_host_list(_hosts_file)
            else:
                self.hosts = _hosts_file

        try:
            self.enable_check_ntp = args.enable_check_ntp
        except AttributeError:
            pass
        try:
            self.ntp_server = args.ntp_server
        except AttributeError:
            pass
        try:
            self.enable_swap = args.enable_swap
        except AttributeError:
            pass
        try:
            self.disable_irqbalance = args.disable_irqbalance
        except AttributeError:
            pass
        try:
            self.timezone = args.timezone
        except AttributeError:
            pass
        self.script_path = '{}/tiops/check/'.format(os.environ['TIUP_COMPONENT_INSTALL_DIR'])
        self.check_dir = '/tmp/tidb_check'
        self.host_vars = utils.profile_path('host_vars')

    def __read_host_list(self, fname=None):
        try:
            _host_info = utils.read_yaml(fname)
            if isinstance(_host_info, str):
                self.hosts = _host_info.replace(' ', ',')
            elif not isinstance(_host_info, dict):
                msg = "host list file containes unsupported content format."
                raise exceptions.TiOPSConfigError(_host_info, msg=msg)
            _ip_list = []
            for _grp, _service in _host_info.iteritems():
                if not isinstance(_service, list):
                    term.debug("Skipped unsupported section {}.".format(_grp))
                    continue
                for _node in _service:
                    try:
                        _ip_list.append(_node['ip'])
                    except KeyError:
                        pass
            self.hosts = ','.join(set(_ip_list))
        except Exception as e:
            term.debug('Exception when parsing host list: {}, ignored.'.format(e))

    def _check_ip_list(self, ipList=None):
        if not ipList:
            ipList = self.hosts

        invalid_list = []
        for ip in ipList.split(','):
            if not utils.is_valid_ip(ip):
                invalid_list.append(ip)

        if invalid_list:
            term.fatal('{} is invalid.'.format(','.join(invalid_list)))
            exit(1)

    def init(self, demo=False):
        term.notice('Start init management machine.')
        user_home = utils.home()
        if not os.path.exists('{}/.ssh'.format(user_home)):
            utils.create_dir(os.path.join(user_home, '.ssh'))
            os.chmod(os.path.join(user_home, '.ssh'), 0o700)
        if not os.path.isfile(os.path.join(user_home, '.ssh/id_rsa')) and \
                not os.path.isfile(os.path.join(user_home, '.ssh/id_rsa.pub')):
            term.info(
                '{} does not have SSH key. Start generating.'.format(getpass.getuser()))
            os.system(
                '/usr/bin/ssh-keygen -t rsa -N \'\' -f ~/.ssh/id_rsa -q'.format(getpass.getuser()))
        else:
            term.normal(
                '{} already have SSH key, skip create'.format(getpass.getuser()))

        utils.ansible_config()

        if demo:
            term.notice('Finished init management machine.')
        else:
            term.notice('Done!!!')

    def init_network(self, demo=False):
        term.notice(
            'Start creating no-password ssh connections between the management machine and other machines')
        self._check_ip_list()

        # get password from prompt
        if not self._args.password:
            term.info(
                'Please enter the password of init user on deployment server ({} password)'.format(self.init_user))
            _passwd = term.getpass()
        else:
            _passwd = self._args.password

        # create ansible runner
        initnet = ansibleapi.ANSRunner(
            ips=self.hosts, user=self.init_user, tiargs=self._args, passwd=_passwd)
        term.info('Create {} user on remote machines.'.format(self.user))
        initnet.run_model('user',
                          'name=%s '
                          'shell=/bin/bash '
                          'createhome=yes' % (self.user),
                          become=True)
        term.info(
            'Set authorized_keys for {} on cluster machine.'.format(self.user))
        initnet.run_model('authorized_key',
                          'user=%s '
                          'key={{ lookup("file", "%s/.ssh/id_rsa.pub") }}'
                          % (self.user, utils.home()),
                          become=True)
        term.info(
            'Add sudo permissions for {} on cluster machine.'.format(self.user))
        initnet.run_model('lineinfile',
                          'path=/etc/sudoers '
                          'line="{} ALL=(ALL) NOPASSWD: ALL" '
                          'regexp="^{} .*" '
                          'insertafter=EOF '
                          'state=present'
                          .format(self.user, self.user),
                          become=True)
        if demo:
            term.notice('Finished setting up SSH keys.')
        else:
            term.notice('Done!!!')

    def check_hostname(self, facts=None):
        # facts is ansible callback
        _hostname_list = {}
        for _host, _vars in facts['success'].iteritems():
            _hostname = _vars['ansible_facts']['ansible_hostname']
            if _hostname_list.has_key(_hostname):
                _hostname_list[_hostname].append(_host)
            else:
                _hostname_list[_hostname] = [_host]

        # check if have conflict hostname between different host
        _cache_hostname_list = copy.deepcopy(_hostname_list)
        for _host_name, _ip in _hostname_list.iteritems():
            if len(_ip) == 1:
                del _cache_hostname_list[_host_name]

        if _cache_hostname_list:
            term.warn("Some machine\'s hostname conflict.")
            _length = max(max([len(str(x)) for x in _cache_hostname_list.keys()]), len('Hostname'))
            term.normal('Hostname'.ljust(_length + 2) + 'Hosts')
            for _hostname, _hosts in _cache_hostname_list.iteritems():
                term.normal('{}{}'.format(_hostname.ljust(_length + 2), ', '.join(_hosts)))
            if not utils.ticontinue():
                exit(1)

    def check_os_platform(self, facts=None):
        _unsupport_os = []

        # get operation system platform
        for _host, _vars in facts['success'].iteritems():
            _platform = _vars['ansible_facts']['ansible_os_family']
            if 'redhat' == _platform.lower():
                continue
            _unsupport_os.append([_host, _platform])

        if _unsupport_os:
            term.fatal('Some machine\'s OS is not support, Please use Redhat / CentOS.')
            _length = max(max([len(str(x[0])) for x in _unsupport_os]), len('IP'))
            term.normal('IP'.ljust(_length + 2) + 'OS_Family')
            for _node in _unsupport_os:
                term.normal('{}{}'.format(_node[0].ljust(_length + 2), _node[1]))
            exit(1)

    def check_os_version(self, facts=None):
        _lower_version = []

        for _host, _vars in facts['success'].iteritems():
            # get system version
            _sysversion = str(_vars['ansible_facts']['ansible_distribution_version'])
            if _sysversion < '7':
                _lower_version.append([_host, _sysversion])

        if _lower_version:
            term.fatal('Some machine\'s OS version dosen\'t support.')
            _length = max(max([len(str(x[0])) for x in _lower_version]), len('IP'))
            term.normal('IP'.ljust(_length + 2) + 'OS_Version')
            for _node in _lower_version:
                term.normal('{}{}'.format(_node[0].ljust(_length + 2), _node[1]))
            exit(1)

    def check_systemd_version(self, info=None):
        # info: systemd info callback by ansible
        _lower_version = []

        for _host, _vars in info['success'].iteritems():
            for systemd_info in _vars['results']:
                # get sysmted version
                if systemd_info['yumstate'] == 'installed':
                    _version = '{}-{}'.format(systemd_info['version'], systemd_info['release'])
                    # when version less than 219-52.el7, will record
                    if _version < '219-52.el7':
                        _lower_version.append([_host, _version])

        if _lower_version:
            term.warn('Some machine\'s systemd service version lower than "219-52.el7".')
            _length = max(max([len(str(x[0])) for x in _lower_version]), len('IP'))
            term.normal('IP'.ljust(_length + 2) + 'Systemd_Version')
            for _node in _lower_version:
                term.normal('{}{}'.format(_node[0].ljust(_length + 2), _node[1]))
            term.warn('There are some memory bugs in lower systemd version(lower than "219-52.el7"). '
                      'Refer to https://access.redhat.com/discussions/3536621.')
            if not utils.ticontinue():
                exit(1)

    def init_host(self, demo=False):
        term.notice('Begin initializing the cluster machine.')
        self._check_ip_list()
        inithost = ansibleapi.ANSRunner(
            ips=self.hosts, user=self.user, tiargs=self._args)

        inithost.run_model('file',
                           'path=%s '
                           'state=directory '
                           'owner=%s '
                           'group=%s' % (self.check_dir, self.user, self.user),
                           become=True)

        if not os.path.exists(self.host_vars):
            os.mkdir(self.host_vars)
        elif not os.path.isdir(self.host_vars):
            os.remove(self.host_vars)
            os.mkdir(self.host_vars)

        # get host environment vars
        term.info('Get all host environment vars.')
        setup = inithost.run_model('setup',
                                   'gather_subset="all" '
                                   'gather_timeout=120')

        # record host environment vars
        for _host, _vars in setup['success'].iteritems():
            __vars = json.loads(json.dumps(_vars))
            utils.write_yaml(os.path.join(self.host_vars, '{}.yaml'.format(_host)), __vars)

        if self.enable_checks:
            term.info('Check OS platform.')
            self.check_os_platform(setup)

            term.info('Check OS version.')
            self.check_os_version(setup)

            # check cpu if support EPOLLEXCLUSIVE
            term.info('Check if CPU support EPOLLEXCLUSIVE.')
            inithost.run_model('copy',
                               'src=%s/{{ item }} '
                               'dest=%s '
                               'mode=0755' % (self.script_path, self.check_dir),
                               with_items=['epollexclusive', 'run_epoll.sh'])

            check_epoll = inithost.run_model('shell',
                                             '%s/run_epoll.sh' % (self.check_dir))

            _unsupport_epoll = [host for host, info in check_epoll['success'].iteritems() if
                                'True' not in info['stdout']]

            if _unsupport_epoll:
                raise exceptions.TiOPSRuntimeError(
                    'CPU unsupport epollexclusive on {} machine, please change system version or upgrade kernel verion.'.format(
                        ','.join(_unsupport_epoll).replace('_', '.')), operation='init')

            # check systemd service version (systemd package, not operation system)
            _systemd = inithost.run_model('yum',
                                          'list=systemd',
                                          become=True)
            term.info('Check systemd service version.')
            self.check_systemd_version(_systemd)

        if not demo:
            # check and set cpu mode to performance
            term.info('Set CPU performance mode if support.')

            # enable cpupower service
            inithost.run_model('systemd',
                            'name="cpupower"',
                            'state=started',
                            'enabled=yes')

            # set timezone
            if self.timezone:
                inithost.run_model('blockinfile',
                                'path=/home/%s/.bash_profile '
                                'insertbefore="# End of file" '
                                'block="export TZ=%s"' % (self.user, self.timezone))

            # install ntp package
            if self.ntp_server:
                term.info('Install ntpd package.')
                inithost.run_model('yum',
                                'name="ntp" '
                                'state=present',
                                become=True)

                term.info('Add ntp servers to ntpd config and reload.')

                _ntp_config = ['server {} iburst'.format(server) for server in self.ntp_server.split(',')]

                inithost.run_model('blockinfile',
                                'path="/etc/ntp.conf" '
                                'insertbefore="# End of file" '
                                'marker="#{mark} TiDB %s MANAGED BLOCK" '
                                'block="%s"' % (self.user, '\n'.join(_ntp_config)),
                                become=True)

                inithost.run_model('systemd',
                                'name=ntpd '
                                'state=restarted '
                                'enabled=yes',
                                become=True)

            # check if time synchronize
            if self.enable_check_ntp:
                term.info('Check if NTP is running to synchronize the time.')

                result = inithost.run_model('shell',
                                            'ntpstat | grep -w synchronised | wc -l')

                _unrun = [host for host, info in result['success'].iteritems() if info['stderr']]
                if _unrun:
                    raise exceptions.TiOPSRuntimeError(
                        'Ntp server may stopped on {} machine.'.format(
                            ','.join(_unrun).replace('_', '.')), operation='init')

                _unsync = [host for host, info in result['success'].iteritems() if info['stdout'] == str(0)]
                if _unsync:
                    raise exceptions.TiOPSRuntimeError(
                        'Time unsynchronised on {}, please check ntp status.'.format(','.join(_unsync).replace('_', '.')),
                        operation='init')

            term.info('Update kernel parameters on cluster machine.')
            inithost.run_model('sysctl',
                            'name={{ item.name }} value={{ item.value }}',
                            become=True,
                            with_items=[{'name': 'net.ipv4.tcp_tw_recycle', 'value': 0},
                                        {'name': 'net.core.somaxconn',
                                            "value": 32768},
                                        {'name': 'vm.swappiness', "value": 0},
                                        {'name': 'net.ipv4.tcp_syncookies', "value": 0},
                                        {'name': 'fs.file-max', "value": 1000000}])
            inithost.run_model('blockinfile',
                            'path="/etc/security/limits.conf" '
                            'insertbefore="# End of file" '
                            'marker="#{mark} TiDB %s MANAGED BLOCK" '
                            'block="%s        soft        nofile        1000000\n'
                            '%s        hard        nofile        1000000\n'
                            '%s        soft        stack         10240\n"'
                            % (self.user, self.user, self.user, self.user),
                            become=True)

            if not self.enable_swap:
                term.info('Turn off swap on remote machine.')
                inithost.run_model('shell',
                                'swapoff -a',
                                become=True)
            else:
                term.info('Turn on swap on remote machine')
                inithost.run_model('shell',
                                'swapon -a',
                                become=True)

        # disable selinux
        term.info('Disable selinux.')
        inithost.run_model('selinux',
                           'state=disabled',
                           become=True)

        # set and start irqbalance
        if not demo and not self.disable_irqbalance:
            term.info('Set and start irqbalance.')
            inithost.run_model('lineinfile',
                               'dest=/etc/sysconfig/irqbalance '
                               'regexp="(?<!_)ONESHOT=" '
                               'line="ONESHOT=yes"',
                               become=True)
            inithost.run_model('systemd',
                               'name=irqbalance.service '
                               'state=restarted '
                               'enabled=yes',
                               become=True)

        if demo:
            term.notice('Finished init deployment machine.')
        else:
            term.notice('Done!!!')
