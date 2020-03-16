# coding: utf-8

from __future__ import (absolute_import, division, print_function)

__metaclass__ = type
import os


def cluster_dir(path):
    # while count:
    #     path = os.path.split(path)[0]
    #     count -= 1
    return os.path.split(path)[0]


class FilterModule(object):
    def filters(self):
        return {
            'cluster_dir': cluster_dir
        }