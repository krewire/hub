# `svelte` Module Reference

Import: `github.com/krewire/hub/svelte`

## Purpose & Domain Responsibility

`svelte` is the frontend compiler adapter for Svelte 5 reactive islands. It implements `plugin.Plugin`, `plugin.Installer`, and `plugin.AssetProvider`, automatically registering `&Svelte{}` in `plugin.Registry` via `init()`. It handles project detection, `svelte.config.js` / `vite.config.js` / starter component scaffolding, npm package installation, and Vite production builds.

## Architectural Position

- **Layer**: Layer 2 (Port & Adapter)
- **Module Path**: `svelte/go.mod` (`github.com/krewire/hub/svelte`)
- **Pure-Go Invariant**: Compiles cleanly with `CGO_ENABLED=0`
- **Dependencies**:
  - Third-party: None in Go (shells out to Vite / npm / bun / pnpm / yarn at build or install time)
  - Ecosystem: `github.com/krewire/hub/plugin`, `github.com/krewire/krewire/version`

## Core Concepts & APIs

### Constants & Templates

- **Constants**: `ConfigFile` (`"svelte.config.js"`), `NpmPackage` (`"svelte"`), `PluginPackage` (`"@sveltejs/vite-plugin-svelte"`), `AssetJS` (`"assets/svelte.js"`), `AssetCSS` (`"assets/svelte.css"`).
- **`ConfigFiles`**: `svelte.config.js`, `svelte.config.mjs`, `svelte.config.ts`, `svelte.config.cjs`.
- **Templates**: `ConfigTemplate`, `ViteConfigTemplate` (configured to emit `svelte.js` and `svelte.[ext]` into `.krewire/build/assets`), and `StarterTemplate` (`components/Counter.svelte` using Svelte 5 `$state(0)`).

### `Svelte` Plugin Methods

- **`Name() string`**: Returns `"svelte"`.
- **`Aliases() []string`**: Returns `[]string{"sveltejs"}`.
- **`Version() version.Version`**: Returns `5.0.0`.
- **`Detect(root string) bool`**: Returns `true` if any `ConfigFiles` exist, `package.json` references `"svelte"` or `"@sveltejs/vite-plugin-svelte"`, or `.svelte` files exist under `components`, `src`, `web`, or `pages` (via `plugin.HasExtInDirs`).
- **`Build(root, outDir string) error`**: When `Detect(root)` is true, tries `plugin.BuildViteCandidateCommands(root, outDir)` in order. Build failures log warnings and return `nil` so missing JS toolchains do not abort pure-Go builds.
- **`Assets(root string) []string`**: Returns `AssetJS` and/or `AssetCSS` when present under `root`.
- **`Add(root, ver string) error`**: Runs `EnsureConfig`, `EnsureViteConfig`, and `EnsureStarter`, then installs `svelte@<ver>`, `@sveltejs/vite-plugin-svelte`, and `vite` as dev dependencies via `plugin.InstallPackage`.
- **`Remove(root string) error`**: Removes `ConfigFiles`, deletes `components/Counter.svelte` only if it still matches the starter marker `"Svelte 5 Reactive Island"`, and uninstalls `svelte` and `@sveltejs/vite-plugin-svelte`.

### Scaffolding Helpers

- **`EnsureConfig(root string) error`**, **`EnsureViteConfig(root string) error`**, and **`EnsureStarter(root string) error`**: Idempotent helpers that create missing configuration and starter files without overwriting existing user files.

## Usage Example

```go
package main

import (
	"log"

	"github.com/krewire/hub/svelte"
)

func main() {
	s := &svelte.Svelte{}
	root := "./my-site"

	if err := svelte.EnsureConfig(root); err != nil {
		log.Fatal(err)
	}
	if s.Detect(root) {
		if err := s.Build(root, ".krewire/build"); err != nil {
			log.Fatal(err)
		}
	}
}
```

## Verification

```bash
cd /home/krewire/workspace/krewire/hub/svelte
gofmt -l .
go vet ./...
CGO_ENABLED=0 go test -v ./...
CGO_ENABLED=0 go build ./...
```
