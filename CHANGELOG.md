# Changelog

## Unreleased
### Added
- Support for configuring a separate metadata database via `MetaDB`, `MetaDriver`, and `MetaSchema`.

### Changed
- Metadata persistence is abstracted behind the `MetaStore` interface while remaining backward compatible.
