# AGENT.md

Golem is a extensible personal AI agent written in Go.

## Development

- Practice red-green development. Be sure to run the tests to make sure they fail. 

## Styling

Follow these instructions when working with HTML or CSS: 

- For base styles, always consult: `server/web/static/terracotta-ui/`. These files will help with colors, layout, and typography.
- Colors choices should first consult `terracotta-ui/theme.css`, then `colors.css`.
- Do not edit files in `terracotta-ui/`. Those styles are copied from the terracotta-ui package.

## Docs

- When making user-facing changes, update the relevant pages in `docs/`.

## SDK

- `sdk/` is the Python client for Golem's extension protocol (`golem-agent-sdk` on PyPI). Import as `golem`.
- When the extension protocol or its public API changes, update `sdk/`.
- When you update the SDK, update the relevant pages in `docs/` too. 
