#!/bin/bash

set -eu

old_version=${old_version-v4.0.15}
version=${version-v6.0.0}

source script/upgrade.sh

echo "test upgrade cluster version from $old_version to $version"
upgrade "$old_version" "$version" true
