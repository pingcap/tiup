#!/usr/bin/env bash

checksum() {
  if hash sha1sum 2>/dev/null; then
      sha1sum $1.tar.gz | awk '{print $1}' > $1.sha1
  else
      shasum $1.tar.gz | awk '{print $1}' > $1.sha1
  fi
}

for v in "v1.1.1" "v1.1.2" "nightly"
do
  echo "#!/bin/bash" > test.bin
  echo "echo 'integration test $v'" >> test.bin
  chmod +x test.bin
  for os in "linux" "darwin"
  do
    tar -czf "test-$v-$os-amd64.tar.gz" test.bin
    checksum "test-$v-$os-amd64"
  done
done

rm test.bin