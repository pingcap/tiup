#!/bin/bash

set -eu

source script/grafana_entry.sh

echo "test how topology refreshes maintain the Grafana discovery entry"
grafana_entry
