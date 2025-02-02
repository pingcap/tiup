#!/bin/bash

set -eu


# ref https://docs.pingcap.com/zh/tidb-data-migration/stable/deploy-a-dm-cluster-using-ansible
# script following the docs to deploy dm 1.0.6 using ./ansible_data/inventory.ini
function deploy_by_ansible() {
    # step 1
    apt-get -y install git curl sshpass python-pip sudo

    # step 2
    id tidb || useradd -m -d /home/tidb tidb
    echo "tidb:tidb" | chpasswd
    sed -i '/tidb/d' /etc/sudoers
    echo "tidb ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers

    # use the same key from root instead of create one.
    mkdir -p /home/tidb/.ssh
    cp ~/.ssh/* /home/tidb/.ssh/
    chown -R tidb:tidb /home/tidb/.ssh/

    # step 3
su tidb <<EOF
    cd /home/tidb
    wget -c https://download.pingcap.org/dm-ansible-v1.0.6.tar.gz
EOF

    # step 4
su tidb <<EOF
    cd /home/tidb
    tar -xzvf dm-ansible-v1.0.6.tar.gz &&
        mv dm-ansible-v1.0.6 dm-ansible &&
        cd /home/tidb/dm-ansible &&
        sudo pip install -r ./requirements.txt
    ansible --version
EOF

    # step 5

    # replace ip address from docker network
    ipprefix=${TIUP_TEST_IP_PREFIX:-"172.19.0"}
    sed "s/__IPPREFIX__/$ipprefix/g" ansible_data/hosts.ini.tpl > ansible_data/hosts.ini
    sed "s/__IPPREFIX__/$ipprefix/g" ansible_data/inventory.ini.tpl > ansible_data/inventory.ini

    cp ./ansible_data/hosts.ini /home/tidb/dm-ansible/
    cp ./ansible_data/inventory.ini /home/tidb/dm-ansible/

    cd /home/tidb/dm-ansible
    # not following the docs, use root and without password to run it
    sudo ansible-playbook -i hosts.ini create_users.yml -u root

    #step 6
su tidb <<EOF
    cd /home/tidb/dm-ansible
    ansible-playbook local_prepare.yml
EOF

    # skip 7,8

    # step 9
su tidb <<EOF
    cd /home/tidb/dm-ansible
    ansible -i inventory.ini all -m shell -a 'whoami'
    ansible -i inventory.ini all -m shell -a 'whoami' -b
    ansible-playbook deploy.yml
    ansible-playbook start.yml
EOF

}


function test() {
    deploy_by_ansible

    # stop cluster
su tidb <<EOF
    cd /home/tidb/dm-ansible
    ansible-playbook stop.yml
EOF

    # set up tiup root for tidb user
    mkdir -p /home/tidb/.tiup/bin
    cp /root/.tiup/bin/root.json /home/tidb/.tiup/bin/
    chown -R tidb:tidb /home/tidb/.tiup

    # import and start new cluster
su tidb <<EOF
    tiup-dm --yes import --dir /home/tidb/dm-ansible --cluster-version v2.0.0-rc
    tiup-dm --yes start test-cluster
    tiup-dm --yes destroy test-cluster
EOF
}

test
