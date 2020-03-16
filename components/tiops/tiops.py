#!/usr/bin/env python2.7
# coding: utf-8

import sys
import logging
import traceback

from tiops import operations as op

from tiops import cmd
from tiops import init
from tiops import topology
from tiops import utils
from tiops import TiOPSVer

from tiops.exceptions import TiOPSException
from tiops.exceptions import TiOPSRequestError
from tiops.exceptions import TiOPSRuntimeError
from tiops.exceptions import tierror
from tiops.operations.action import Action
from tiops.tui import term
from tiops.tui import modules as tm


def main(args=None):
    try:
        action = args.action
    except AttributeError:
        pass

    if action == 'version':
        print(term.plain(TiOPSVer()))
        exit(0)

    if action == 'quickdeploy':
        term.warn('The quick deploy mode is for demo and testing, do NOT use in production!')

        # do init
        _init = init.Init(args)
        try:
            _init.init(demo=True)
            _init.init_network(demo=True)
            _init.init_host(demo=True)
        except TiOPSRuntimeError as e:
            tierror(e)
        except TiOPSException:
            term.error(traceback.format_exc())
            sys.exit(1)

        # do deploy
        topo = topology.Topology(args=args, merge=True)
        try:
            op.OprDeploy(args, topo, demo=True).do()
            op.OprStart(args, topo, demo=True).do()
            tm.TUIModule(topo, args=args).display()
        except TiOPSRuntimeError as e:
            tierror(e)
        except TiOPSRequestError as e:
            msg = "{}, URL {} returned {}, please check the network and try again.".format(
                e.msg, e.url, e.code)
            term.error(msg)
            sys.exit(1)
        except TiOPSException:
            term.error(traceback.format_exc())
            sys.exit(1)

    elif action == 'bootstrap-local':
        _init = init.Init(args)
        try:
            _init.init()
        except TiOPSRuntimeError as e:
            tierror(e)
        except TiOPSException:
            term.error(traceback.format_exc())
            sys.exit(1)
    elif action == 'bootstrap-ssh':
        _init = init.Init(args)
        try:
            _init.init_network()
        except TiOPSRuntimeError as e:
            tierror(e)
        except TiOPSException:
            term.error(traceback.format_exc())
            sys.exit(1)
    elif action == 'bootstrap-host':
        _init = init.Init(args)
        try:
            _init.init_host()
        except TiOPSRuntimeError as e:
            tierror(e)
        except TiOPSException:
            term.error(traceback.format_exc())
            sys.exit(1)
    else:
        try:
            if action not in ['deploy', 'display']:
                topo = topology.Topology(args)
        except TiOPSRuntimeError as e:
            tierror(e)
        except TiOPSException:
            term.error(traceback.format_exc())
            sys.exit(1)

        if action == 'display':
            try:
                _cluster_name = args.cluster_name
            except AttributeError:
                _cluster_name = None
            try:
                if _cluster_name and len(_cluster_name) > 0:
                    topo = topology.Topology(args)
                    _list = False
                else:
                    topo = None
                    _list = True
                tm.TUIModule(topo, args=args).display(_list)
            except TiOPSRuntimeError as e:
                tierror(e)
            except TiOPSException:
                term.error(traceback.format_exc())
                sys.exit(1)
        elif action == 'deploy':
            topo = topology.Topology(args=args, merge=True)
            try:
                op.OprDeploy(args, topo).do()
                tm.TUIModule(topo, args=args).display()
            except TiOPSRuntimeError as e:
                tierror(e)
            except TiOPSRequestError as e:
                msg = "{}, URL {} returned {}, please check the network and try again.".format(
                    e.msg, e.url, e.code)
                term.error(msg)
                sys.exit(1)
            except TiOPSException:
                term.error(traceback.format_exc())
                sys.exit(1)
        elif action == 'start':
            try:
                op.OprStart(args, topo).do(node=args.node_id, role=args.role)
                tm.TUIModule(topo, args=args, status=True).display()
            except TiOPSRuntimeError as e:
                tierror(e)
            except TiOPSException:
                term.error(traceback.format_exc())
                sys.exit(1)
        elif action == 'stop':
            try:
                op.OprStop(args, topo).do(node=args.node_id, role=args.role)
            except TiOPSRuntimeError as e:
                tierror(e)
            except TiOPSException:
                term.error(traceback.format_exc())
                sys.exit(1)
        elif action == 'restart':
            try:
                op.OprRestart(args, topo).do(node=args.node_id, role=args.role)
            except TiOPSRuntimeError as e:
                tierror(e)
            except TiOPSException:
                term.error(traceback.format_exc())
                sys.exit(1)
        elif action == 'reload':
            try:
                op.OprReload(args, topo).do(node=args.node_id, role=args.role)
            except TiOPSRuntimeError as e:
                tierror(e)
            except TiOPSException:
                term.error(traceback.format_exc())
                sys.exit(1)
        elif action == 'upgrade':
            try:
                op.OprUpgrade(args, topo).do(node=args.node_id, role=args.role)
            except TiOPSRuntimeError as e:
                tierror(e)
            except TiOPSRequestError as e:
                msg = "{}, URL {} returned {}, please check the network and try again.".format(
                    e.msg, e.url, e.code)
                term.error(msg)
                sys.exit(1)
            except TiOPSException:
                term.error(traceback.format_exc())
                sys.exit(1)
        elif action == 'destroy':
            try:
                op.OprDestroy(args, topo).do()
            except TiOPSRuntimeError as e:
                tierror(e)
            except TiOPSException:
                term.error(traceback.format_exc())
                sys.exit(1)
        elif action == 'edit-config':
            try:
                Action(topo=topo).edit_file()
            except TiOPSRuntimeError as e:
                tierror(e)
            except TiOPSException:
                term.error(traceback.format_exc())
                sys.exit(1)
        elif action == 'scale-out':
            addTopo = utils.read_yaml(args.topology)
            try:
                op.OprScaleOut(args, topo, addTopo).do()
            except TiOPSRuntimeError as e:
                tierror(e)
            except TiOPSException:
                term.error(traceback.format_exc())
                sys.exit(1)
        elif action == 'scale-in':
            try:
                op.OprScaleIn(args, topo, args.node_id).do(node=args.node_id)
            except TiOPSRuntimeError as e:
                tierror(e)
            except TiOPSException:
                term.error(traceback.format_exc())
                sys.exit(1)
        elif action == 'exec':
            try:
                op.OprExec(args, topo).do(node=args.node_id, role=args.role)
            except TiOPSRuntimeError as e:
                tierror(e)
            except TiOPSException:
                term.error(traceback.format_exc())
                sys.exit(1)


if __name__ == '__main__':
    _parser = cmd.TiOPSParser()
    args = _parser()

    # add logging facilities, but outputs are not modified to use it yet
    if args.verbose:
        logging.basicConfig(filename=utils.profile_path("tiops.log"),
                            format='[%(asctime)s] [%(levelname)s] %(message)s (at %(filename)s:%(lineno)d in %(funcName)s).',
                            datefmt='%Y-%m-%d %T %z', level=logging.DEBUG)
        logging.info("Using logging level: DEBUG.")
        logging.debug("Debug logging enabled.")
        logging.debug("Input arguments are: %s" % args)
    else:
        logging.basicConfig(filename=utils.profile_path("tiops.log"),
                            format='[%(asctime)s] [%(levelname)s] %(message)s.',
                            datefmt='%Y-%m-%d %T %z', level=logging.INFO)
        logging.info("Using logging level: INFO.")

    try:
        main(args)
    except KeyboardInterrupt:
        # not auto cleaning up because the operation may be un-defined
        term.notice(
            'Process interrupted, make sure to check if configs are correct.')
