#!/bin/bash

for x in tidb tikv pd
do
	for i in {0..10}
	do
		wget  http://download.pingcap.org/$x-v3.0.$i-darwin-amd64.tar.gz
		sha1sum $x-v3.0.$i-darwin-amd64.tar.gz | awk '{print $1}' > $x-v3.0.$i-darwin-amd64.sha1
	done
done


for i in {0..10}
do
	wget  http://download.pingcap.org/tidb-v3.0.$i-linux-amd64.tar.gz
	tar -xzf  tidb-v3.0.$i-linux-amd64.tar.gz
	for x in tidb tikv pd
	do
		tar -czf  $x-v3.0.$i-linux-amd64.tar.gz tidb-v3.0.$i-linux-amd64/bin/$x-server
		sha1sum $x-v3.0.$i-linux-amd64.tar.gz | awk '{print $1}' > $x-v3.0.$i-linux-amd64.sha1
	done
	rm -rf tidb-v3.0.$i-linux-amd64
done
