#!/bin/bash

checksum() {
  echo "checksum $1"
  if hash sha1sum 2>/dev/null; then
      sha1sum $1.tar.gz | awk '{print $1}' > $1.sha1
  else
      shasum $1.tar.gz | awk '{print $1}' > $1.sha1
  fi
}

for x in tidb tikv pd
do
	for v in $@
	do
		wget https://download.pingcap.org/$x-$v-darwin-amd64.tar.gz
		tar -xzf $x-$v-darwin-amd64.tar.gz
		tar -C $x-$v-darwin-amd64 -czf $x-$v-darwin-amd64.tar.gz bin/$x-server
		rm -rf $x-$v-darwin-amd64
		checksum "$x-$v-darwin-amd64"
	done
done

for v in $@
do
	wget  https://download.pingcap.org/tidb-$v-linux-amd64.tar.gz
	tar -xzf  tidb-$v-linux-amd64.tar.gz
	for x in tidb tikv pd
	do
		tar -C tidb-$v-linux-amd64 -czf  $x-$v-linux-amd64.tar.gz bin/$x-server
		checksum "$x-$v-linux-amd64"
	done
	rm -rf tidb-$v-linux-amd64
done