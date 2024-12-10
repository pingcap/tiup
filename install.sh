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

check_depends() {
    pass=0
    command -v curl >/dev/null || {
        echo "Dependency check failed: please install 'curl' before proceeding."
        pass=1
    }
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

"$bin_dir/tiup" mirror set $repo --silent

bold=$(tput bold 2>/dev/null)
green=$(tput setaf 2 2>/dev/null)
cyan=$(tput setaf 6 2>/dev/null)
sgr0=$(tput sgr0 2>/dev/null)

echo

# Refrence: https://stackoverflow.com/questions/14637979/how-to-permanently-set-path-on-linux-unix
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
echo
echo "${bold}${green}✔ ${sgr0}Installed in ${bold}$bin_dir/tiup${sgr0}"
case :$PATH: in
    *:$bin_dir:*) echo "${bold}${green}✔ ${sgr0}tiup PATH is already set, skip" ;;
    *) printf '\nexport PATH=%s:$PATH\n' "$bin_dir" >> "$PROFILE"
        echo "${bold}${green}✔ ${sgr0}Added tiup PATH into ${bold}${shell}${sgr0} profile"
        ;;
esac
echo
echo "${bold}tiup is installed now${sgr0} 🎉"
echo
echo Next step:
echo
echo "  1: To make PATH change effective, restart your shell or execute:"
echo "     ${bold}${cyan}source ${PROFILE}${sgr0}"
echo
echo "  2: Start a local TiDB for development:"
echo "     ${bold}${cyan}tiup playground${sgr0}"
