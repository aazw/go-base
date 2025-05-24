# for main image
ARG DOCKER_MAIN_VERSION=1.24.3-bookworm

# for builder images
ARG DOCKER_NODE_VERSION=22.16.0-bookworm
ARG DOCKER_JQ_VERSION=1.7.1
ARG DOCKER_GOLANGCI_LINT_VERSION=v2.1.6
ARG DOCKER_SQLC_VERSION=1.29.0
ARG DOCKER_GOLANG_MIGRATE_VERSION=v4.18.3
ARG DOCKER_DIVE_VERSION=v0.13.1
ARG DOCKER_K6_VERSION=1.0.0

# for go install
ARG GO_DOCKERFMT_VERSION=v0.3.7
ARG GO_DELVE_VERSION=v1.24.2
ARG GO_OAPI_CODEGEN_VERSION=v2.4.1
ARG GO_OGEN_VERSION=v1.13.0
ARG GO_SHFMT_VERSION=v3.11.0
ARG GO_MKCERT_VERSION=v1.4.4
ARG GO_DEPENDABOT_VERSION=v1.65.0
ARG GO_GO_LICENSES=v1.6.0
ARG GO_GO_TASK_VERSION=v3.43.3

# for npm tools
ARG NPM_OPENAPI_GENERATOR_CLI_VERSION=2.20.2
ARG NPM_IBM_OPENAPI_VALIDATOR_VERSION=1.35.2
ARG NPM_REDOCLY_CLI_VERSION=1.34.3
ARG NPM_SQL_FORMATTER_VERSION=15.6.2
ARG NPM_PRETTIER_VERSION=3.5.3
ARG NPM_RENOVATE_VERSION=40.28.0

# for pipx tools
ARG PIPX_GITINGEST_VERSION=0.1.4

# ##############################
# builder images
# ##############################

# node
# https://hub.docker.com/_/node
# https://note.milldea.com/posts/two-ways-to-install-nodejs-with-fixed-version-in-dockerfile
FROM node:${DOCKER_NODE_VERSION} AS node-builder

# jq
# https://github.com/jqlang/jq
FROM ghcr.io/jqlang/jq:${DOCKER_JQ_VERSION} AS jq-builder

# golangci-lint
# https://golangci-lint.run/welcome/install/#local-installation
FROM golangci/golangci-lint:${DOCKER_GOLANGCI_LINT_VERSION} AS golangci-lint-builder

# sqlc
# https://github.com/sqlc-dev/sqlc
# https://sqlc.dev/
# https://hub.docker.com/r/sqlc/sqlc
FROM sqlc/sqlc:${DOCKER_SQLC_VERSION} AS sqlc-builder

# golang-migrate
# https://github.com/golang-migrate/migrate
# https://hub.docker.com/r/migrate/migrate
FROM migrate/migrate:${DOCKER_GOLANG_MIGRATE_VERSION} AS golang-migrate-builder

# dive
# https://github.com/wagoodman/dive
# https://hub.docker.com/r/wagoodman/dive
FROM wagoodman/dive:${DOCKER_DIVE_VERSION} AS dive-builder

# k6
# https://github.com/grafana/k6
# https://hub.docker.com/r/grafana/k6
FROM grafana/k6:${DOCKER_K6_VERSION} AS k6-builder

# ##############################
# main image
# ##############################

# golang

# debian version
# 10 (Buster)
# 11 (Bullseye)
# 12 (Bookworm)
# 13 (Trixie)
# 14 (Forky)

# golang
# https://hub.docker.com/_/golang
# https://github.com/devcontainers/images/tree/main/src/go
# https://github.com/devcontainers/images/blob/main/src/go/.devcontainer/Dockerfile
# https://mcr.microsoft.com/v2/vscode/devcontainers/go/tags/list
FROM golang:${DOCKER_MAIN_VERSION}

RUN apt-get -y update
RUN apt-get -y install \
    ca-certificates \
    curl \
    git \
    nano

# node
# refs above
COPY --from=node-builder /usr/local/bin/node /usr/local/bin/node
COPY --from=node-builder /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -s /usr/local/bin/node /usr/local/bin/nodejs \
    && ln -s /usr/local/lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
    && ln -s /usr/local/lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npx

# yarn
RUN npm install -g yarn

# pipx
# https://github.com/pypa/pipx
RUN apt-get -y install pipx
RUN pipx ensurepath

# add vscode user
# https://code.visualstudio.com/remote/advancedcontainers/add-nonroot-user#_creating-a-nonroot-user
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN groupadd --gid $USER_GID $USERNAME \
    && useradd --uid $USER_UID --gid $USER_GID -m $USERNAME \
    && apt-get update \
    && apt-get install -y sudo \
    && echo $USERNAME ALL=\(root\) NOPASSWD:ALL >/etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME
USER $USERNAME

# dockerfmt
# https://github.com/reteps/dockerfmt
ARG GO_DOCKERFMT_VERSION
RUN go install github.com/reteps/dockerfmt@${GO_DOCKERFMT_VERSION}

# delve
# https://github.com/go-delve/delve
ARG GO_DELVE_VERSION
RUN go install github.com/go-delve/delve/cmd/dlv@${GO_DELVE_VERSION}

# jq
# refs above
COPY --from=jq-builder /jq /usr/local/bin/

# golangci-lint
# refs above
COPY --from=golangci-lint-builder /usr/bin/golangci-lint /usr/local/bin/

# sqlc
# refs above
COPY --from=sqlc-builder /workspace/sqlc /usr/local/bin/

# golang-migrate
# refs above
COPY --from=golang-migrate-builder /usr/local/bin/migrate /usr/local/bin/

# openapi-generator-cli
# https://github.com/OpenAPITools/openapi-generator
# https://github.com/OpenAPITools/openapi-generator-cli
ARG NPM_OPENAPI_GENERATOR_CLI_VERSION
RUN yarn global add @openapitools/openapi-generator-cli@${NPM_OPENAPI_GENERATOR_CLI_VERSION}

# oapi-codegen
# https://github.com/oapi-codegen/oapi-codegen
ARG GO_OAPI_CODEGEN_VERSION
RUN go install -v github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@${GO_OAPI_CODEGEN_VERSION}

# ogen
# https://github.com/ogen-go/ogen
ARG GO_OGEN_VERSION
RUN go install -v github.com/ogen-go/ogen/cmd/ogen@${GO_OGEN_VERSION}

# lint-openapi
# https://github.com/IBM/openapi-validator
ARG NPM_IBM_OPENAPI_VALIDATOR_VERSION
RUN yarn global add ibm-openapi-validator@${NPM_IBM_OPENAPI_VALIDATOR_VERSION}

# redocly-cli
# https://github.com/Redocly/redocly-cli
ARG NPM_REDOCLY_CLI_VERSION
RUN yarn global add @redocly/cli@${NPM_REDOCLY_CLI_VERSION}

# dive
# refs above
COPY --from=dive-builder /usr/local/bin/dive /usr/local/bin/

# shfmt
# https://github.com/mvdan/sh
ARG GO_SHFMT_VERSION
RUN go install mvdan.cc/sh/v3/cmd/shfmt@${GO_SHFMT_VERSION}

# sql-formatter
# https://github.com/sql-formatter-org/sql-formatter
ARG NPM_SQL_FORMATTER_VERSION
RUN yarn global add sql-formatter@${NPM_SQL_FORMATTER_VERSION}

# mkcert
# https://github.com/FiloSottile/mkcert
ARG GO_MKCERT_VERSION
RUN go install filippo.io/mkcert@${GO_MKCERT_VERSION}

# gitingest
# https://github.com/cyclotruc/gitingest
ARG PIPX_GITINGEST_VERSION
RUN pipx install gitingest==${PIPX_GITINGEST_VERSION}

# prettier
# https://prettier.io/docs/install
ARG NPM_PRETTIER_VERSION
RUN yarn global add prettier@${NPM_PRETTIER_VERSION}

# dependabot
# https://github.com/dependabot/cli
ARG GO_DEPENDABOT_VERSION
RUN go install github.com/dependabot/cli/cmd/dependabot@${GO_DEPENDABOT_VERSION}

# renovate
# https://github.com/renovatebot/renovate
# https://www.npmjs.com/package/renovate
ARG NPM_RENOVATE_VERSION
RUN yarn global add renovate@${NPM_RENOVATE_VERSION}

# k6
# refs above
COPY --from=k6-builder /usr/bin/k6 /usr/local/bin/

# go-licenses
# https://github.com/google/go-licenses
ARG GO_GO_LICENSES
RUN go install github.com/google/go-licenses@${GO_GO_LICENSES}

# go-task
# https://github.com/go-task/task
# https://taskfile.dev/
ARG GO_GO_TASK_VERSION
RUN go install github.com/go-task/task/v3/cmd/task@${GO_GO_TASK_VERSION}