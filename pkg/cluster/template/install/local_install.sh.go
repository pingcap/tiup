// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package install

import "github.com/pingcap/tiup/pkg/utils"

// WriteLocalInstallScript writes the install script into specified path
func WriteLocalInstallScript(path string) error {
	return utils.WriteFile(path, []byte(script), 0755)
}

var script = `#!/bin/sh

case $(uname -s) in
    Linux|linux) os=linux ;;
    Darwin|darwin) os=darwin ;;
    *) os= ;;
esac

if [ -z "$os" ]; then
    echo "OS $(uname -s) not supported." >&2
    exit 1
fi

case $(uname -m) in
    amd64|x86_64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) arch= ;;
esac

if [ -z "$arch" ]; then
    echo "Architecture  $(uname -m) not supported." >&2
    exit 1
fi

if [ -z "$TIUP_HOME" ]; then
    TIUP_HOME=$HOME/.tiup
fi
bin_dir=$TIUP_HOME/bin
mkdir -p "$bin_dir"

script_dir=$(cd $(dirname $0) && pwd)

install_binary() {
  tar -zxf "$script_dir/tiup-$os-$arch.tar.gz" -C "$bin_dir" || return 1
  # Use the offline root.json
  cp "$script_dir/root.json" "$bin_dir" || return 1
  # Remove old manifests
  rm -rf $TIUP_HOME/manifests
  return 0
}

check_depends() {
    pass=0
    command -v tar >/dev/null || {
        echo "Dependency check failed: please install 'tar' before proceeding."
        pass=1
    }
    return $pass
}

if ! check_depends; then
    exit 1
fi

if ! install_binary; then
    echo "Failed to download and/or extract tiup archive."
    exit 1
fi

chmod 755 "$bin_dir/tiup"

# telemetry is not needed for offline installations
"$bin_dir/tiup" telemetry disable

# set mirror to the local path
"$bin_dir/tiup" mirror set ${script_dir}

bold=$(tput bold 2>/dev/null)
sgr0=$(tput sgr0 2>/dev/null)

# Reference: https://stackoverflow.com/questions/14637979/how-to-permanently-set-path-on-linux-unix
shell=$(echo $SHELL | awk 'BEGIN {FS="/";} { print $NF }')
echo "Detected shell: ${bold}$shell${sgr0}"
if [ -f "${HOME}/.${shell}_profile" ]; then
    PROFILE=${HOME}/.${shell}_profile
elif [ -f "${HOME}/.${shell}_login" ]; then
    PROFILE=${HOME}/.${shell}_login
elif [ -f "${HOME}/.${shell}rc" ]; then
    PROFILE=${HOME}/.${shell}rc
else
    PROFILE=${HOME}/.profile
fi
echo "Shell profile:  ${bold}$PROFILE${sgr0}"

case :$PATH: in
    *:$bin_dir:*) : "PATH already contains $bin_dir" ;;
    *) printf 'export PATH=%s:$PATH\n' "$bin_dir" >> "$PROFILE"
        echo "$PROFILE has been modified to to add tiup to PATH"
        echo "open a new terminal or ${bold}source ${PROFILE}${sgr0} to use it"
        ;;
esac

echo "Installed path: ${bold}$bin_dir/tiup${sgr0}"
echo "==============================================="
echo "1. ${bold}source ${PROFILE}${sgr0}"
echo "2. Have a try:   ${bold}tiup playground${sgr0}"
echo "==============================================="
`
