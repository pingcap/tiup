#!/usr/bin/env bash
# Copyright 2026 PingCAP, Inc.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Exercise real signing and downloads using disposable keys and a loopback server.
# The inert test package is never executed; no remote mirror or deployment is used.
set -euo pipefail

tiup=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
server=$(cd "$(dirname "$2")" && pwd)/$(basename "$2")
temporary=$(mktemp -d "${TMPDIR:-/tmp}/tiup-server-private-files.XXXXXX")
server_pid=

stop_server() {
    if [ -n "$server_pid" ]; then
        kill "$server_pid" 2>/dev/null || true
        wait "$server_pid" 2>/dev/null || true
        server_pid=
    fi
}

cleanup() {
    stop_server
    rm -rf "$temporary"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

unset TIUP_MIRROR_SYNC_SCRIPT
while IFS= read -r variable; do
    case "$variable" in
        *_proxy|*_PROXY) unset "$variable" ;;
    esac
done < <(compgen -e)

run_cli() {
    if ! "$tiup" "$@" > "$work/cli.log" 2>&1; then
        cat "$work/cli.log" >&2
        return 1
    fi
}

snapshot_keys() {
    (cd "$keys" && find . -type f -exec shasum -a 256 {} \; | LC_ALL=C sort)
}

for layout in default custom external; do
    work="$temporary/$layout"
    root="$work/mirror"
    export TIUP_HOME="$work/profile" TIUP_MIRRORS="$root"
    export TIUP_LOG_PATH="$work/logs" TIUP_COMPONENT_DATA_DIR="$work/server-data"
    mkdir -p "$TIUP_HOME/bin" "$TIUP_COMPONENT_DATA_DIR"
    cd "$work"

    run_cli mirror init "$root"
    keys="$root/keys"
    test -d "$keys"
    test ! -e "$work/mirror.keys"
    run_cli mirror set "$root"
    run_cli mirror genkey
    run_cli mirror grant pingcap
    snapshot_keys > "$work/keys-before.sha256"
    args=("$root" --upstream=)
    if [ "$layout" != default ]; then
        if [ "$layout" = custom ]; then
            selected="$root/private-signing"
        else
            selected="$work/private"
        fi
        mv "$keys" "$selected"
        keys="$selected"
        args+=(--key-dir "$keys")
        mkdir "$root/keys"
        printf '%s\n' 'LEGACY PRIVATE MARKER' > "$root/keys/legacy.json"
    fi

    printf '%s\n' 'inert package; never executed' > "$work/marker.txt"
    tar -czf "$work/inert.tar.gz" -C "$work" marker.txt

    # Retry only an occupied random loopback port; other startup failures are errors.
    ready=false
    for ((attempt = 0; attempt < 5; attempt++)); do
        base="http://127.0.0.1:$((20000 + RANDOM))"
        "$server" "${args[@]}" --addr="${base#http://}" > "$work/server.log" 2>&1 &
        server_pid=$!
        for ((poll = 0; poll < 100; poll++)); do
            if ! kill -0 "$server_pid" 2>/dev/null; then
                break
            fi
            if curl --noproxy '*' --silent --fail --max-time 1 "$base/root.json" \
                -o "$work/served-root.json" 2>/dev/null && \
                cmp -s "$root/root.json" "$work/served-root.json"; then
                ready=true
                break
            fi
            sleep 0.1
        done
        if "$ready"; then
            break
        fi
        stop_server
        if ! grep -q 'address already in use' "$work/server.log"; then
            break
        fi
    done
    if ! "$ready"; then
        cat "$work/server.log" >&2
        echo "Server startup failed for $layout layout" >&2
        exit 1
    fi

    TIUP_MIRRORS="$base" run_cli mirror publish marker v1.0.0 "$work/inert.tar.gz" marker.txt \
        --os=linux --arch=amd64
    TIUP_MIRRORS="$base" run_cli list marker
    curl --noproxy '*' --silent --show-error --fail --max-time 5 \
        "$base/marker-v1.0.0-linux-amd64.tar.gz" -o "$work/download.tar.gz"
    cmp "$work/inert.tar.gz" "$work/download.tar.gz"

    denied=()
    if [ "$layout" = external ]; then
        denied+=(/keys/legacy.json)
    else
        if [ "$layout" = default ]; then
            prefix=/keys
        else
            prefix=/private-signing
            denied+=(/keys/legacy.json)
        fi
        while IFS= read -r key_file; do
            denied+=("$prefix/${key_file#"$keys"/}")
        done < <(find "$keys" -type f)
    fi
    denied+=(/keys/missing.json /KEYS/missing.json)
    for private_path in "${denied[@]}"; do
        # Discard response bytes, including on a failing private-access assertion.
        status=$(curl --noproxy '*' --silent --show-error --max-time 5 \
            -o /dev/null -w '%{http_code}' "$base$private_path")
        if [ "$status" != 403 ]; then
            echo "Private path returned $status ($layout): $private_path" >&2
            exit 1
        fi
    done
    stop_server

    if [ "$layout" = default ]; then
        run_cli mirror publish marker v1.0.1 "$work/inert.tar.gz" marker.txt --os=linux --arch=amd64
        run_cli list marker
    fi
    snapshot_keys > "$work/keys-after.sha256"
    cmp "$work/keys-before.sha256" "$work/keys-after.sha256"
    test ! -e "$work/mirror.keys"
    echo "PASS: $layout keys retained, server signing works, private HTTP denied"
done
