#!/bin/bash

# move to project root
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
cd ${SCRIPT_DIR}/..

# renovate dry-run
LOG_LEVEL=debug renovate --platform=local --repository-cache=reset --dry-run=true
