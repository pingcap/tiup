#!/bin/bash

# Run in a subshell so the temporary profile and cleanup trap do not affect
# other cases sourced by run.sh. The fixed hosts belong to the Docker fixture.
function detach_grafana() (
    set -euo pipefail

    local name="test_detach_grafana_$RANDOM"
    local state
    state=$(mktemp -d /tmp/tiup-detach-grafana.XXXXXX)
    trap 'rm -rf "$state"' EXIT
    export TIUP_COMPONENT_DATA_DIR="$state/profile"
    local destroyed=false
    local failure_dir=''
    local units="pd-2379 tidb-4000 tikv-20160 prometheus-9090 grafana-3000 grafana-3001 grafana-3002 node_exporter-9100 blackbox_exporter-9115"

    # A detached host cannot be reached through cluster exec. Use the fixture's
    # root key throughout, including after destroy removes the cluster SSH key.
    function detach_remote() {
        local host=$1
        shift
        ssh -o BatchMode=yes -o ConnectTimeout=10 -i "$HOME/.ssh/id_rsa" "root@$host" "$@"
    }

    function detach_save_failure() {
        local host unit journal_args=''
        if [ -n "$failure_dir" ]; then
            return
        fi
        # The profile can contain test SSH keys. Keep it in a private directory
        # inside the control container, and only print the directory name.
        failure_dir=$(mktemp -d "/tmp/tiup-detach-failure-$name.XXXXXX") || return $?
        cp -a "$state" "$failure_dir/state"
        for unit in $units; do
            journal_args="$journal_args -u $unit.service"
        done
        for host in n1 n2 n3 n4 n5; do
            detach_remote "$host" "journalctl --no-pager -n 200 $journal_args" \
                > "$failure_dir/$host.journal.log" 2> "$failure_dir/$host.journal.stderr" || true
            detach_remote "$host" "bash -o pipefail -c 'find /home/tidb/detach-deploy -type f \\( -name \"*.log\" -o -name \"*.log.*\" \\) -print0 | tar --ignore-failed-read --null --verbatim-files-from -czf - -T -'" \
                > "$failure_dir/$host.logs.tar.gz" 2> "$failure_dir/$host.logs.stderr" || true
        done
        printf 'Detach failure diagnostics saved in %s\n' "$failure_dir"
    }

    # Refuse to replace resources from an earlier test. Install the destructive
    # cleanup trap only after proving these fixture paths and units are unused.
    for host in n1 n2 n3 n4 n5; do
        detach_remote "$host" "test ! -e /home/tidb/detach-deploy && test ! -e /home/tidb/detach-data && for unit in $units; do test ! -e /etc/systemd/system/\$unit.service || exit 1; done"
    done

    function detach_cleanup() {
        local status=$?
        local cleanup_failed=0 cluster_key=''
        trap - EXIT
        set +e
        if [ -r "$state/cluster.pub" ]; then
            cluster_key=$(base64 < "$state/cluster.pub" | tr -d '\n')
        elif [ -r "$TIUP_COMPONENT_DATA_DIR/clusters/$name/ssh/id_rsa.pub" ]; then
            # A failed deploy may already have distributed its key, before the
            # normal-path copy of cluster.pub below is reached.
            cluster_key=$(base64 < "$TIUP_COMPONENT_DATA_DIR/clusters/$name/ssh/id_rsa.pub" | tr -d '\n')
        fi
        if [ "$status" -ne 0 ]; then
            detach_save_failure
        fi
        if [ "$destroyed" = false ]; then
            tiup-cluster --yes destroy "$name" --force
        fi
        # This also removes agents orphaned on n2. Every path/unit below was
        # absent before this case, so cleanup cannot target another test.
        for host in n1 n2 n3 n4 n5; do
            if ! detach_remote "$host" "set -e; for unit in $units; do if test -e /etc/systemd/system/\$unit.service; then systemctl stop \$unit.service; systemctl disable \$unit.service; rm -f /etc/systemd/system/\$unit.service; fi; done; systemctl daemon-reload; rm -rf /home/tidb/detach-deploy /home/tidb/detach-data; if test -n '$cluster_key' && test -f /home/tidb/.ssh/authorized_keys; then key=\$(printf '%s' '$cluster_key' | base64 --decode); tmp=\$(mktemp /home/tidb/.ssh/detach-authorized.XXXXXX); awk -v key=\"\$key\" '\$0 != key' /home/tidb/.ssh/authorized_keys > \$tmp; cat \$tmp > /home/tidb/.ssh/authorized_keys; rm -f \$tmp; fi"; then
                cleanup_failed=1
            fi
        done
        if [ "$status" -eq 0 ] && [ "$cleanup_failed" -ne 0 ]; then
            status=1
            detach_save_failure
        fi
        rm -rf "$state"
        exit "$status"
    }
    trap detach_cleanup EXIT

    function detach_grafana_key() {
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

    function detach_assert_key() {
        local expected=$1 actual
        actual=$(detach_grafana_key) || return $?
        if [ "$actual" != "$expected" ]; then
            echo "Unexpected Grafana discovery record: expected '$expected', got '$actual'"
            return 1
        fi
    }

    function detach_snapshot() {
        local host=$1 port=$2
        local deploy="/home/tidb/detach-deploy/grafana-$port"
        # Grafana updates its database and logs itself. Compare only files
        # that TiUP owns and would otherwise replace during reload/upgrade.
        detach_remote "$host" "bash -o pipefail -c 'test -d $deploy/provisioning && test -d $deploy/dashboards && sha256sum $deploy/bin/bin/grafana-server $deploy/scripts/run_grafana.sh $deploy/conf/grafana.ini /etc/systemd/system/grafana-$port.service && find $deploy/provisioning $deploy/dashboards -type f -print0 | LC_ALL=C sort -z | xargs -0 -r sha256sum'"
    }

    function detach_assert_preserved() {
        local host=$1 port=$2
        detach_remote "$host" "systemctl is-active --quiet grafana-$port.service"
        test "$(detach_remote "$host" "systemctl show --property=MainPID --value grafana-$port.service")" = "$(cat "$state/$port.pid")"
        curl --fail --silent --show-error --connect-timeout 5 --max-time 15 "http://$host:$port/api/health"
        detach_snapshot "$host" "$port" > "$state/$port.current"
        diff -u "$state/$port.before" "$state/$port.current"
    }

    local key_a='{"ip":"n2","port":3000,"deploy_path":"/home/tidb/detach-deploy/grafana-3000"}'
    local key_b='{"ip":"n1","port":3001,"deploy_path":"/home/tidb/detach-deploy/grafana-3001"}'
    local key_c='{"ip":"n5","port":3002,"deploy_path":"/home/tidb/detach-deploy/grafana-3002"}'

    tiup-cluster --yes deploy "$name" v8.5.6 ./topo/detach_grafana.yaml -i "$HOME/.ssh/id_rsa"
    cp "$TIUP_COMPONENT_DATA_DIR/clusters/$name/ssh/id_rsa.pub" "$state/cluster.pub"
    tiup-cluster --yes start "$name"
    tiup-cluster _test "$name" writable
    detach_assert_key "$key_a"
    for entry in n2:3000 n1:3001; do
        host=${entry%:*}
        port=${entry#*:}
        curl --fail --silent --show-error --retry 10 --retry-connrefused --retry-delay 1 --max-time 15 "http://$host:$port/api/health"
        detach_snapshot "$host" "$port" > "$state/$port.before"
        detach_remote "$host" "systemctl show --property=MainPID --value grafana-$port.service" > "$state/$port.pid"
        test "$(cat "$state/$port.pid")" -gt 0
    done

    # Validate all requested nodes before committing any removal.
    sha256sum "$TIUP_COMPONENT_DATA_DIR/clusters/$name/meta.yaml" > "$state/meta.before"
    if tiup-cluster --yes detach-grafana "$name" -N n2:3000,n1:4000; then
        echo "detach unexpectedly accepted a non-Grafana node"
        return 1
    fi
    sha256sum --check "$state/meta.before"

    # An invalid proxy key must not be read for this local-only command.
    tiup-cluster --yes --ssh-proxy-host unused.invalid \
        --ssh-proxy-identity-file "$state/missing-proxy-key" \
        detach-grafana "$name" -N n2:3000
    detach_assert_key "$key_a"
    tiup-cluster display "$name" > "$state/display"
    ! grep -F 'n2:3000' "$state/display"
    grep -F 'n1:3001' "$state/display"
    detach_assert_preserved n2 3000

    # A remaining managed Grafana takes precedence on the next refresh.
    tiup-cluster --yes start "$name"
    detach_assert_key "$key_b"
    tiup-cluster --yes detach-grafana "$name" -N n1:3001
    detach_assert_key "$key_b"
    tiup-cluster display "$name" > "$state/display"
    ! grep -F 'n1:3001' "$state/display"

    tiup-cluster --yes start "$name"
    detach_assert_key "$key_b"
    tiup-cluster --yes reload "$name"
    detach_assert_key "$key_b"
    # Refresh removes the detached Grafana probes and the exclusive host's
    # exporter targets, while the shared host remains monitored.
    detach_remote n1 "! grep -q \"group: 'grafana'\" /home/tidb/detach-deploy/prometheus-9090/conf/prometheus.yml && ! grep -q 'n2:9100' /home/tidb/detach-deploy/prometheus-9090/conf/prometheus.yml && grep -q 'n1:9100' /home/tidb/detach-deploy/prometheus-9090/conf/prometheus.yml"
    detach_remote n2 "systemctl is-active --quiet node_exporter-9100.service"

    tiup-cluster --yes upgrade "$name" v8.5.7 --transfer-timeout 120
    tiup-cluster _test "$name" writable
    detach_assert_key "$key_b"
    detach_assert_preserved n2 3000
    detach_assert_preserved n1 3001

    # Starting to manage another Grafana overwrites discovery; removing that
    # last managed instance deletes the key instead of restoring an old one.
    tiup-cluster --yes scale-out "$name" ./topo/detach_grafana_scale_out.yaml -i "$HOME/.ssh/id_rsa"
    detach_assert_key "$key_c"
    tiup-cluster --yes scale-in "$name" -N n5:3002
    detach_assert_key ''
    detach_assert_preserved n2 3000
    detach_assert_preserved n1 3001

    tiup-cluster --yes destroy "$name"
    destroyed=true
    detach_assert_preserved n2 3000
    detach_assert_preserved n1 3001
)
