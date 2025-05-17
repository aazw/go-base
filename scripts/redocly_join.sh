#!/bin/bash

# move to project root
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
cd ${SCRIPT_DIR}/..

# redocly
# https://github.com/Redocly/redocly-cli
redocly join openapi/base.yaml openapi/specs/*.yaml -o .openapi/openapi.yaml
