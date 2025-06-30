# go-custom-field-model
[![Docs](https://img.shields.io/badge/docs-latest-blue)](https://faciam-dev.github.io/gcfm/)
A model system that provides custom fields

## Build

Run `make build` to compile the CLI and API server. Binaries will be placed in
the `bin` directory.


## fieldctl scan

Scan existing database tables and store the metadata in the `custom_fields` table.

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
    "log"

    "github.com/faciam-dev/gcfm/sdk"
)

func main() {
    svc := sdk.New(sdk.ServiceConfig{})
    ctx := context.Background()
    yaml, err := svc.Export(ctx, sdk.DBConfig{DSN: "mysql://user:pass@tcp(localhost:3306)/app", Schema: "app"})
    if err != nil {
        log.Fatal(err)
    }
    if _, err := svc.Apply(ctx, sdk.DBConfig{DSN: "mysql://user:pass@tcp(localhost:3306)/app", Schema: "app"}, yaml, sdk.ApplyOptions{}); err != nil {
        log.Fatal(err)
    }
}
```

## Authentication

Run migrations then obtain a token with the seeded admin user. The login
endpoint expects a JSON body, so be sure to set the `Content-Type` header.
Before running `make db-init`, compile the CLI with `make build` and set the
`DB_DSN` environment variable to point at your database. `make db-init` simply
runs `fieldctl db migrate --seed` using this DSN. For example:

```bash
DB_DSN=postgres://user:pass@localhost:5432/app make build db-init
```

After seeding the admin account, obtain a token:

```bash
curl -X POST http://localhost:8080/v1/auth/login \
     -H 'Content-Type: application/json' \
     -d '{"username":"admin","password":"admin123"}'
```

## クイックスタート

1. DB 初期化（スキーマ & admin 作成）

```bash
fieldctl db migrate --db postgres://user:pass@localhost:5432/app --schema public --seed
```

2. 追加ユーザー

```bash
fieldctl user create --db ... --username alice --password s3cr3t --role editor
```

