#!/usr/bin/env bash

VERSION="2.17.1"
DIRECTORY=$(cd $(dirname "$0") && pwd)

echo "==> $DIRECTORY"

for p in "darwin" "linux"
do
  cd "$DIRECTORY"

  wget https://github.com/prometheus/prometheus/releases/download/v$VERSION/prometheus-$VERSION.$p-amd64.tar.gz
  tar -zxf prometheus-$VERSION.$p-amd64.tar.gz
  mv prometheus-$VERSION.$p-amd64 prometheus
  tiup package "prometheus" --arch amd64 --os "$p" --desc "The Prometheus monitoring system and time series database." --entry "prometheus/prometheus" --name prometheus --release "v$VERSION"

  cd "$DIRECTORY"
  rm -rf "prometheus"
  rm -f prometheus-$VERSION.$p-amd64.tar.gz
done

# Update to CDN
rm -rf ~/.qshell/qupload/
qshell account $ACCESSKEY $SECRETKEY $QINIUNAME
qshell qupload2 --src-dir=package/ --bucket=tiup-mirrors --overwrite

rm -rf package



