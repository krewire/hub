# hub

**hub** is the Krewire ecosystem's official plugin and third-party integration registry.

## Packages

- **plugin** — Plugin contract and implementations for build-time extensions (Tailwind CSS, PostCSS, etc.)
- **packages** — Package resolver chain (`plugin → Go module → npm`) powering `kiw add` and `kiw remove`

## Overview

hub provides the scalable plugin and package infrastructure for Krewire:

- **Plugin Interface** — Minimal contract for build-time extensions (`Detect`, `Build`)
- **Installer Interface** — Optional lifecycle contract for `kiw add/remove <name>@<version>`
- **Registry** — Self-registering plugin collection via `init()`
- **Resolver Chain** — Seamless package resolution order across ecosystem plugins, Go modules, and npm packages

## Plugins Included

- **Tailwind CSS** — Auto-detects `tailwind.config.js`, compiles via Tailwind CLI, supports `kiw add twcss` / `kiw remove twcss`

## Usage

```go
import "github.com/krewire/hub/plugin"

// Register a custom build plugin
func init() {
    plugin.Register(&MyPlugin{})
}

// Resolve and install packages
import "github.com/krewire/hub/packages"

resolved, _ := packages.DefaultChain().Resolve(packages.ParseSpec("twcss@latest"))
resolved.Installer.Add(projectRoot, "latest")
```

## Architecture

Part of the Krewire unified framework vision ([KWF-M8K2Q](https://github.com/krewire/framework/blob/main/docs/specs/KWF-ARCH-M8K2Q-unified-framework-vision.md)).

- **Scalable** — New plugins require zero changes to the `kiw` CLI
- **Optional** — Plugin build errors do not fail the core site build
- **Declarative** — Activated by config file presence at the project root
