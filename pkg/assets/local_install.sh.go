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

package assets

import "io/ioutil"

// WriteLocalInstallScript writes the install script into specified path
func WriteLocalInstallScript(path string) error {
	return ioutil.WriteFile(path, []byte(script), 0755)
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

install_binary() {
    tar -zxf "tiup-$os-$arch.tar.gz" -C "$bin_dir" || return 1
    return 0
}

if ! install_binary; then
    echo "Failed to download and/or extract tiup archive."
    exit 1
fi

chmod 755 "$bin_dir/tiup"

bold=$(tput bold 2>/dev/null)
sgr0=$(tput sgr0 2>/dev/null)

echo "Detected shell: ${bold}$SHELL${sgr0}"
case $SHELL in
    *bash*) PROFILE=$HOME/.bash_profile;;
     *zsh*) PROFILE=$HOME/.zshrc;;
         *) PROFILE=$HOME/.profile;;
esac

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
echo "Have a try:     ${bold}tiup playground${sgr0}"
echo "==============================================="
`
