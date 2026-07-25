#!/bin/bash

set -eu

function external_labels() {
    mkdir -p ~/.tiup/bin/

    version=$1
    name="test_external_labels_$RANDOM"
    topo=./topo/external_labels.yaml
    invalid_topo=./topo/external_labels_invalid.yaml

    echo "check invalid external_labels topology"
    set +e
    invalid_output=$(tiup-cluster check $invalid_topo -i ~/.ssh/id_rsa 2>&1)
    invalid_status=$?
    set -e
    if [ $invalid_status -eq 0 ]; then
        echo "expected invalid external_labels topology check to fail"
        exit 1
    fi
    echo "$invalid_output" | grep "contains reserved label 'cluster'"

    echo "deploy cluster with external_labels"
    tiup-cluster --yes deploy $name $version $topo -i ~/.ssh/id_rsa
    tiup-cluster list | grep "$name"

    echo "start cluster"
    tiup-cluster --yes start $name

    tiup-cluster _test $name writable
    tiup-cluster display $name

    # check the generated Prometheus config after the initial deploy
    assert_prometheus_external_labels $name n1 production us-east-1

    echo "edit config and reload cluster"
    EDITOR=ex tiup-cluster edit-config -y $name <<EOEX
:%s/production/staging/g
:%s/us-east-1/eu-central-1/g
:x
EOEX
    yes | tiup-cluster reload $name --transfer-timeout 60

    tiup-cluster _test $name writable

    # verify reload updates the rendered external_labels instead of keeping the old values
    assert_prometheus_external_labels $name n1 staging eu-central-1
    assert_prometheus_external_label_absent $name n1 environment production
    assert_prometheus_external_label_absent $name n1 region us-east-1

    echo "remove external_labels and reload cluster"
    EDITOR=ex tiup-cluster edit-config -y $name <<EOEX
:g/    external_labels:/.,+2d
:x
EOEX
    yes | tiup-cluster reload $name --transfer-timeout 60

    tiup-cluster _test $name writable

    assert_default_prometheus_external_labels $name n1
    assert_prometheus_external_label_absent $name n1 environment staging
    assert_prometheus_external_label_absent $name n1 region eu-central-1

    echo "destroy cluster"
    tiup-cluster --yes destroy $name
}
