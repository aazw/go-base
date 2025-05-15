# for main image
ARG TYPESCRIPT_NODE_VERSION=22-bookworm

# for builder images
ARG GOLANG_VERSION=1.24
ARG JQ_VERSION=1.7.1
ARG GOLANGCI_LINT_VERSION=v2.1.6
ARG SQLC_VERSION=1.29.0
ARG GOLANG_MIGRATE_VERSION=v4.18.3
ARG DIVE_VERSION=v0.13.1

# for go install
ARG DOCKERFMT_VERSION=v0.3.7
ARG DELVE_VERSION=v1.24.2
ARG OAPI_CODEGEN_VERSION=v2.4.1
ARG OGEN_VERSION=v1.12.0
ARG SHFMT_VERSION=v3.11.0

# for npm tools
ARG OPENAPI_GENERATOR_CLI_VERSION=2.20.0
ARG IBM_OPENAPI_VALIDATOR_VERSION=1.35.2
ARG REDOCLY_CLI_VERSION=1.34.3
ARG SQL_FORMATTER_VERSION=15.6.1

# ##############################
# builder images
# ##############################

# golang
# https://hub.docker.com/_/golang
FROM golang:${GOLANG_VERSION} AS golang-builder

# jq
# https://github.com/jqlang/jq
FROM ghcr.io/jqlang/jq:${JQ_VERSION} AS jq-builder

# golangci-lint
# https://golangci-lint.run/welcome/install/#local-installation
FROM golangci/golangci-lint:${GOLANGCI_LINT_VERSION} AS golangci-lint-builder

# sqlc
# https://github.com/sqlc-dev/sqlc
# https://sqlc.dev/
# https://hub.docker.com/r/sqlc/sqlc
FROM sqlc/sqlc:${SQLC_VERSION} AS sqlc-builder

# golang-migrate
# https://github.com/golang-migrate/migrate
# https://hub.docker.com/r/migrate/migrate
FROM migrate/migrate:${GOLANG_MIGRATE_VERSION} AS golang-migrate-builder

# dive
# https://github.com/wagoodman/dive
# https://hub.docker.com/r/wagoodman/dive
FROM wagoodman/dive:${DIVE_VERSION} AS dive-builder

# ##############################
# main image
# ##############################

# golang
# https://mcr.microsoft.com/v2/vscode/devcontainers/go/tags/list
# FROM mcr.microsoft.com/devcontainers/go:${GOLANG_VERSION}

# typescript-node
# https://mcr.microsoft.com/v2/vscode/devcontainers/typescript-node/tags/list
FROM mcr.microsoft.com/devcontainers/typescript-node:${TYPESCRIPT_NODE_VERSION}

RUN apt-get -y update
RUN apt-get -y install --no-install-recommends \
    apt-transport-https \
    build-essential \
    ca-certificates \
    gnupg \
    gnupg-agent \
    lsb-release \
    software-properties-common \
    curl \
    git \
    procps

# golang
# refs above
COPY --from=golang-builder /usr/local/go /usr/local/
ENV PATH=$PATH:/usr/local/go/bin/:/root/go/bin/

# docker
# https://matsuand.github.io/docs.docker.jp.onthefly/engine/install/debian/
RUN for pkg in docker.io docker-doc docker-compose podman-docker containerd runc; do sudo apt-get remove $pkg; done
RUN curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
RUN echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/debian \
    $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list >/dev/null
RUN apt-get -y update
RUN apt-get -y install docker-ce docker-ce-cli containerd.io docker-compose-plugin

# dockerfmt
# https://github.com/reteps/dockerfmt
ARG DOCKERFMT_VERSION
RUN go install github.com/reteps/dockerfmt@${DOCKERFMT_VERSION}

# delve
# https://github.com/go-delve/delve
ARG DELVE_VERSION
RUN go install github.com/go-delve/delve/cmd/dlv@${DELVE_VERSION}

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
RUN yarn global add @openapitools/openapi-generator-cli@${OPENAPI_GENERATOR_CLI_VERSION}

# oapi-codegen
# https://github.com/oapi-codegen/oapi-codegen
ARG OAPI_CODEGEN_VERSION
RUN go install -v github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@${OAPI_CODEGEN_VERSION}

# ogen
# https://github.com/ogen-go/ogen
ARG OGEN_VERSION
RUN go install -v github.com/ogen-go/ogen/cmd/ogen@${OGEN_VERSION}

# lint-openapi
# https://github.com/IBM/openapi-validator
RUN yarn global add ibm-openapi-validator@${IBM_OPENAPI_VALIDATOR_VERSION}

# redocly-cli
# https://github.com/Redocly/redocly-cli
RUN yarn global add @redocly/cli@${REDOCLY_CLI_VERSION}

# dive
# refs above
COPY --from=dive-builder /usr/local/bin/dive /usr/local/bin/

# shfmt
# https://github.com/mvdan/sh
ARG SHFMT_VERSION
RUN go install mvdan.cc/sh/v3/cmd/shfmt@${SHFMT_VERSION}

# sql-formatter
# https://github.com/sql-formatter-org/sql-formatter
RUN yarn global add sql-formatter@${SQL_FORMATTER_VERSION}

# mkcert
# https://github.com/FiloSottile/mkcert
RUN apt-get install -y libnss3-tools \
    && apt-get install -y mkcert