# go-custom-field-model (GCFM)

[![Docs](https://img.shields.io/badge/docs-latest-blue)](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md)
[![CI](https://github.com/faciam-dev/gcfm/actions/workflows/ci.yml/badge.svg)](https://github.com/faciam-dev/gcfm/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/faciam-dev/gcfm)](https://goreportcard.com/report/github.com/faciam-dev/gcfm)
[![codecov](https://codecov.io/gh/faciam-dev/gcfm/branch/main/graph/badge.svg)](https://codecov.io/gh/faciam-dev/gcfm)
[![License](https://img.shields.io/badge/license-MIT-green)](./LICENSE)

**go-custom-field-model (GCFM)** は、RDB や NoSQL 上のカスタムフィールド管理を自動化する Go 製フレームワークです。主に次のコンポーネントを提供します。

* **CLI (`fieldctl`)** — スキーマスキャン、ドリフト検出、YAML レジストリ操作
* **SDK (`sdk`)** — アプリケーションに組み込める Go API
* **API Server (`api-server`)** — REST API を公開し、[gcfm-dashboard](https://github.com/faciam-dev/gcfm-dashboard) と連携

---

## ✨ 特長

* **Schema Scan** — データベースのスキーマを自動検出し、メタデータを記録
* **YAML Registry** — バージョン管理された YAML でカスタムフィールド定義を管理
* **Diff & Apply** — スキーマドリフトを検知し、差分を適用
* **Multi-Tenant** — メタデータベースと複数のターゲットデータベースを管理
* **Extensible** — Go プラグインでバリデータを拡張可能
* **Observability** — Prometheus メトリクスと CI 用ドリフトガードを提供
* **Dashboard UI** — 別リポジトリの [gcfm-dashboard](https://github.com/faciam-dev/gcfm-dashboard) から操作

---

## 🚀 クイックスタート

### 1. ビルド

```bash
make build   # bin/ 配下に CLI と API サーバーをビルド
```

### 2. DB 初期化

```bash
bin/fieldctl db migrate \
  --table-prefix="gcfm_" \
  --db "root:rootpw@tcp(localhost:3306)/gcfm" \
  --schema public \
  --seed \
  --driver=mysql
```

### 3. API サーバー起動

```bash
bin/api-server -addr=:18081 --driver=mysql --dsn "root:rootpw@tcp(localhost:3306)/gcfm"
```

---

## 📷 CLI 利用例

### 既存データベースのスキャン

```bash
$ fieldctl scan --db "postgres://user:pass@localhost:5432/app" \
    --schema public --driver postgres
INSERT 8  UPDATE 2  SKIP 3 (reserved)
```

### DB と registry.yaml の差分表示

```diff
$ fieldctl diff --db "postgres://user:pass@localhost:5432/app" \
    --schema public --driver postgres --file registry.yaml --fail-on-change

--- posts.title (DB)
+++ posts.title (YAML)
- type: varchar(255)
+ type: text
```

### registry.yaml の適用 (ドライラン)

```bash
$ fieldctl apply --db "postgres://user:pass@localhost:5432/app" \
    --schema public --file registry.yaml --dry-run

[DRY-RUN] Would ALTER COLUMN posts.title TYPE text
```

---

## 🖥 ダッシュボード

[gcfm-dashboard](https://github.com/faciam-dev/gcfm-dashboard) を利用すると、ブラウザからフィールドを管理できます。

[![Dashboard Screenshot](https://faciam-dev.github.io/gcfm/img/dashboard.png)](https://github.com/faciam-dev/gcfm-dashboard)

---

## 📚 ドキュメント

* [CLI コマンド](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md#cli)
* [YAML レジストリ](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md#yaml-registry)
* [SDK サンプル](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md#sdk)
* [マルチテナント構成](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md#multi-tenant)
* [スナップショット & ロールバック](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md#snapshots)
* [メトリクス & CI ガード](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md#metrics)

👉 詳細は [docs/index.md](https://github.com/faciam-dev/gcfm/blob/main/docs/index.md) を参照してください。

---

## 📦 パッケージ構成

* `sdk/` — 公開 SDK
* `internal/` — サーバー実装
* `pkg/` — 共有ユーティリティ
* `cmd/fieldctl/` — CLI ツール

---

## 🤝 コントリビューション

Issue や Pull Request を歓迎しています。詳細は [CONTRIBUTING.md](./CONTRIBUTING.md) を参照してください。

---

## 📄 ライセンス

[MIT License](./LICENSE)

