#!/bin/bash
set -eu

err_num=$(find $1 -name "*.log" -exec grep "\[ERROR\]" {} \; | wc -l)
if [ ${err_num} != "0" ]; then
  echo "detect ${err_num} [ERROR] log"
fi

err_num=$(find $1 -name "*stderr.log" -exec cat {} \; | wc -l)
if [ ${err_num} != "0" ]; then
  echo "detect ${err_num} stderr log"
fi

echo "no error log found"
