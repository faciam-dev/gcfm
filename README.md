# go-custom-field-model (GCFM)

[![Docs](https://img.shields.io/badge/docs-latest-blue)](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md)
[![CI](https://github.com/faciam-dev/gcfm/actions/workflows/ci.yml/badge.svg)](https://github.com/faciam-dev/gcfm/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/faciam-dev/gcfm)](https://goreportcard.com/report/github.com/faciam-dev/gcfm)
[![codecov](https://codecov.io/gh/faciam-dev/gcfm/branch/main/graph/badge.svg)](https://codecov.io/gh/faciam-dev/gcfm)
[![License](https://img.shields.io/badge/license-MIT-green)](./LICENSE)

**go-custom-field-model (GCFM)** is a Go framework that automates custom field management on RDBs and NoSQL databases. It provides:

* **CLI (`fieldctl`)** — schema scanning, drift detection, and YAML registry operations
* **SDK (`sdk`)** — Go API for embedding in applications
* **API Server (`api-server`)** — exposes a REST API and integrates with [gcfm-dashboard](https://github.com/faciam-dev/gcfm-dashboard)

---

## ✨ Features

* **Schema Scan** — automatically detects database schemas and records metadata
* **YAML Registry** — manages custom field definitions in versioned YAML
* **Diff & Apply** — detects schema drift and applies differences
* **Multi-Tenant** — manages a meta database and multiple target databases
* **Extensible** — extend validators via Go plugins
* **Observability** — Prometheus metrics and a CI drift guard
* **Dashboard UI** — operate from the separate [gcfm-dashboard](https://github.com/faciam-dev/gcfm-dashboard) repository

---

## 🚀 Quick Start

### 1. Build

```bash
make build   # builds the CLI and API server (outputs to bin/)
```

### 2. Initialize DB

```bash
fieldctl db migrate --db postgres://user:pass@localhost:5432/app --schema public --seed
```

### 3. Start API Server

```bash
bin/api-server -addr :8080
```

---

## 📷 CLI Examples

### Scan existing database

```bash
$ fieldctl scan --db "postgres://user:pass@localhost:5432/app" \
    --schema public --driver postgres
INSERT 8  UPDATE 2  SKIP 3 (reserved)
```

### Show drift between DB and registry.yaml

```diff
$ fieldctl diff --db "postgres://user:pass@localhost:5432/app" \
    --schema public --driver postgres --file registry.yaml --fail-on-change

--- posts.title (DB)
+++ posts.title (YAML)
- type: varchar(255)
+ type: text
```

### Apply registry.yaml (dry-run)

```bash
$ fieldctl apply --db "postgres://user:pass@localhost:5432/app" \
    --schema public --file registry.yaml --dry-run

[DRY-RUN] Would ALTER COLUMN posts.title TYPE text
```

---

## 🖥 Dashboard Example

By using [gcfm-dashboard](https://github.com/faciam-dev/gcfm-dashboard), you can manage fields from the browser.

[![Dashboard Screenshot](https://faciam-dev.github.io/gcfm/img/dashboard.png)](https://github.com/faciam-dev/gcfm-dashboard)

---

## 📚 Documentation

* [CLI Commands](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md#cli)
* [YAML Registry](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md#yaml-registry)
* [SDK Examples](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md#sdk)
* [Multi-Tenant Setup](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md#multi-tenant)
* [Snapshots & Rollback](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md#snapshots)
* [Metrics & CI Guard](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md#metrics)

👉 See [docs/index.md](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md) for details.

---

## 📦 Package Layout

* `sdk/` — public SDK
* `internal/` — server implementation
* `pkg/` — shared utilities
* `cmd/fieldctl/` — CLI tool

---

## 🤝 Contributing

We welcome pull requests, issues, and feature proposals. See [CONTRIBUTING.md](./CONTRIBUTING.md) for details.

---

## 📄 License

[MIT License](./LICENSE)

