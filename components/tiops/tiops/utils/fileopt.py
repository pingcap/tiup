# coding: utf-8
# simple file related utilities

import logging
import os
import re
import shutil
import tarfile
import yaml

from distutils.dir_util import copy_tree

from . import util


# read data from file
def read_file(filename, mode='r'):
    data = None
    with open(filename, mode) as f:
        data = f.read()
    f.close()
    return data


# write data to file
def write_file(filename, data, mode='w'):
    with open(filename, mode) as f:
        logging.debug("Writting %s of data to %s" % (len(data), filename))
        try:
            f.write(str(data, 'utf-8'))
        except (TypeError, UnicodeError):
            f.write(data)


def read_yaml(filename):
    data = None
    with open(filename, 'r') as f:
        data = yaml.safe_load(f)
    return data


def write_yaml(filename, data):
    with open(filename, 'w') as f:
        yaml.safe_dump(data, f, encoding='utf-8', indent=2,
                       default_flow_style=False, allow_unicode=True)
    f.close()


# create target directory, do nothing if it already exists
def create_dir(path):
    try:
        os.makedirs(path)
        return path
    except OSError as e:
        # There is FileExistsError (devided from OSError) in Python 3.3+,
        # but only OSError in Python 2, so we use errno to check if target
        # dir already exists.
        import errno
        if e.errno == errno.EEXIST and os.path.isdir(path):
            logging.info("Target path \"%s\" already exists." % path)
            return path
        else:
            logging.fatal("Can not create dir, error is: %s" % str(e))
            exit(e.errno)
    return None


# remove a directory
def remove_dir(path):
    try:
        shutil.rmtree(path)
    except OSError as e:
        # There is FileExistsError (devided from OSError) in Python 3.3+,
        # but only OSError in Python 2, so we use errno to check if target
        # dir already exists.
        import errno
        if e.errno == errno.ENOENT:
            pass
        else:
            raise


# os.scandir() is added in Python 3.5 and has better performance than os.listdir()
# so we try to use it if available, and fall back to os.listdir() for older versions
def list_dir(path):
    file_list = []
    try:
        if util.python_version(3, 5):
            for entry in os.scandir(path):
                file_list.append("%s/%s" % (path, entry.name))
        else:
            for file in os.listdir(path):
                file_list.append("%s/%s" % (path, file))
    except OSError as e:
        # There is PermissionError in Python 3.3+, but only OSError in Python 2
        import errno
        if e.errno == errno.EACCES or e.errno == errno.EPERM:
            logging.warning("Permission Denied reading %s" % path)
        elif e.errno == errno.ENOENT:
            logging.warning("File not found: %s" % path)
            # ignore for now, the file might be deleted after the initial read
            pass
    return file_list


# list all files under path recusively, use filter to match filenames
def list_files(path, filter=None):
    f_list = []
    for file in list_dir(path):
        if os.path.isdir(file):
            f_list += list_files(file, filter=filter)
        elif not filter:
            f_list.append(file)
        else:
            if filter in file:
                f_list.append(file)
            else:
                pass
    return f_list


# when scaleout, need modify scripts
def script_template(path=None, template=None, service=None):
    current_topology = read_yaml(
        util.profile_path(path, 'topology.yaml'))
    current_profile = read_yaml(
        util.profile_path(path, 'meta.yaml'))
    if current_profile['enable_tls']:
        scheme = 'https'
    else:
        scheme = 'http'
    pd_list = []
    for srv in current_topology['pd_servers']:
        host = srv['ip']
        port = srv['client_port']
        if service in ['pd', 'pump', 'drainer']:
            pd_list.append('{}://{}:{}'.format(scheme, host, port))
        elif service in ['tidb', 'tikv']:
            pd_list.append('{}:{}'.format(host, port))

    with open(template, 'r') as f:
        _original = f.read()

    if service == 'pd':
        new_template = re.sub(r'    --initial-cluster.*',
                              r'    --join="{}" \\'.format(','.join(pd_list)), _original)
    elif service in ['tidb', 'tikv']:
        new_template = re.sub(r'{{ all_pd \| join\(\',\'\) }}', r'{}'.format(
            ','.join(pd_list)), _original)
    elif service in ['pump', 'drainer']:
        new_template = re.sub(r'{{ all_pd \| join\(\',\'\) }}', r'{}'.format(
            ','.join(pd_list)), _original)
    return _original, new_template


def write_template(path=None, info=None):
    with open(path, 'w') as f:
        f.write(info)


def editfile(fpath=None):
    editor = os.environ.get('EDITOR') if os.environ.get('EDITOR') else (
        'vim' if not os.system('which vim &> /dev/null') else 'vi')
    os.system('{} {}'.format(editor, fpath))


def processTidbConfig(fpath=None, sname=None):
    keys = {'pd': ['name =', 'data-dir', 'client-urls', 'peer-urls', 'initial-cluster', '#filename', 'prometheus',
                   r'.*\n.*\n.*\n.*\nlocation-labels'],
            'tidb': ['# TiDB server host', 'host =', 'advertise', '# TiDB server port', 'port =', 'mocktikv',
                     '# TiDB storage path', 'path =', '# Log file name', 'filename =', '# TiDB status host',
                     'status-host =', 'Prometheus', '# TiDB status port', 'status-port',
                     'metrics-addr', 'metrics-interval'],
            'tikv': ['File to store logs', 'logs will be appended to stderr', 'log-file =', 'Listening address',
                     '# addr =', 'Advertise listening address', '`addr` will be used', 'advertise-addr',
                     'Status address', 'status of TiKV', 'Empty string means disabling it', '# status-addr',
                     'The path to RocksDB directory', '# data-dir', 'endpoints', r'.*\n.*labels ='],
            'pump': ['host:port', 'addr =', 'endpoints', 'pd-urls', 'data directory', 'data-dir'],
            'drainer': ['host:port', 'addr =', 'register this addr into etcd', 'endpoints', 'pd-urls', 'data directory',
                        'data-dir =']}

    if sname not in keys:
        return

    if fpath and sname:
        with open(fpath, 'r') as rd:
            _configInfo = rd.read()

        for key in keys[sname]:
            _configInfo = re.sub('.*{}.*?\n'.format(key), '', _configInfo)

        _configInfo = re.sub('\n\n+', '\n', _configInfo)

        with open(fpath, 'w') as wt:
            wt.write(_configInfo)


def copy_template(source=None, target=None):
    for file in os.listdir(source):
        name = os.path.join(source, file)
        back_name = os.path.join(target, file)
        if os.path.isfile(name):
            if os.path.isfile(back_name):
                if util.gen_md5(name) != util.gen_md5(back_name):
                    shutil.copy(name, back_name)
            else:
                shutil.copy(name, back_name)
        else:
            if not os.path.isdir(back_name):
                os.makedirs(back_name)
            copy_template(name, back_name)


def copy_dir(source=None, target=None):
    if not source or not target:
        return
    copy_tree(source, target)


def ansible_config():
    _fpath = os.path.join(util.home(), '.bashrc')
    _config_list = ['ANSIBLE_HOST_KEY_CHECKING=False',
                    'ANSIBLE_PIPELINING=True',
                    'ANSIBLE_CACHE_PLUGIN="jsonfile"',
                    'ANSIBLE_CACHE_PLUGIN_CONNECTION="~/.ansible/ansible_fact_cache"',
                    'ANSIBLE_CACHE_PLUGIN_TIMEOUT=86400',
                    'ANSIBLE_SSH_ARGS="-C -o ControlMaster=auto -o ControlPersist=1d"',
                    'ANSIBLE_GATHER_TIMEOUT=120']

    with open(_fpath, 'r') as f:
        _finfo = f.read()

    for _var in _config_list:
        _re = re.search(r'(ANSIBLE.*?)=(.*)', _var)
        _key = _re.group(1)
        # _value = _re.group(2)
        # os.environ[_key] = _value.strip('"')
        _exist = '{}='.format(_key) in _finfo
        if _exist:
            _finfo = re.sub(r'export +{}=.*'.format(_key),
                            'export {}'.format(_var), _finfo)
        else:
            _finfo += 'export {}\n'.format(_var)

    with open(_fpath, 'w') as f:
        f.write(_finfo)


def untar(fname):
    finfo = tarfile.open(fname)
    finfo.extractall()
