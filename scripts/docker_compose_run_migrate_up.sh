#!/bin/bash

# move to project root
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
cd ${SCRIPT_DIR}/..

# docker compose
docker compose run -i --rm goapp-migrate-up
