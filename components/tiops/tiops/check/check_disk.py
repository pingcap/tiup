#!/usr/bin/env python2.7
# coding: utf-8


import json
import argparse
from os import system, popen, remove
from os.path import exists


def parse_opts():
    parser = argparse.ArgumentParser(description="Check disk file system")
    parser.add_argument("--filesystem", dest='filesystem', action="store_true", default=False,
                        help="check file system")
    parser.add_argument("--read-iops", dest='read_iops', action="store_true", default=False,
                        help="fio read IOPS.")
    parser.add_argument("--write-iops", dest='write_iops', action="store_true", default=False,
                        help="fio write IOPS.")
    parser.add_argument("--latency", dest='latency', action="store_true", default=False,
                        help="fio latency. (ns)")
    return parser.parse_args()


def run_shell(cmd, info=False):
    re_code = system('{} &> /dev/null'.format(cmd))
    if re_code:
        return 'error: "{}" run error.'.format(cmd)
    if info:
        return popen(cmd).read().split('\n')


class CheckFilesystem(object):
    def __init__(self, path):
        self.path = path
        self.filename = 'tidb_test'

    def check_ext4(self):
        current_mount_point = run_shell("df -T %s | tail -1 | awk '{print $NF}'", info=True)[0]
        mount_infos = run_shell('mount')
        mount_infos_format = [x.split(' ') for x in mount_infos]
        for mount_info in mount_infos_format:
            if mount_info[2] == current_mount_point:
                mount_parameter = mount_info[-1]

                if 'nodelalloc' not in mount_parameter:
                    return 'warn: fstype is ext4, but nodelalloc is not set.'
                else:
                    return 'info: mount parameters check successful.'

    def check_xfs(self):
        if exists(self.filename):
            remove(self.filename)
        run_shell('fallocate -n -o 0 -l 9192 {}'.format(self.filename))
        run_shell('printf "a%.0s" {1..5000} > ' + self.filename)
        run_shell('truncate -s 5000 {}'.format(self.filename))
        run_shell('fallocate -p -n -o 5000 -l 4192 {}'.format(self.filename))
        re_info = run_shell("LANG=en_US.UTF-8  stat %s |awk 'NR==2{print $2}'" % self.filename, info=True)[0]
        if int(re_info) != 5000:
            return 'fatal: after testing, there is a bug in this xfs file system, it is not recommended.'
        else:
            return 'info: xfs file system check successful.'

    def check_filesystem(self):
        fstype = run_shell("df -T %s | tail -1 | awk '{print $2}'" % self.path, info=True)[0]
        if fstype not in ['ext4', 'xfs']:
            return 'fatal: "{}" files ystem is not support.'.format(fstype)
        elif fstype == 'ext4':
            return self.check_ext4()
        elif fstype == 'xfs':
            return self.check_xfs()


class CheckIOPS(object):
    def __init__(self):
        pass

    def fio_randread(self):
        if exists('./fio_randread_test.txt'):
            remove('./fio_randread_test.txt')
        if exists('./fio_randread_result.json'):
            remove('./fio_randread_result.json')
        cmd = "./fio -ioengine=psync -bs=32k -fdatasync=1 -thread -rw=randread -size=10G " \
              "-filename=fio_randread_test.txt -name='fio randread test' -iodepth=4 -runtime=60 -numjobs=4 " \
              "-group_reporting --output-format=json --output=fio_randread_result.json"
        run_shell(cmd)
        with open('./fio_randread_result.json', 'r') as f:
            result = json.load(f)
        result_read = result['jobs'][0]['read']
        read_iops = int(result_read['iops'])
        return read_iops

    def fio_readread_write(self):
        if exists('./fio_randread_write_test.txt'):
            remove('./fio_randread_write_test.txt')
        if exists('./fio_randread_write_test.json'):
            remove('./fio_randread_write_test.json')
        cmd = "./fio -ioengine=psync -bs=32k -fdatasync=1 -thread -rw=randrw -percentage_random=100,0 " \
              "-size=10G -filename=fio_randread_write_test.txt -name='fio mixed randread and sequential write test' " \
              "-iodepth=4 -runtime=60 -numjobs=4 -group_reporting --output-format=json " \
              "--output=fio_randread_write_test.json"
        run_shell(cmd)
        with open('fio_randread_write_test.json', 'r') as f:
            result = json.load(f)
        result_read = result['jobs'][0]['read']
        read_iops = int(result_read['iops'])
        result_write = result['jobs'][0]['write']
        write_iops = int(result_write['iops'])
        return {'mix_read_iops': read_iops, 'mix_write_iops': write_iops}

    def fio_randread_write_latency(self):
        if exists('./fio_randread_write_latency_test.txt'):
            remove('./fio_randread_write_latency_test.txt')
        if exists('./fio_randread_write_latency_test.json'):
            remove('./fio_randread_write_latency_test.json')
        cmd = "./fio -ioengine=psync -bs=32k -fdatasync=1 -thread -rw=randrw " \
              "-percentage_random=100,0 -size=10G -filename=fio_randread_write_latency_test.txt " \
              "-name='fio mixed randread and sequential write test' -iodepth=1 -runtime=60 " \
              "-numjobs=1 -group_reporting --output-format=json --output=fio_randread_write_latency_test.json"
        run_shell(cmd)
        with open('./fio_randread_write_latency_test.json', 'r') as f:
            result = json.load(f)
        result_read = result['jobs'][0]['read']
        read_lag_ns = result_read['lat_ns']
        read_lag_ns_avg = int(read_lag_ns['mean'])

        result_write = result['jobs'][0]['write']
        write_lag_ns = result_write['lat_ns']
        write_lag_ns_avg = int(write_lag_ns['mean'])

        return {'mix_read_latency': read_lag_ns_avg, 'mix_write_latency': write_lag_ns_avg}


if __name__ == '__main__':
    args = parse_opts()
    result = {}

    if args.filesystem:
        _filesystem = CheckFilesystem('.')
        _file_info = _filesystem.check_filesystem()
        result['filesystem'] = _file_info

    if args.read_iops or args.write_iops or args.latency:
        _iops = CheckIOPS()

        if args.read_iops:
            result['read_iops'] = _iops.fio_randread()

        if args.write_iops:
            _iops_result = _iops.fio_readread_write()
            result['mix_read_iops'] = _iops_result['mix_read_iops']
            result['mix_write_iops'] = _iops_result['mix_write_iops']

        if args.latency:
            _latency = _iops.fio_randread_write_latency()
            result['mix_read_latency'] = _latency['mix_read_latency']
            result['mix_write_latency'] = _latency['mix_write_latency']

    print(result)
