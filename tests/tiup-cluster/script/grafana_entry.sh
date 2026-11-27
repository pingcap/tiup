#!/bin/bash

# Run in a subshell so the temporary profile and cleanup trap do not affect
# other cases sourced by run.sh. The fixed hosts belong to the Docker fixture.
function grafana_entry() (
    set -euo pipefail

    local name="test_grafana_entry_$RANDOM"
    local state
    state=$(mktemp -d /tmp/tiup-grafana-entry.XXXXXX)
    export TIUP_COMPONENT_DATA_DIR="$state/profile"
    local meta="$TIUP_COMPONENT_DATA_DIR/clusters/$name/meta.yaml"
    local destroyed=false
    trap 'rm -rf "$state"' EXIT

    # A failed step may leave a hand-written cleanup record behind in the
    # metadata. Restore the file as TiUP wrote it before destroying, so that
    # the cluster can still be destroyed and no record leaks into another run.
    function grafana_entry_cleanup() {
        local status=$?
        trap - EXIT
        set +e
        if [ "$destroyed" = false ]; then
            if [ -f "$state/meta.before-edit" ]; then
                cp "$state/meta.before-edit" "$meta"
            fi
            tiup-cluster --yes destroy "$name" --force
        fi
        rm -rf "$state"
        exit "$status"
    }

    # Refuse to replace resources from an earlier test. Install the destructive
    # cleanup trap only after proving the fixture paths are unused.
    for host in n1 n2 n3 n4 n5; do
        ssh -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new \
            -i "$HOME/.ssh/id_rsa" "root@$host" \
            "test ! -e /home/tidb/entry-deploy && test ! -e /home/tidb/entry-data"
    done
    trap grafana_entry_cleanup EXIT

    function grafana_entry_key() {
        local response encoded
        # PD enables the etcd gRPC gateway by default. Reading one exact key
        # avoids an additional etcdctl installation in the control container.
        response=$(curl --fail --silent --show-error --connect-timeout 5 --max-time 15 \
            -H 'Content-Type: application/json' \
            --data '{"key":"L3RvcG9sb2d5L2dyYWZhbmE="}' \
            http://n3:2379/v3/kv/range) || return $?
        # This helper runs in command substitution, where Bash may disable
        # errexit. Never turn a failed read into a successful missing-key check.
        [[ "$response" == *'"header"'* ]] || return 1
        encoded=$(printf '%s' "$response" | sed -n 's/.*"value"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p') || return $?
        if [ -n "$encoded" ]; then
            printf '%s' "$encoded" | base64 --decode || return $?
        fi
    }

    function grafana_entry_assert_key() {
        local expected=$1 actual
        actual=$(grafana_entry_key) || return $?
        if [ "$actual" != "$expected" ]; then
            echo "Unexpected Grafana discovery record: expected '$expected', got '$actual'"
            return 1
        fi
    }

    function grafana_entry_write_key() {
        # Same gateway, standing in for an operator who maintains the
        # discovery record by hand.
        local encoded
        encoded=$(printf '%s' "$1" | base64 | tr -d '\n') || return $?
        curl --fail --silent --show-error --connect-timeout 5 --max-time 15 \
            -H 'Content-Type: application/json' \
            --data "{\"key\":\"L3RvcG9sb2d5L2dyYWZhbmE=\",\"value\":\"$encoded\"}" \
            http://n3:2379/v3/kv/put
    }

    function grafana_entry_owe_cleanup() {
        # Rebuild what a scale-in leaves behind when its PD update fails: the
        # entry is still in etcd and the metadata still owes its cleanup.
        # UpdateMeta writes exactly this record. It is the last key under
        # `topology:`, which yaml.v3 writes with 4-space indentation.
        printf '    pending_grafana_entry_cleanups:\n        - host: %s\n          port: %s\n' "$1" "$2" \
            >> "$meta"
    }

    function grafana_entry_assert_settled() {
        # A refresh that succeeded has dealt with the cleanup it owed. `! grep`
        # would not fail the script under set -e, so assert explicitly.
        if grep -q 'pending_grafana_entry_cleanups' "$meta"; then
            echo "metadata still records a pending Grafana entry cleanup"
            return 1
        fi
    }

    local key_a='{"ip":"n2","port":3000,"deploy_path":"/home/tidb/entry-deploy/grafana-3000"}'
    local key_b='{"ip":"n1","port":3001,"deploy_path":"/home/tidb/entry-deploy/grafana-3001"}'
    local key_c='{"ip":"n5","port":3002,"deploy_path":"/home/tidb/entry-deploy/grafana-3002"}'
    local key_moved='{"ip":"192.0.2.10","port":3000,"deploy_path":"/srv/grafana"}'

    tiup-cluster --yes deploy "$name" v8.5.6 ./topo/grafana_entry.yaml -i "$HOME/.ssh/id_rsa"
    tiup-cluster --yes start "$name"
    tiup-cluster _test "$name" writable

    # While TiUP manages a Grafana, the entry follows the managed instances:
    # scaling in one of them leaves the entry with a survivor.
    grafana_entry_assert_key "$key_a"
    tiup-cluster --yes scale-in "$name" -N n2:3000
    grafana_entry_assert_key "$key_b"

    # Scaling in the last managed Grafana deletes the entry: TiUP removed the
    # instance it points at. The cleanup recorded for a retry is settled by
    # the same command, and later refreshes find no entry and do not write one
    # back.
    tiup-cluster --yes scale-in "$name" -N n1:3001
    grafana_entry_assert_key ''
    grafana_entry_assert_settled
    tiup-cluster --yes start "$name"
    grafana_entry_assert_key ''

    # Once the cluster manages no Grafana, the entry belongs to whoever runs
    # Grafana. A record written by hand, here for a Grafana outside TiUP, is
    # left alone by a refresh. Every refresh used to delete it.
    grafana_entry_write_key "$key_moved"
    tiup-cluster --yes start "$name"
    grafana_entry_assert_key "$key_moved"

    # A managed Grafana takes the entry over; removing that last managed
    # instance deletes it instead of restoring the handwritten one.
    tiup-cluster --yes scale-out "$name" ./topo/grafana_entry_scale_out.yaml -i "$HOME/.ssh/id_rsa"
    grafana_entry_assert_key "$key_c"
    tiup-cluster --yes scale-in "$name" -N n5:3002
    grafana_entry_assert_key ''
    grafana_entry_assert_settled

    cp "$meta" "$state/meta.before-edit"

    # Rebuild what that scale-in leaves behind when its PD update fails: the
    # entry is still in etcd and the metadata still owes its cleanup. The next
    # refresh must retry the deletion and drop the record.
    grafana_entry_write_key "$key_c"
    grafana_entry_owe_cleanup n5 3002
    tiup-cluster --yes start "$name"
    grafana_entry_assert_key ''
    grafana_entry_assert_settled

    # The retry deletes only an entry that still points at the removed
    # instance. One rewritten meanwhile belongs to whoever rewrote it, and
    # settling the cleanup must not cost it.
    grafana_entry_write_key "$key_moved"
    grafana_entry_owe_cleanup n5 3002
    tiup-cluster --yes start "$name"
    grafana_entry_assert_key "$key_moved"
    grafana_entry_assert_settled

    tiup-cluster --yes destroy "$name"
    destroyed=true
)
