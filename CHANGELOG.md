# Changelog

## Unreleased
### Added
- Support for configuring a separate metadata database via `MetaDB`, `MetaDriver`, and `MetaSchema`.
- Support for multiple target databases via `Targets`, context-based selection with `TargetResolver`, and `TargetRegistry` for registration and iteration.

### Changed
- Metadata persistence is abstracted behind the `MetaStore` interface while remaining backward compatible.
- All monitoring access now routes through the selected target connection while metadata writes go through the `MetaStore`.
