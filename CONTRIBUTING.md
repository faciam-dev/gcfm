# Contributing to go-custom-field-model

Thank you for considering a contribution! We welcome pull requests, issues, and feature proposals.

## Development workflow

1. Fork the repository and create a branch for your change.
2. Follow the project's Onion Architecture. Place new packages under `internal/` unless they are intended for external use.
3. Run `make lint` and `make test` to ensure code quality and tests pass.
4. Open a pull request describing your changes.

## Security practices

- Run `gosec ./...` and `go test ./...` after addressing vulnerabilities.
- Keep dependencies up to date and monitor advisories.
- Ensure security-related changes receive code review.

## Reporting issues

- Use the GitHub issue tracker.
- Include steps to reproduce and any relevant environment details.

## Code style

- Format code with `gofmt`.
- Add tests for new features and bug fixes.

We appreciate your help improving go-custom-field-model!

