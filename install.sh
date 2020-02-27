#! /bin/sh

repo='http://tidb.tnthub.com:81/mirror'
os=$(uname | tr '[:upper:]' '[:lower:]')

TIUP_HOME=${HOME}/.tiup
mkdir -p ${TIUP_HOME}/bin/
curl ${repo}/tiup-${os} -o ${TIUP_HOME}/bin/tiup

chmod 755 ${TIUP_HOME}/bin/tiup

PROFILE=${HOME}/.profile
if echo "$SHELL" | grep -Eq "bash"
then
    PROFILE=${HOME}/.bash_profile
fi
if echo "$SHELL" | grep -Eq "zsh"
then
    PROFILE=${HOME}/.zsh_profile
fi

cat >> ${PROFILE} << EOF

export PATH=${TIUP_HOME}/bin:\${PATH}
EOF

source ${HOME}/.bash_profile

echo "tiup is installed in ${TIUP_HOME}/bin/tiup"
echo "we have modify ${PROFILE} to add tiup to PATH"
echo "you can open a new terminal or source ${PROFILE} to use it"