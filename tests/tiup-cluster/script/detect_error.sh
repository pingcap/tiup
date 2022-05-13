#!/bin/bash
set -eu

err_num=$(find $1 -name "*.log" -exec grep "\[ERROR\]" {} \; | wc -l)
if [ ${err_num} != "0" ]; then
  echo ${err_num}
  exit 1
fi

echo "no error log found"