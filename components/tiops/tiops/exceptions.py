# coding: utf-8

import re
import sys
import logging
from tiops.tui import term

reload(sys)
sys.setdefaultencoding('utf-8')

class TiOPSException(Exception):
    """Basic exception for errors raised by TiOPS"""

    def __init__(self, msg=None):
        super(TiOPSException, self).__init__(msg)


class TiOPSAttributeError(TiOPSException, AttributeError):
    """Attribute Error of TiOPS"""

    def __init__(self, msg=None):
        if not msg:
            msg = 'Incorrect attribution assignment.'
        super(TiOPSAttributeError, self).__init__(msg)


class TiOPSElementError(TiOPSException, KeyError):
    """Error when accessing one of element of TiOPS objects"""

    def __init__(self, key, msg=None):
        if not msg:
            msg = 'Can not access {}'.format(key)
        super(TiOPSElementError, self).__init__(msg)
        self.key = key


class TiOPSConfigError(TiOPSException, ValueError):
    """Config validation or processing error"""

    def __init__(self, config, msg=None):
        if not msg:
            msg = 'Incorrect config: {}'.format(config)
        super(TiOPSConfigError, self).__init__(msg)
        self.config = config


class TiOPSArgumentError(TiOPSException, ValueError):
    """Exception for unsupported arguments passed to an object"""

    def __init__(self, msg=None):
        if not msg:
            msg = 'Incorrect argument.'
        super(TiOPSArgumentError, self).__init__(msg)


# It does not replace HTTPError
class TiOPSRequestError(TiOPSException):
    """Exception when issuing HTTP requests"""

    def __init__(self, url, code, msg=None):
        if not msg:
            msg = 'Error requesting URL: {}'.format(url)
        super(TiOPSRequestError, self).__init__(msg)
        self.url = url
        self.code = code
        self.msg = msg


class TiOPSRuntimeError(TiOPSException, RuntimeError):
    """Exception when processing operations"""

    def __init__(self, msg=None, ctx=None, component=None, operation=None, tp='tiops'):
        if not msg:
            msg = 'Error processing operation.'
        super(TiOPSRuntimeError, self).__init__(msg)
        self.msg = msg
        self.ctx = ctx
        self.component = component
        self.operation = operation
        self.tp = tp


class TiOPSWarning(UserWarning):
    """Exception that need to take care but not fatal"""

    def __init__(self, msg=None):
        super(TiOPSWarning, self).__init__(msg)


def tierror(msg):
    def ansible_info(precode, info):
        failed = info['failed']
        unreachable = info['unreachable']
        _msg = ''

        if failed:
            for _uuid, _stderr in failed.iteritems():
                _ip = _stderr.keys()[0]
                _err = _stderr.values()[0]
                if 'Failed to stop' in _err and 'service not loaded' in _err:
                    logging.debug('Skipped error: {} {} {}'.format(_uuid, _ip, _err))
                    continue

                _ecode, _resolve = get_ansible_errcode(_err)
                _format_err = '{}{}; {}{}; {}{}; {}{}; {}{}'.format(
                    term.highlight_red('Node ID: '),
                    _uuid,
                    term.highlight_red('Node IP: '),
                    _ip,
                    term.highlight_red('Error Code: '),
                    precode + _ecode,
                    term.highlight_red('Error msg: '),
                    _err[0].replace('\n', ' ') if len(_err) == 1 else _err,
                    term.highlight_red('Reason/Solution: '),
                    _resolve
                )
                if _format_err not in _msg:
                    _msg += '{}\n'.format(_format_err)
        if unreachable:
            for _uuid, _stderr in unreachable.iteritems():
                _ip = _stderr.keys()[0]
                _err = _stderr.values()[0]
                _ecode, _resolve = get_ansible_errcode(_err)
                _format_err = '{}{}; {}{}; {}{}; {}{}; {}{}'.format(
                    term.highlight_red('Node ID: '),
                    _uuid,
                    term.highlight_red('Node IP: '),
                    _ip,
                    term.highlight_red('Error Code: '),
                    precode + _ecode,
                    term.highlight_red('Error msg: '),
                    _err[0].replace('\n', ' ') if len(_err) == 1 else _err,
                    term.highlight_red('Reason/Solution: '),
                    _resolve
                )
                if _format_err not in _msg:
                    _msg += '{}\n'.format(_format_err)

        return _msg

    # error code relation to ansible
    def get_ansible_errcode(info):
        _info = ','.join(info)
        if re.search(r'Failed.*connect.*via ssh', _info, flags=re.IGNORECASE) or re.search(r'host.*reached.*ssh', _info,
                                                                                           flags=re.IGNORECASE):
            return '101', 'ssh key has error permission or use mismatch password'
        elif re.search(r'No such.*file.*or.*directory', _info, flags=re.IGNORECASE):
            return '102', 'file or directory are missing or have wrong permission'
        elif not re.search(r'Failed.*connect.*via ssh', _info, flags=re.IGNORECASE) and re.search(
                r'.*Permission denied.*', _info, flags=re.IGNORECASE):
            return '103', 'file or directory have wrong permission'
        elif re.search(r'Authentication.*permission.*remote.*tmp.*path', _info, flags=re.IGNORECASE):
            return '104', 'Maybe ~/.ansible/tmp have wrong permission'
        elif re.search(r'port.*is not up', _info, flags=re.IGNORECASE):
            return '105', 'Server start failed, need to check log'
        elif re.search(r'port.*may still be up', _info, flags=re.IGNORECASE):
            return '106', 'Server stop failed, maybe other port is listened by other server, need to check port or log'
        elif re.search(r'-1.*not \[200\].*Connection refused', _info, flags=re.IGNORECASE):
            return '107', 'request url failed [-1], please check if the network is restricted or the service is down'
        elif re.search(r'503.*not \[200\].*Service Unavailable', _info, flags=re.IGNORECASE):
            return '108', 'request url failed [503], the service may be in an abnormal state, please check log'
        elif re.search(r'parse cmd flags error', _info, flags=re.IGNORECASE) or re.search(
                r'invalid auto generated configuration file', _info, flags=re.IGNORECASE) or re.search(
            r'time=\".*\" level=fatal msg=.*', _info, flags=re.IGNORECASE):
            return '109', 'there are some misconfigurations in configuration, please re-edit'
        else:
            return '999', ''

    if isinstance(msg.msg, str):
        _msg = msg.msg.lower()
    else:
        _msg = msg.msg
    _component = msg.component
    _operation = msg.operation
    _type = msg.tp

    if _operation == 'init':
        _precode = '10'
    elif _operation == 'download':
        _precode = '11'
    elif _operation == 'deploy':
        _precode = '12'
    elif _operation == 'start':
        _precode = '13'
    elif _operation == 'stop':
        _precode = '14'
    elif _operation == 'restart':
        _precode = '15'
    elif _operation == 'scaleIn':
        _precode = '16'
    elif _operation == 'scaleOut':
        _precode = '17'
    elif _operation == 'reload':
        _precode = '18'
    elif _operation == 'upgrade':
        _precode = '19'
    else:
        _precode = '20'

    output = ''
    if _type == 'tiops':
        if 'not access to the internet network'.lower() in _msg:
            _code = '001'
            output = '{}{}; {}{}; {}{}\n'.format(
                term.highlight_red('Error Code: '),
                _precode + _code,
                term.highlight_red('Error msg: '),
                _msg,
                term.highlight_red('Reason/Solution: '),
                'Need to configure dns or provide internet'
            )
        elif 'Some pd node is unhealthy, maybe server stoppd or network unreachable'.lower() in _msg:
            _code = '002'
            output = '{}{}; {}{}; {}{}\n'.format(
                term.highlight_red('Error Code: '),
                _precode + _code,
                term.highlight_red('Error msg: '),
                _msg,
                term.highlight_red('Reason/Solution: '),
                'check if server is running or network limit'
            )
        elif 'Downgrade is not supported'.lower() in _msg:
            _code = '003'
            output = '{}{}; {}{}; {}{}\n'.format(
                term.highlight_red('Error Code: '),
                _precode + _code,
                term.highlight_red('Error msg: '),
                _msg,
                term.highlight_red('Reason/Solution: '),
                'can not downgrade'
            )
        elif 'Ntp server may stopped'.lower() in _msg:
            _code = '004'
            output = '{}{}; {}{}; {}{}\n'.format(
                term.highlight_red('Error Code: '),
                _precode + _code,
                term.highlight_red('Error msg: '),
                _msg,
                term.highlight_red('Reason/Solution: '),
                'check ntp server status, maybe need reset or start ntp server'
            )
        elif 'Time unsynchronised'.lower() in _msg:
            _code = '005'
            output = '{}{}; {}{}; {}{}\n'.format(
                term.highlight_red('Error Code: '),
                _precode + _code,
                term.highlight_red('Error msg: '),
                _msg,
                term.highlight_red('Reason/Solution: '),
                'wait until time synchronised'
            )
        elif 'Some pd nodes are in false state'.lower() in _msg:
            _code = '006'
            output = '{}{}; {}{}; {}{}\n'.format(
                term.highlight_red('Error Code: '),
                _precode + _code,
                term.highlight_red('Error msg: '),
                _msg,
                term.highlight_red('Reason/Solution: '),
                'check failed node, if service is not started or have some errors'
            )
        else:
            _code = '999'
            output = '{}{}; {}{}\n'.format(
                term.highlight_red('Error Code: '),
                _precode + _code,
                term.highlight_red('Error msg: '),
                _msg
            )

    elif _type == 'ansible':
        output = ansible_info(_precode, _msg)
    else:
        _code = '999'
        output = '{}{}; {}{}\n'.format(
            term.highlight_red('Error Code: '),
            _precode + _code,
            term.highlight_red('Error msg: '),
            _msg
        )

    print output
    sys.exit(1)
