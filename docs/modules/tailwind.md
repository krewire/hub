# `tailwind` Module Reference

Import: `github.com/krewire/hub/tailwind`

## Purpose & Domain Responsibility

`tailwind` is the frontend CSS compiler adapter for Tailwind CSS (supporting both v3 and v4 conventions). It implements `plugin.Plugin`, `plugin.Installer`, and `plugin.AssetProvider`, self-registering `&Tailwind{}` in `plugin.Registry` via `init()`. It detects Tailwind configs or CSS directives, compiles minified stylesheets using either Node-based runners or the standalone `tailwindcss` binary, and scaffolds `tailwind.config.js` and `assets/tailwind.css`.

## Architectural Position

- **Layer**: Layer 2 (Port & Adapter)
- **Module Path**: `tailwind/go.mod` (`github.com/krewire/hub/tailwind`)
- **Pure-Go Invariant**: Compiles cleanly with `CGO_ENABLED=0`
- **Dependencies**:
  - Third-party: None in Go (invokes `tailwindcss` CLI or `bunx`/`pnpm`/`yarn`/`npx` at build time)
  - Ecosystem: `github.com/krewire/hub/plugin`, `github.com/krewire/krewire/version`

## Core Concepts & APIs

### Detection & Input Resolution

- **`ConfigFiles`**: `tailwind.config.js`, `tailwind.config.cjs`, `tailwind.config.mjs`, `tailwind.config.ts`, `tailwind.config.json`.
- **`Name() string`**: Returns `"tailwind"`.
- **`Aliases() []string`**: Returns `[]string{"twcss", "tailwindcss"}`.
- **`Version() version.Version`**: Returns `3.4.0`.
- **`Detect(root string) bool`**: Checks in order:
  1. Presence of any file in `ConfigFiles`
  2. `package.json` containing `"tailwindcss"` or `"@tailwindcss/cli"`
  3. Common stylesheets (`assets/tailwind.css`, `assets/style.css`, `assets/app.css`, `styles/tailwind.css`) containing `@tailwind`, `@import "tailwindcss"`, `@import 'tailwindcss'`, or `@theme`

### Build & Asset Discovery

- **`Build(root, outDir string) error`**: Resolves the input stylesheet, creates `<outDir>/<inputDir>`, and tries candidate CLI runners in priority order:
  1. Local `./node_modules/.bin/tailwindcss` and `./node_modules/.bin/@tailwindcss/cli`
  2. Standalone binary (`./bin/tailwindcss`, `./tailwindcss`, or `tailwindcss` on `PATH` — ideal for Node-free Go setups)
  3. `bunx`, `pnpm exec` / `pnpm dlx`, `yarn`
  4. `npx` fallback
  All runners invoke `-i <input> -o <output> --minify`. If no CLI is available or a runner fails, `Build` logs a warning and returns `nil` so Tailwind remains an optional build step.
- **`Assets(root string) []string`**: Returns the resolved logical stylesheet path (e.g., `"assets/tailwind.css"`) if the input file exists under `root`.

### Installation & Removal

- **`Add(root, version string) error`**: Scaffolds `tailwind.config.js` (configured for `.kiw`, `.html`, `.md`, and `.go` content globs plus Krewire theme CSS variables) and `assets/tailwind.css`, then installs `tailwindcss@<version>` as a dev dependency via `plugin.InstallPackage`.
- **`Remove(root string) error`**: Deletes `ConfigFiles`, removes `assets/tailwind.css` only if it matches the unmodified starter template, and uninstalls `tailwindcss`.

## Usage Example

```go
package main

import (
	"fmt"
	"log"

	"github.com/krewire/hub/tailwind"
)

func main() {
	tw := &tailwind.Tailwind{}
	root := "."

	if tw.Detect(root) {
		if err := tw.Build(root, ".krewire/build"); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Tailwind assets:", tw.Assets(root))
	}
}
```

## Verification

```bash
cd /home/krewire/workspace/krewire/hub/tailwind
gofmt -l .
go vet ./...
CGO_ENABLED=0 go test -v ./...
CGO_ENABLED=0 go build ./...
```
