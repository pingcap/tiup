#! /bin/sh

repo='http://tidb.tnthub.com:81/mirror'
os=$(uname | tr '[:upper:]' '[:lower:]')

TIUP_HOME=${HOME}/.tiup
mkdir -p ${TIUP_HOME}/bin/
curl ${repo}/tiup-${os} -o ${TIUP_HOME}/bin/tiup

chmod 755 ${TIUP_HOME}/bin/tiup

cat >> ${HOME}/.bash_profile << EOF

export PATH=${TIUP_HOME}/bin:\${PATH}
EOF

source ${HOME}/.bash_profile

echo "tiup is installed in ${TIUP_HOME}/bin/tiup"
echo "we have modify ${HOME}/.bash_profile to add tiup to PATH"
echo "you can open a new terminal or source ${HOME}/.bash_profile to use it"