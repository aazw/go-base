# My Go boilerplate project

This is a personal boilerplate codebase for my Go projects.

![Go version](https://img.shields.io/github/go-mod/go-version/aazw/go-base.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/aazw/go-base.svg)](https://pkg.go.dev/github.com/aazw/go-base)
[![Go Report Card](https://goreportcard.com/badge/github.com/aazw/go-base)](https://goreportcard.com/report/github.com/aazw/go-base)

![License](https://img.shields.io/github/license/aazw/go-base.svg)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/aazw/go-base)

## Progress

- [x] VSCode IDE
  - [x] Dev Container by Compose
  - [x] Formatter
  - [x] Linter
  - [x] Tasks (tasks.json)
  - [x] Debugger (launch.json)
- [x] Dockerfile
- [x] Docker Compose
  - [x] GoApp
  - [x] PostgreSQL
  - [x] PgWeb
  - [x] Valkey
  - [x] RedisInsight
  - [x] Grafana
  - [x] Prometheus
  - [x] Grafana Mimir
  - [x] Grafana Alloy
  - [x] Grafana Tempo
  - [x] Grafana Pyroscope
  - [x] Grafana Loki with Promtail
  - [x] K6
  - [x] openapi viewer (redocly/cli)
- [x] GoApp Base
  - [x] cli by cobra
  - [x] configuration by viper
  - [x] sqlc
  - [x] code genereate from sql by sqlc
  - [x] connection pooling with pgxpool
  - [x] golang-migrate
  - [x] openapi spec
  - [x] code generate from openapi spec by oapi-codegen
  - [x] go-playground/validator
    - [x] config validation
    - [x] request body validation
      - [x] custom validation error message
      - [x] rfc7807 problem details
  - [x] structed logs for Grafana Logs (Loki)
  - [x] tracing with Grafana Tempo
  - [x] profiling with Grafana Pyroscope
  - [x] metrics with Grafana Alloy/Mimir or Prometheus
  - [x] custom error
  - [x] uuid v7 for effective indexing in rerational database
  - [x] cors
  - [x] rate limit
  - [x] crean architecture based packaging
  - [x] graceful shutdown
  - [x] request timeout
  - [x] version embedeed
  - [x] リクエストボディサイズ制限
  - [x] セキュリティヘッダー挿入
    - [x] セキュリティに限らずカスタムヘッダ付与対応
 - [ ] Unittest
- [ ] CI
  - [x] Formatter (prettier, gofmt, shfmt, etc...)
  - [ ] Linter
  - [x] Dependencies (renovate.json)

