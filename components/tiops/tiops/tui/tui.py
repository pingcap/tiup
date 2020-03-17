# coding: utf-8

import getpass
import logging
import six

from colorama import deinit
from colorama import init
from colorama import Back
from colorama import Fore
from colorama import Style


# log everything that has printed to terminal
def strip_newline(msg):
    # strip_newline replaces newline '\n' with "\n" string and format the multiline
    # text to a long one-line string, in order not to break log format
    return "\\n".join(msg.split('\n'))


def log_debug(func):
    def log(obj, msg):
        logging.debug(strip_newline(msg))
        if logging.getLogger().getEffectiveLevel() > logging.DEBUG:
            return
        return func(obj, msg)
    return log


def log_info(func):
    def log(obj, msg):
        logging.info(strip_newline(msg))
        if logging.getLogger().getEffectiveLevel() > logging.INFO:
            return
        return func(obj, msg)
    return log


def log_warn(func):
    def log(obj, msg):
        logging.warn(strip_newline(msg))
        if logging.getLogger().getEffectiveLevel() > logging.WARNING:
            return
        return func(obj, msg)
    return log


def log_error(func):
    def log(obj, msg):
        logging.error(strip_newline(msg))
        if logging.getLogger().getEffectiveLevel() > logging.ERROR:
            return
        return func(obj, msg)
    return log


def log_fatal(func):
    def log(obj, msg):
        logging.fatal(strip_newline(msg))
        if logging.getLogger().getEffectiveLevel() > logging.CRITICAL:
            return
        return func(obj, msg)
    return log


def autoreset_style(func):
    def reset(obj, msg=None):
        msg += Style.RESET_ALL
        return func(obj, msg)
    return reset


class TUI(object):
    # some extra style definitions
    class style:
        BLINK = '\033[5m'
        BOLD = '\033[1m'
        UNDERLINE = '\033[4m'

    def __init__(self):
        init(autoreset=True)

    # output functions
    # wrap by usages
    @log_debug
    def debug(self, msg):
        print(self.plain_magenta(msg))

    @log_info
    def normal(self, msg):
        print(self.plain(msg))

    @log_info
    def ok(self, msg):
        print(self.plain_green(msg))

    @log_info
    def info(self, msg):
        print(self.plain_cyan(msg))

    @log_info
    def notice(self, msg):
        print(self.bold_cyan(msg))

    @log_warn
    def warn(self, msg):
        print(self.plain_yellow(msg))

    @log_error
    def error(self, msg):
        print(self.plain_red(msg))

    @log_fatal
    def fatal(self, msg):
        print(self.bold_red(msg))

    @log_info
    def title(self, msg):
        print(self.highlight_white(msg))

    # wrap by styles
    @autoreset_style
    def plain(self, msg):
        return msg

    @autoreset_style
    def plain_blue(self, msg):
        return Fore.BLUE + msg

    @autoreset_style
    def plain_cyan(self, msg):
        return Fore.CYAN + msg

    @autoreset_style
    def plain_green(self, msg):
        return Fore.GREEN + msg

    @autoreset_style
    def plain_red(self, msg):
        return Fore.RED + msg

    @autoreset_style
    def plain_yellow(self, msg):
        return Fore.YELLOW + msg

    @autoreset_style
    def plain_magenta(self, msg):
        return Fore.MAGENTA + msg

    @autoreset_style
    def bold_cyan(self, msg):
        return self.style.BOLD + Fore.CYAN + msg

    @autoreset_style
    def bold_red(self, msg):
        return self.style.BOLD + Fore.RED + msg

    @autoreset_style
    def bold_yellow(self, msg):
        return self.style.BOLD + Fore.YELLOW + msg

    @autoreset_style
    def highlight_cyan(self, msg):
        return Back.BLACK + Fore.CYAN + msg

    @autoreset_style
    def highlight_green(self, msg):
        return Back.BLACK + Fore.GREEN + msg

    @autoreset_style
    def highlight_red(self, msg):
        return Back.BLACK + Fore.RED + msg

    @autoreset_style
    def highlight_white(self, msg):
        return Back.WHITE + Fore.BLACK + self.style.UNDERLINE + msg

    @autoreset_style
    def warn_red(self, msg):
        return Back.BLACK + Fore.RED + self.style.BOLD + msg

    def yes_no(self):
        return Style.RESET_ALL + '[(' \
            + self.highlight_green('Y') + Style.RESET_ALL \
            + ')es/(' \
            + self.highlight_green('N') + Style.RESET_ALL \
            + ')o]' + Style.RESET_ALL

    # input functions
    def input(self, prompt):
        return six.moves.input(prompt)

    def getpass(self, prompt='Password: '):
        return getpass.getpass(prompt=prompt)
