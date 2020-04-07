#!/usr/bin/env bash

VERSION="6.7.1"
DIRECTORY=$(cd $(dirname "$0") && pwd)

echo "==> $DIRECTORY"

for p in "darwin" "linux"
do
  cd "$DIRECTORY"

  wget https://dl.grafana.com/oss/release/grafana-$VERSION.$p-amd64.tar.gz
  tar -zxf grafana-$VERSION.$p-amd64.tar.gz
  cd "grafana-$VERSION"
  tiup package "." --arch amd64 --os "$p" --desc "Grafana is the open source analytics & monitoring solution for every database" --entry "bin/grafana" --name grafana --release "v$VERSION"

  # Update to CDN
  rm -rf ~/.qshell/qupload/
  qshell account $ACCESSKEY $SECRETKEY $QINIUNAME
  qshell qupload2 --src-dir=package/ --bucket=tiup-mirrors --overwrite

  cd "$DIRECTORY"
  rm -rf "grafana-$VERSION"
  rm -f grafana-$VERSION.$p-amd64.tar.gz
done


