#!/bin/bash

# move to project root
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
cd ${SCRIPT_DIR}/..

# oapi-codegen by go generate
# https://github.com/oapi-codegen/oapi-codegen
go generate ./...
