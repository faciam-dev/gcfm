# Package Layout

This project follows the onion architecture described in `AGENT.md`.
This document provides a quick reference of major Go packages.

## internal/

Application code organized into domain, usecase, infrastructure and interface layers.

## sdk/

Public SDK exposing high level custom field operations.

## pkg/

Shared utility libraries. Packages here can be imported by external programs.
Currently there is no `pkg/customfields`; custom field helpers reside under `sdk` and `internal/customfield`.
Adding such a package would require additional design and is deferred to a later stage.

