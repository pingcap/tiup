#!/usr/bin/env bash

COMPONENT="alertmanager"
VERSION="0.20.0"
DIRECTORY=$(cd $(dirname "$0") && pwd)

echo "==> $DIRECTORY"

for p in "darwin" "linux"
do
  cd "$DIRECTORY"

  wget https://github.com/prometheus/$COMPONENT/releases/download/v$VERSION/$COMPONENT-$VERSION.$p-amd64.tar.gz
  tar -zxf $COMPONENT-$VERSION.$p-amd64.tar.gz
  mv $COMPONENT-$VERSION.$p-amd64 $COMPONENT
  tiup package "$COMPONENT" --arch amd64 --os "$p" --desc "Prometheus $COMPONENT" --entry "$COMPONENT/$COMPONENT" --name $COMPONENT --release "v$VERSION"

  cd "$DIRECTORY"
  rm -rf "$COMPONENT"
  rm -f $COMPONENT-$VERSION.$p-amd64.tar.gz
done

# Update to CDN
rm -rf ~/.qshell/qupload/
qshell account $ACCESSKEY $SECRETKEY $QINIUNAME
qshell qupload2 --src-dir=package/ --bucket=tiup-mirrors --overwrite

rm -rf package



