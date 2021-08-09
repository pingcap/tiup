#!/bin/bash

set -eu

old_version=${old_version-v3.0.20}
version=${version-v4.0.12}

source script/upgrade.sh

echo "test upgrade cluster version from $old_version to $version"
upgrade "$old_version" "$version" false
