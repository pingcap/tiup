#! /bin/sh

repo='https://tiup-mirrors.pingcap.com'
os=$(uname | tr '[:upper:]' '[:lower:]')

TIUP_HOME=${HOME}/.tiup
mkdir -p ${TIUP_HOME}/bin/
curl ${repo}/tiup-${os}-amd64.tar.gz | tar x -C ${TIUP_HOME}/bin/

chmod 755 ${TIUP_HOME}/bin/tiup

PROFILE=${HOME}/.profile
if echo "$SHELL" | grep -Eq "bash"
then
    PROFILE=${HOME}/.bash_profile
fi
if echo "$SHELL" | grep -Eq "zsh"
then
    PROFILE=${HOME}/.zshrc
fi

cat >> ${PROFILE} << EOF

export PATH=${TIUP_HOME}/bin:\${PATH}
EOF

source ${HOME}/.bash_profile

echo "tiup is installed in ${TIUP_HOME}/bin/tiup"
echo "we have modify ${PROFILE} to add tiup to PATH"
echo "you can open a new terminal or source ${PROFILE} to use it"