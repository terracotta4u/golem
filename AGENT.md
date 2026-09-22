# AGENT.md

Golem is a extensible personal AI agent written in Go.

## Development

- Practice red-green development. Be sure to run the tests to make sure they fail. 

## Docs

- When making user-facing changes, update the relevant pages in `docs/`.

## SDK

- `sdk/` is the Python client for Golem's extension protocol (`golem-agent-sdk` on PyPI). Import as `golem`.
- When the extension protocol or its public API changes, update `sdk/`.
- When you update the SDK, update the relevant pages in `docs/` too. 
