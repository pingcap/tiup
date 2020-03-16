# coding: utf-8
# simple utilities

import logging
import os
import semver
import sys
import json
import uuid
import hashlib
import ssl
import time
import re

from subprocess import Popen, PIPE

try:
    import fileopt
except ModuleNotFoundError:
    from . import fileopt

try:
    # For Python 2
    import urllib2 as urlreq
    from urllib2 import HTTPError, URLError
except ImportError:
    # For Python 3
    import urllib.request as urlreq
    from urllib.error import HTTPError, URLError


# check if this script is run as root
def is_root_privilege():
    return os.getuid() == 0


# full directory path of this script
def pwd():
    return os.path.dirname(os.path.realpath(__file__))


# full path of current working directory
def cwd():
    return os.getcwd()


# return the home directory of current user
def home():
    if python_version(3, 5):
        from pathlib import Path
        return str(Path.home())
    else:
        return os.path.expanduser("~")


# return the profile directory of current user
def profile_dir():
    dir_path = os.path.join(home(), ".tiops")
    # create the profile dir if it not exists
    # not checking if dir_path exists but is not a dir, do it at access time
    if not os.path.exists(dir_path):
        os.makedirs(dir_path)
    return dir_path


# return the full path of a file or directory in user's profile directory
def profile_path(cluster=None, subpath=None):
    profile = profile_dir()
    if not os.path.isdir(profile):
        import errno
        err = OSError("{} is not a directory".format(profile))
        err.errno = errno.EEXIST
        raise err
    if not cluster or len(cluster) < 1:
        cluster = 'default-cluster'
    if not subpath:
        subpath = ''
    return os.path.join(profile, cluster, subpath)


def chdir(nwd):
    return os.chdir(nwd)


def is_abs_path(path):
    return os.path.isabs(path)


def run_cmd(cmd, shell=False, input=None):
    p = Popen(cmd, shell=shell, stdin=PIPE, stdout=PIPE, stderr=PIPE)
    return p.communicate(input=input)


# check for Python version
def python_version(major, minor):
    return sys.version_info >= (major, minor)


def is_valid_ip(address):
    if python_version(3, 3):
        import ipaddress
        try:
            ipaddress.ip_address(address)
            return True
        except ValueError:
            return False
    else:
        import netaddr
        return netaddr.valid_ipv4(address) or netaddr.valid_ipv6(address)


def gen_uuid(node=None):
    _id = str(uuid.uuid5(uuid.NAMESPACE_DNS, node))[:8]
    if _id[0].isdigit():
        return '{}{}'.format(chr(ord(_id[0]) + 49), _id[1:])
    return _id


def read_url(url, data=None, timeout=10):
    if not url or url == "":
        return None, None

    return request_url(url, data=data, timeout=timeout)


def request_url(url, data=None, method='GET', headers=None, timeout=10):
    if headers:
        request = urlreq.Request(url, data=data, headers=headers)
    else:
        request = urlreq.Request(url, data=data)
    request.get_method = lambda: method
    try:
        logging.debug('Requesting URL: %s' % url)
        response = urlreq.urlopen(request, timeout=timeout)
        return response.read(), response.getcode()
    except HTTPError as e:
        logging.debug('HTTP Error: %s' % e.read())
        return e.read(), e.getcode()
    except (URLError, ssl.SSLError) as e:
        logging.debug('Reading URL %s error: %s' % (url, e))
        return None, None


def find_uuid(uuid=None, config=None):
    _grep_result = {}

    for _uuid in uuid.split(','):
        for grp, servers in config.items():
            i = 0
            for srv in servers:
                if srv['uuid'] == _uuid:
                    try:
                        _grep_result[grp].append(servers[i])
                    except KeyError:
                        _grep_result[grp] = [servers[i]]
    return _grep_result if len(_grep_result) > 0 else None


def valid_tidb_ver(ver):
    verstr = '{}'.format(ver).lstrip('v')  # convert to semver format
    try:
        # we only support TiDB version 3.0.x for now
        if semver.compare(ver, '3.0.0') < 0 or semver.compare(ver, '3.1.0') >= 0:
            return None
        # store vMAJOR.MINOR.PATCH version string
        return 'v{}'.format(verstr)
    except:
        return None


def gen_md5(fpath=None):
    with open(fpath, 'r') as f:
        _data = f.read()
    file_md5 = hashlib.md5(_data).hexdigest()
    return file_md5


def wait(seconds=5):
    time.sleep(seconds)


def format_labels(label=None):
    if not label:
        return 'None', 'None'
    _pd_label = map(lambda x: x.split('=')[0], filter(None, map(lambda x: re.sub(r' ', '', x), label.split(','))))
    _tikv_label = dict(map(lambda n: n.split('='), [x for x in map(lambda y: re.sub(r' ', '', y), label.split(','))]))
    return ','.join(_pd_label), json.dumps(_tikv_label)


def ticontinue():
    _choice = '[\033[1;32;40mY\033[0m(es) or \033[1;32;40mN\033[0m(o)]'
    while True:
        _input = raw_input('Do you want to continue {}: '.format(_choice))
        if _input.lower() not in ['y', 'n', 'yes', 'no']:
            print('Invalid input, please enter [\033[1;32;40mY\033[0m(es) or \033[1;32;40mN\033[0m(o)].')
        else:
            break
    if _input.lower() in ['n', 'no']:
        return False
    elif _input.lower() in ['y', 'yes']:
        return True
