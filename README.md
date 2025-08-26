# go-custom-field-model
[![Docs](https://img.shields.io/badge/docs-latest-blue)](https://faciam-dev.github.io/gcfm/)
A model system that provides custom fields

## Package Layout

- `sdk/`: Public SDK exposing custom field operations.
- `internal/`: Application code implementing the server.
- `pkg/`: Shared utility packages. There is currently no `pkg/customfields`; custom field helpers live under `sdk` and `internal/customfield`.

## Build

Run `make build` to compile the CLI and API server. Binaries will be placed in
the `bin` directory.


## fieldctl scan

Scan existing database tables and store the metadata in the `gcfm_custom_fields` table.

```
fieldctl scan --db "postgres://user:pass@localhost:5432/testdb" --schema public --driver postgres
```

Use `--dry-run` to print the discovered fields without inserting:

```
fieldctl scan --db "user:pass@tcp(localhost:3306)/testdb" --schema testdb --dry-run
```

Scan a monitored database by ID with verbose output:

```bash
fieldctl scan --db-id prod -v --db "postgres://user:pass@localhost:5432/core" --schema public --driver postgres
# INSERT 8  UPDATE 2  SKIP 3 (reserved)
```

## registry YAML

`registry.yaml` describes custom field metadata.

```yaml
version: 0.2
fields:
  - table: posts
    column: author_email
    type: varchar(255)
    display:
      labelKey: field.author_email.label
      widget: email
      placeholderKey: field.author_email.ph
    validator: email
```

## fieldctl export

Export metadata from the database to a YAML file.

```
fieldctl export --db "mongodb://localhost:27017" --schema appdb --driver mongo --out registry.yaml
```

## fieldctl apply

Apply a YAML file to the database. Use `--dry-run` to see the diff only.

```
fieldctl apply --db "postgres://user:pass@localhost:5432/testdb" --schema public --driver postgres --file registry.yaml --dry-run
```

## fieldctl diff

Show schema drift between a YAML file and the database. Use `--fail-on-change` to exit with code 2 when drift exists.

```
fieldctl diff --db "postgres://user:pass@localhost:5432/testdb" --schema public --driver postgres --file registry.yaml --table-prefix gcfm_ --fail-on-change
```
If you pass `--fallback-export`, the command will export the current database schema to the given file when it does not exist and exit with code 3.

### Skip reserved tables
Reserved or system tables are excluded from diffs by default.
Disable this behavior with `--skip-reserved=false` to include them.

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | no drift detected or drift ignored |
| 1 | command error |
| 2 | drift detected with `--fail-on-change` |
| 3 | registry exported because file was missing |

## Supported Drivers

| Driver | Package | Test Container |
|--------|---------|----------------|
| MySQL | internal/driver/mysql | mysql:8 |
| PostgreSQL | internal/driver/postgres | postgres:16 |
| MongoDB | internal/driver/mongo | mongo:7 |

## Runtime Cache

Use the runtime cache to load custom field metadata at startup and reload it periodically:

```go
import (
    runtimecache "github.com/faciam-dev/gcfm/internal/customfield/runtime/cache"
    "github.com/faciam-dev/gcfm/pkg/registry"
    mysqlscanner "github.com/faciam-dev/gcfm/pkg/driver/mysql"
    "go.uber.org/zap"
    "time"
)

db, _ := sql.Open("mysql", "user:pass@tcp(localhost:3306)/app")
sc := mysqlscanner.NewScanner(db)
cache, err := runtimecache.New(ctx, sc, registry.DBConfig{Schema: "app"}, time.Minute, zap.NewNop().Sugar())
```


## Dynamic Targets

Load target databases from an external file and hot-reload without downtime:

```go
meta, _ := sql.Open("postgres", os.Getenv("META_DSN"))
svc := sdk.New(sdk.ServiceConfig{
  MetaDB:        meta,
  MetaDriver:    "postgres",
  MetaSchema:    "gcfm_meta",
  Driver:        "mysql", // default
  Connector:     nil,      // built-in connector
  TargetResolver: sdk.TenantResolverFromPrefix("tenant:"),
})

stop := svc.StartTargetWatcher(context.Background(), sdk.NewFileProvider("targets.json"), 5*time.Second)
// defer stop()

ctx := sdk.WithTenantID(context.Background(), "A")
info, _ := svc.DescribeTable(ctx, "posts") // routed to tenant:A
```

### Query targets by label

```go
// 例: 「東京リージョンの prod テナント」をナイトリーにスキャン
q, _ := sdk.ParseQuery("region=tokyo,env=prod,!deprecated")
_ = svc.targets.ForEachByQuery(q, func(key string, t sdk.TargetConn) error {
  tables, err := listTables(ctx, t)
  if err != nil { return fmt.Errorf("%s: %w", key, err) }
  tx, _ := svc.meta.BeginTx(ctx, nil)
  defer rollbackIfNeeded(tx)
  for _, tb := range tables {
    if err := svc.meta.RecordScanResult(ctx, tx, ScanResult{Key: key, Table: tb}); err != nil {
      return err
    }
  }
  return tx.Commit()
})
```

Labels and queries are normalized to lowercase, so lookups are case-insensitive.


## Plugin 作成ガイド

カスタムバリデータを Go プラグインとして追加できます。最小構成は以下の通りです。

```go
package main

import (
    "github.com/faciam-dev/gcfm/pkg/customfield"
)

type myPlugin struct{}

func (myPlugin) Name() string { return "hello" }
func (myPlugin) Validate(v any) error { return nil }

func New() customfield.ValidatorPlugin { return myPlugin{} }
```

ビルドしてインストール:

```
fieldctl plugins install ./path/to/plugin
fieldctl plugins list
```

`fieldctl plugins install` only allows installing modules whose import path
matches prefixes listed in the `FIELDCTL_TRUSTED_MODULE_PREFIXES` environment
variable. When unset, the default prefix defined by
`DefaultTrustedModulePrefix` (`github.com/faciam-dev/`) is used. Specify a comma
separated list to allow additional trusted module paths.

CI などで静的インポートする場合は:

```
fieldctl generate plugin-import github.com/example/validator > plugins_gen.go
```

## Registry Schema Version

The registry schema is versioned using embedded SQL migrations and currently
supports MySQL and PostgreSQL databases. The current
version can be checked with:

```
fieldctl registry version --db "mysql://user:pass@/app"
```

Upgrade or downgrade the schema using the migrate command:

```
fieldctl registry migrate --db "mysql://user:pass@/app"        # upgrade to latest
fieldctl registry migrate --db "mysql://user:pass@/app" --to 1 # migrate down
```

If `fieldctl apply` detects that the YAML version is newer than the database
schema, it will exit with an error asking to run migration first.


## Go で使う例

以下のように `sdk.Service` を使うことで CLI を介さずコードから直接操作できます。

```go
package main

import (
	"context"
	"database/sql"
	"log"
	
	client "github.com/faciam-dev/gcfm/sdk/client"
	"github.com/faciam-dev/gcfm/sdk"
)

func main() {
    db, _ := sql.Open("mysql", "user:pass@tcp(localhost:3306)/app")
    svc := sdk.New(sdk.ServiceConfig{DB: db, Driver: "mysql", Schema: "app"})
    cli := client.NewLocalService(svc)
    ctx := context.Background()
    yaml, err := svc.Export(ctx, sdk.DBConfig{DSN: "mysql://user:pass@tcp(localhost:3306)/app", Schema: "app"})
    if err != nil {
        log.Fatal(err)
    }
    if _, err := svc.Apply(ctx, sdk.DBConfig{DSN: "mysql://user:pass@tcp(localhost:3306)/app", Schema: "app"}, yaml, sdk.ApplyOptions{}); err != nil {
        log.Fatal(err)
    }

      _ = cli.Create(ctx, sdk.FieldMeta{TableName: "posts", ColumnName: "title", DataType: "text"})
  }
  ```

### Separate metadata database

```go
meta := sql.Open("postgres", os.Getenv("META_DSN"))
target := sql.Open("mysql", os.Getenv("TARGET_DSN"))

svc := sdk.New(sdk.ServiceConfig{
    // Monitored target DB (defaults)
    DB:     target,
    Driver: "mysql",
    Schema: "",

    // Separate metadata store
    MetaDB:     meta,
    MetaDriver: "postgres",
    MetaSchema: "gcfm_meta",
})
```

### Multiple target databases

```go
// MetaDB (PostgreSQL)
meta, _ := sql.Open("postgres", os.Getenv("META_DSN"))

// Target DBs (tenants split on MySQL)
dbA, _ := sql.Open("mysql", os.Getenv("TENANT_A_DSN"))
dbB, _ := sql.Open("mysql", os.Getenv("TENANT_B_DSN"))

svc := sdk.New(sdk.ServiceConfig{
    // Legacy-compatible default (can be empty if unused)
    DB:     dbA,
    Driver: "mysql",

    // Separate MetaDB
    MetaDB:     meta,
    MetaDriver: "postgres",
    MetaSchema: "gcfm_meta",

    // Multiple targets
    Targets: []sdk.TargetConfig{
        { Key: "tenant:A", DB: dbA, Driver: "mysql", Schema: "" },
        { Key: "tenant:B", DB: dbB, Driver: "mysql", Schema: "" },
    },

    // Resolve target from tenant ID
    TargetResolver: sdk.TenantResolverFromPrefix("tenant:"),
    ReadSource:     sdk.ReadFromMeta, // read custom field metadata from MetaDB
})

// Usage from a single request
ctx := sdk.WithTenantID(context.Background(), "A")
defs, err := svc.ListCustomFields(ctx, 1, "posts") // definitions loaded from MetaDB
if err != nil { /* handle */ }

// Optional drift check and repair
rep, _ := svc.ReconcileCustomFields(ctx, 1, "posts", true)
_ = rep

// Nightly batch scan across all targets
_ = svc.NightlyScan(context.Background())
```

`NightlyScan` iterates over every registered target via the registry and stores
its results in the MetaDB. This pattern can be adapted for other batch jobs
that need to touch each tenant database.

### Automatic label-based target resolution

```go
// Gateway/Envoyからのヘッダ、JWTクレーム、BFFが詰めたctx値を使う例
svc := sdk.New(sdk.ServiceConfig{
  MetaDB: meta, MetaDriver: "postgres", MetaSchema: "gcfm_meta",
  DefaultStrategy:   sdk.SelectConsistentHash,
  DefaultPreferLabel: "primary=true",
  TargetResolverV2: sdk.AutoLabelResolver(sdk.AutoLabelResolverOptions{
    HTTP: &sdk.HTTPLabelRules{
      HeaderMap: map[string]string{
        "x-tenant-id": "tenant",
        "x-region":    "region",
        "x-env":       "env",
      },
      Fixed: map[string]string{ "primary": "true" }, // 例：常にprimary=true優先
    },
    GRPC: &sdk.GRPCLabelRules{
      MetaMap: map[string]string{
        "x-tenant-id": "tenant",
        "x-region":    "region",
      },
    },
    JWT: &sdk.JWTLabelRules{
      ClaimMap: map[string]string{
        "tid": "tenant",
      },
    },
    Ctx: &sdk.CtxValueRules{
      KeyMap: map[any]string{
        sdk.TenantIDKey{}: "tenant", // 既存の WithTenantID 由来
      },
    },
    Hint: &sdk.SelectionHint{
      Strategy:   sdk.SelectConsistentHash,
      HashSource: "tenant:acme", // 例（実際は ctx から拾って入れる実装にしてもOK）
    },
  }),
})
```

```go
// HTTP ミドルウェア例（router 側）
func inject(ctx context.Context, w http.ResponseWriter, r *http.Request) context.Context {
  ctx = sdk.WithHTTPRequest(ctx, r)
  if tid := r.Header.Get("x-tenant-id"); tid != "" {
    ctx = sdk.WithTenantID(ctx, tid)
  }
  // JWT パース済みクレームを入れる
  if claims := parseJWT(r); claims != nil {
    ctx = sdk.WithJWTClaims(ctx, claims)
  }
  return ctx
}
```

### Operational tips

- Resolution order: explicit `Key` > `Query` > legacy resolver (V1) > default target
- When queries often match many targets, `SelectConsistentHash` with a stable `HashSource` (e.g. tenant ID) balances load and preserves stickiness
- For redundant pairs like primary/secondary, set `SelectPreferLabel("primary=true")` as the default strategy
- Labels are normalized to lower-case, trimmed, and restricted to safe characters
- Emit DEBUG logs with the chosen key, collected labels, strategy, hash source, and candidate count to ease audits

### Transaction policy

Each target database operation uses its own transaction. Metadata is persisted
to the MetaDB in a separate transaction and the SDK does not attempt any
distributed commits. Audit logs and notifications are emitted only after the
MetaDB transaction commits.

### Remote HTTP

```go
cli := client.NewHTTP("https://api.example.com", client.WithToken("TOKEN"))
fields, _ := cli.List(ctx, "posts")
```

## Authentication

Run migrations then obtain a token with the seeded admin user. The login
endpoint expects a JSON body, so be sure to set the `Content-Type` header.
Before running `make db-init`, compile the CLI with `make build` and set the
`DB_DSN` environment variable to point at your database. `make db-init` simply
runs `fieldctl db migrate --seed` using this DSN and will reset the admin
password to `admin` if the user already exists. For example:

```bash
DB_DSN=postgres://user:pass@localhost:5432/app make build db-init
```

After seeding the admin account, obtain a token. The API server listens on
`8080` by default, but you can change it with the `-addr` flag if needed:

```bash
curl -X POST http://localhost:8080/v1/auth/login \
    -H 'Content-Type: application/json' \
    -H 'X-Tenant-ID: default' \
    -d '{"username":"admin","password":"admin123"}'
```

Example request to create a custom field with all options:

```bash
curl -X POST http://localhost:8080/v1/custom-fields \
     -H 'Content-Type: application/json' \
     -d '{"table":"posts","column":"foo","type":"varchar","display":{"labelKey":"field.foo.label","placeholderKey":"field.foo.ph","widget":"text"},"nullable":true,"unique":false,"default":"bar","validator":"uuid"}'
```

## Reserved Tables

Certain tables are protected from custom field modifications. Regex patterns are defined in `configs/default.yaml` and can be overridden with the `CF_RESERVED_TABLES` environment variable.

The metadata endpoint `/v1/metadata/tables?db_id=1` marks each table with a `reserved` flag so frontends can hide them.
## Events

| Name | Description |
|------|-------------|
| `cf.field.created` | Custom field added |
| `cf.field.updated` | Custom field updated |
| `cf.field.deleted` | Custom field deleted |


## クイックスタート

1. DB 初期化（スキーマ & admin 作成）

```bash
fieldctl db migrate --db postgres://user:pass@localhost:5432/app --schema public --seed
```

2. 追加ユーザー

```bash
fieldctl user create --db ... --username alice --password s3cr3t --role editor
```

## Metrics

| Metric | Description |
|--------|-------------|
| `cf_api_requests_total` | REST call counter |
| `cf_api_latency_seconds` | API latency histogram |
| `cf_fields_total` | Number of custom fields |
| `cf_apply_errors_total` | Apply failures by table |
| `cf_cache_hits_total` | Runtime cache hits |
| `cf_cache_misses_total` | Runtime cache misses |
| `cf_audit_events_total` | Audit log events |
| `cf_audit_errors_total` | Audit write errors |
| `cf_targets_total` | Registered targets |
| `cf_target_labels_total` | Targets per label |
| `cf_target_operation_seconds` | Registry operation latency |

### Quick Start

```bash
docker compose up -d prometheus grafana
```

Open <http://localhost:3000> (admin/admin) and load the **CustomField Overview** dashboard.

### Table prefix
If you keep your CF tables namespaced (e.g. `gcfm_custom_fields`), pass `--table-prefix gcfm_` or set `CF_TABLE_PREFIX=gcfm_`.
The migrator will automatically create `<prefix>registry_schema_version` on first run.


### 🔄 CI Drift Guard
1. PR ごとに PostgreSQL コンテナを起動  
2. `fieldctl db migrate --seed` で最新スキーマに  
3. `fieldctl diff --skip-reserved --fail-on-change` で registry.yaml と比較
4. 差分があれば PR に sticky コメント + ジョブ失敗


### マルチテナント
1. すべてのリクエストに `X-Tenant-ID: <tid>` ヘッダー、または JWT の `tid` クレームを付与してください。
2. 既存データを移行するには次のコマンドを実行します。
   ```
   fieldctl db migrate --seed --tenant default
   ```
3. CLI は `--tenant <id>` または環境変数 `CF_TENANT` を受け付けます。

### Snapshot & Rollback

```bash
fieldctl snapshot --bump minor
fieldctl revert --to 1.4.0
fieldctl diff-snap --from 1.3.0 --to 1.4.0
```
Endpoint docs: /docs/api/#tag/Snapshot.
