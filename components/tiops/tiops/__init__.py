# coding: utf-8

from .exceptions import *

# define tiops version number
TiOPSVerMajor = 0
TiOPSVerMinor = 2
TiOPSVerPatch = 0

class TiOPSVersion(object):
    def __init__(self):
        self.major = TiOPSVerMajor
        self.minor = TiOPSVerMinor
        self.patch = TiOPSVerPatch

    def __call__(self):
        return 'v{}.{}.{}'.format(
            self.major,
            self.minor,
            self.patch
        )

TiOPSVer = TiOPSVersion()

