#!/usr/bin/env bash

DIR=$(cd $(dirname "$0"); pwd)

checksum() {
  if hash sha1sum 2>/dev/null; then
      sha1sum $1.tar.gz | awk '{print $1}' > $1.sha1
  else
      shasum $1.tar.gz | awk '{print $1}' > $1.sha1
  fi
}

os=$(uname | tr '[:upper:]' '[:lower:]')
rm -f "$DIR/tiup-manifest.index"

# Prepare tarball for `tiup update --self`
mv "$DIR/../tiup_home/bin/tiup.2" "$DIR/tiup"
tar -czf "tiup-$os-amd64.tar.gz" tiup
checksum "tiup-$os-amd64"
rm -f "$DIR/tiup"

# Prepare for package component
mv "$DIR/../tiup_home/bin/package" "$DIR/pack"
TIUP_WORK_DIR="$DIR" ./pack -test.coverprofile="$TEST_DIR/../cover/cov.integration-test.package.out" pack --release=v0.0.1 --entry=pack --arch=amd64 --os="$os" --name=package
mv "./package"/* "./"
# Coverage the manifest exists scenario
TIUP_WORK_DIR="$DIR" ./pack -test.coverprofile="$TEST_DIR/../cover/cov.integration-test.package2.out" pack --release=v0.0.1 --entry=pack --arch=amd64 --os="$os" --name=package
rm pack
mv "./package"/* "./"

# Prepare the mirrors component
mv "$DIR/../tiup_home/bin/mirrors" "$DIR/mirrors"
tiup package -- mirrors --release=v0.0.1 --entry=mirrors --arch=amd64 --os="$os" --name=mirrors --standalone
rm mirrors
mv "./package"/* "./"

# Prepare the doc component
mv "$DIR/../tiup_home/bin/doc" "$DIR/doc"
tiup package -- doc --release=v0.0.1 --entry=doc --arch=amd64 --os="$os" --name=doc --standalone
rm doc
mv "./package"/* "./"

# Prepare the playground component
mv "$DIR/../tiup_home/bin/playground" "$DIR/playground"
tiup package -- playground --release=v0.0.1 --entry=playground --arch=amd64 --os="$os" --name=playground --standalone
rm playground
mv "./package"/* "./"

# Prepare the ctl component
OFFICICAL_MIRRORS="tiup-mirrors.pingcap.com"
VERSION=v3.0.1
wget -nc --quiet "$OFFICICAL_MIRRORS/ctl-$VERSION-$os-amd64.tar.gz"
mkdir -p ctls
tar xf ctl-$VERSION-$os-amd64.tar.gz -C ctls
mv "$DIR/../tiup_home/bin/ctl" "./ctls/ctl"
tiup package -- $(ls ctls) --release=v0.0.1 --entry=ctl --arch=amd64 --os="$os" --name=ctl --standalone -C ctls
rm -rf ./ctls
mv "./package"/* "./"

# Prepare for mock test tarball
for v in "v1.1.1" "v1.1.2" "nightly"
do
  echo "#!/bin/bash" > test.bin
  echo "echo 'integration test $v'" >> test.bin
  chmod +x test.bin
  tiup package -- test.bin --release="$v" --entry=test.bin --arch=amd64 --os="$os" --name=test
done
mv "./package"/* "./"

if [ $GITHUB_ACTION ]; then
  # Prepare playground
  OFFICICAL_MIRRORS="tiup-mirrors.pingcap.com"
  VERSION=v4.0.0-rc
  for NAME in "tidb" "tikv" "pd" "prometheus" "tiflash"
  do
    echo "Download component $NAME, $VERSION"
    wget -nc --quiet "$OFFICICAL_MIRRORS/tiup-component-$NAME.index" -O "tiup-component-$NAME.index"
    wget -nc --quiet "$OFFICICAL_MIRRORS/$NAME-$VERSION-$os-amd64.tar.gz" -O "$NAME-$VERSION-$os-amd64.tar.gz"
    wget -nc --quiet "$OFFICICAL_MIRRORS/$NAME-$VERSION-$os-amd64.sha1" -O "$NAME-$VERSION-$os-amd64.sha1"
  done

  echo $(cat tiup-manifest.index | jq '.components += [{"name": "tidb", "platforms":["darwin/amd64","linux/amd64"]}]') > tiup-manifest.index
  echo $(cat tiup-manifest.index | jq '.components += [{"name": "tikv", "platforms":["darwin/amd64","linux/amd64"]}]') > tiup-manifest.index
  echo $(cat tiup-manifest.index | jq '.components += [{"name": "pd", "platforms":["darwin/amd64","linux/amd64"]}]') > tiup-manifest.index
  echo $(cat tiup-manifest.index | jq '.components += [{"name": "prometheus", "platforms":["darwin/amd64","linux/amd64"]}]') > tiup-manifest.index
  echo $(cat tiup-manifest.index | jq '.components += [{"name": "tiflash", "platforms":["darwin/amd64","linux/amd64"]}]') > tiup-manifest.index
fi
rmdir "./package"
rm -f test.bin

# Mock wrong data file
touch $TIUP_HOME/data/invalid-file
mkdir $TIUP_HOME/data/invalid-dir
chmod -r $TIUP_HOME/data/invalid-dir