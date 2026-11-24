# Krewire Hub

Official plugin registry, third-party integrations, and ecosystem extension hub for Krewire.

## Architecture & Repositories

Krewire Hub is organized as a domain-first multi-module monorepo:
- `plugin`: Plugin lifecycle, contracts, and execution sandbox.
- `resolver`: Dynamic plugin dependency and version resolver.
- `redis`: Redis third-party integration driver adhering to Krewire plugin and protocol standards.

## License

MIT License
