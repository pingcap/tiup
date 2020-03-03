#!/bin/bash

for x in tidb tikv pd
do
	for v in $@
	do
		wget https://download.pingcap.org/$x-$v-darwin-amd64.tar.gz
		tar -xzf $x-$v-darwin-amd64.tar.gz
		tar -C $x-$v-darwin-amd64 -czf $x-$v-darwin-amd64.tar.gz bin/$x-server
		rm -rf $x-$v-darwin-amd64
		sha1sum $x-$v-darwin-amd64.tar.gz | awk '{print $1}' > $x-$v-darwin-amd64.sha1
	done
done

for v in $@
do
	wget  https://download.pingcap.org/tidb-$v-linux-amd64.tar.gz
	tar -xzf  tidb-$v-linux-amd64.tar.gz
	for x in tidb tikv pd
	do
		tar -C tidb-$v-linux-amd64 -czf  $x-$v-linux-amd64.tar.gz bin/$x-server
		sha1sum $x-$v-linux-amd64.tar.gz | awk '{print $1}' > $x-$v-linux-amd64.sha1
	done
	rm -rf tidb-$v-linux-amd64
done