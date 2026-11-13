#!/bin/bash

set -eu

source script/detach_grafana.sh

echo "test Grafana handoff and discovery during TiDB v8.5.6 to v8.5.7 upgrade"
detach_grafana
