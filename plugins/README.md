# Plugins

Custom validators and widgets can be built as Go plugins and loaded at runtime.

## Interfaces

```go
import sdkplugin "github.com/faciam-dev/gcfm/sdk/plugin"

// Validator implements custom validation logic.
type Validator interface {
    Name() string
    Validate(value any) error
}

// Widget provides metadata to render custom UI components.
type Widget interface {
    Name() string
    Schema() map[string]any
}
```

A plugin should export `Validators` and `Widgets` slices containing implementations.

## Building

Use the Go toolchain to build a plugin:

```bash
go build -buildmode=plugin -o email_validator.so ./examples/plugins/email_validator
```

## Usage in YAML

Reference a validator with the `custom://` scheme and widgets with `plugin://`.

```yaml
validator: custom://email
display:
  widget: plugin://calendar
```
