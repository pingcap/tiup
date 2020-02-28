#!/bin/bash

for x in tidb tikv pd
do
	for i in {0..10}
	do
		wget https://download.pingcap.org/$x-v3.0.$i-darwin-amd64.tar.gz
		tar -xzf $x-v3.0.$i-darwin-amd64.tar.gz
		tar -C $x-v3.0.$i-darwin-amd64 -czf $x-v3.0.$i-darwin-amd64.tar.gz bin/$x-server
		rm -rf $x-v3.0.$i-darwin-amd64
		sha1sum $x-v3.0.$i-darwin-amd64.tar.gz | awk '{print $1}' > $x-v3.0.$i-darwin-amd64.sha1
	done
done


for i in {0..10}
do
	wget  https://download.pingcap.org/tidb-v3.0.$i-linux-amd64.tar.gz
	tar -xzf  tidb-v3.0.$i-linux-amd64.tar.gz
	for x in tidb tikv pd
	do
		tar -C tidb-v3.0.$i-linux-amd64 -czf  $x-v3.0.$i-linux-amd64.tar.gz bin/$x-server
		sha1sum $x-v3.0.$i-linux-amd64.tar.gz | awk '{print $1}' > $x-v3.0.$i-linux-amd64.sha1
	done
	rm -rf tidb-v3.0.$i-linux-amd64
done