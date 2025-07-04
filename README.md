# go-custom-field-model
[![Docs](https://img.shields.io/badge/docs-latest-blue)](https://faciam-dev.github.io/gcfm/)
A model system that provides custom fields

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

## Supported Drivers

| Driver | Package | Test Container |
|--------|---------|----------------|
| MySQL | internal/driver/mysql | mysql:8 |
| PostgreSQL | internal/driver/postgres | postgres:16 |
| MongoDB | internal/driver/mongo | mongo:7 |

### CI Drift Check

1. Push PR → GitHub Action runs `fieldctl db migrate` and `fieldctl apply` before
   executing **fieldctl diff**. These steps ensure the temporary database schema
   matches the registry file.
2. If diff exists, PR is marked ❌ and a sticky comment shows the delta.

#### Local
```bash
fieldctl db migrate --db $DSN --schema public --driver postgres
fieldctl apply --db $DSN --schema public --driver postgres --file registry.yaml
fieldctl diff --db $DSN --schema public --file registry.yaml
```

## Runtime Cache

Use the runtime cache to load custom field metadata at startup and reload it periodically:

```go
import (
    runtimecache "github.com/faciam-dev/gcfm/internal/customfield/runtime/cache"
    "github.com/faciam-dev/gcfm/internal/customfield/registry"
    mysqlscanner "github.com/faciam-dev/gcfm/internal/driver/mysql"
    "time"
)

db, _ := sql.Open("mysql", "user:pass@tcp(localhost:3306)/app")
sc := mysqlscanner.NewScanner(db)
cache, err := runtimecache.New(ctx, sc, registry.DBConfig{Schema: "app"}, time.Minute)
```


## Plugin 作成ガイド

カスタムバリデータを Go プラグインとして追加できます。最小構成は以下の通りです。

```go
package main

import (
    "github.com/faciam-dev/gcfm/internal/customfield"
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
password to `admin123` if the user already exists. For example:

```bash
DB_DSN=postgres://user:pass@localhost:5432/app make build db-init
```

After seeding the admin account, obtain a token. The API server listens on
`8080` by default, but you can change it with the `-addr` flag if needed:

```bash
curl -X POST http://localhost:8080/v1/auth/login \
     -H 'Content-Type: application/json' \
     -d '{"username":"admin","password":"admin123"}'
```

Example request to create a custom field with all options:

```bash
curl -X POST http://localhost:8080/v1/custom-fields \
     -H 'Content-Type: application/json' \
     -d '{"table":"posts","column":"foo","type":"varchar","display":{"labelKey":"field.foo.label","placeholderKey":"field.foo.ph","widget":"text"},"nullable":true,"unique":false,"default":"bar","validator":"uuid"}'
```

## Reserved Tables

Certain tables are protected from custom field modifications. The default list is stored in `configs/default.yaml` and can be overridden with the `CF_RESERVED_TABLES` environment variable.

The metadata endpoint `/v1/metadata/tables` marks each table with a `reserved` flag so frontends can hide them.
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

### Quick Start

```bash
docker compose up -d prometheus grafana
```

Open <http://localhost:3000> (admin/admin) and load the **CustomField Overview** dashboard.

