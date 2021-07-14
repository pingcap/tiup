#!/bin/bash
set -eu

version=${version-v4.0.12}
old_version=${old_version-v3.0.20}

source script/upgrade.sh

echo "test upgrade from $old_version to $version TLS, via easy ssh"
upgrade "$old_version" "$version" true false true
