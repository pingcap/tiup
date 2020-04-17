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

# Prepare for mock test tarball
for v in "v1.1.1" "v1.1.2" "nightly"
do
  echo "#!/bin/bash" > test.bin
  echo "echo 'integration test $v'" >> test.bin
  chmod +x test.bin
  tiup package -- test.bin --release="$v" --entry=test.bin --arch=amd64 --os="$os" --name=test
done
mv "./package"/* "./"

rmdir "./package"
rm -f test.bin