#!/bin/bash

# move to project root
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
cd ${SCRIPT_DIR}/..

# golangci-lint
# https://github.com/golangci/golangci-lint
golangci-lint run
