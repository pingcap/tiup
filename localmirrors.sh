#!/bin/bash

OFFICICAL_MIRRORS="tiup-mirrors.pingcap.com"

wget $OFFICICAL_MIRRORS/tiup-manifest.index -O tiup-manifest.index

COUNT=$(jq < tiup-manifest.index ".components | length")

echo "COUNT: $COUNT"

for i in $(seq 1 $COUNT)
do
  NAME=$(jq < tiup-manifest.index -r ".components[$(($i-1))] | .name")
  wget "$OFFICICAL_MIRRORS/tiup-component-$NAME.index" -O "tiup-component-$NAME.index"
  VER=$(jq < "tiup-component-$NAME.index" ".versions | length")
  for v in $(seq 1 $VER)
  do
    VERSION=$(jq < "tiup-component-$NAME.index" -r ".versions[$(($v-1))] | .version")
    echo "$NAME => $VERSION"
    wget "$OFFICICAL_MIRRORS/$NAME-$VERSION-darwin-amd64.tar.gz" -O "$NAME-$VERSION-darwin-amd64.tar.gz"
    wget "$OFFICICAL_MIRRORS/$NAME-$VERSION-darwin-amd64.sha1" -O "$NAME-$VERSION-darwin-amd64.sha1"
    wget "$OFFICICAL_MIRRORS/$NAME-$VERSION-linux-amd64.tar.gz" -O "$NAME-$VERSION-linux-amd64.tar.gz"
    wget "$OFFICICAL_MIRRORS/$NAME-$VERSION-linux-amd64.sha1" -O "$NAME-$VERSION-linux-amd64.sha1"
  done
done
