#!/bin/sh

repo='https://tiup-mirrors.pingcap.com'
if [ -n "$TIUP_MIRRORS" ]; then
    repo=$TIUP_MIRRORS
fi

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

install_binary() {
    curl "$repo/tiup-$os-$arch.tar.gz?$(date "+%Y%m%d%H%M%S")" -o "/tmp/tiup-$os-$arch.tar.gz" || return 1
    tar -zxf "/tmp/tiup-$os-$arch.tar.gz" -C "$bin_dir" || return 1
    rm "/tmp/tiup-$os-$arch.tar.gz"
    return 0
}

if ! install_binary; then
    echo "Failed to download and/or extract tiup archive."
    exit 1
fi

chmod 755 "$bin_dir/tiup"

"$bin_dir/tiup" mirror set $repo

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
