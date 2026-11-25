# `vue` Module Reference

Import: `github.com/krewire/hub/vue`

## Purpose & Domain Responsibility

`vue` is the frontend compiler adapter for Vue 3 Composition API islands. It implements `plugin.Plugin`, `plugin.Installer`, and `plugin.AssetProvider`, automatically registering `&Vue{}` in `plugin.Registry` via `init()`. It detects Vue projects, scaffolds `vite.config.js` and `components/Counter.vue`, manages npm dependencies, and builds assets through Vite.

## Architectural Position

- **Layer**: Layer 2 (Port & Adapter)
- **Module Path**: `vue/go.mod` (`github.com/krewire/hub/vue`)
- **Pure-Go Invariant**: Compiles cleanly with `CGO_ENABLED=0`
- **Dependencies**:
  - Third-party: None in Go (shells out to Vite / npm / bun / pnpm / yarn at build or install time)
  - Ecosystem: `github.com/krewire/hub/plugin`, `github.com/krewire/krewire/version`

## Core Concepts & APIs

### `Vue` Plugin Methods

- **`Name() string`**: Returns `"vue"`.
- **`Aliases() []string`**: Returns `[]string{"vuejs", "vue3"}`.
- **`Version() version.Version`**: Returns `3.5.0`.
- **`Detect(root string) bool`**: Checks in order:
  1. Vue config files (`vue.config.js`, `vue.config.ts`, `vue.config.mjs`, `vue.config.cjs`)
  2. `package.json` containing `"vue"` or `"@vitejs/plugin-vue"`
  3. `vite.config.js` / `vite.config.ts` / `vite.config.mjs` referencing `plugin-vue` or `vue()`
  4. `.vue` files in `components`, `src`, `web`, or `pages` (via `plugin.HasExtInDirs`)
- **`Build(root, outDir string) error`**: When `Detect(root)` is true, executes `plugin.BuildViteCandidateCommands(root, outDir)`. Runner failures log warnings and return `nil`.
- **`Assets(root string) []string`**: Returns `"assets/vue.js"` and/or `"assets/vue.css"` when present under `root`.
- **`Add(root, ver string) error`**: Creates `vite.config.js` (if missing) configured to output `vue.js` and `vue.[ext]` to `.krewire/build/assets`, creates `components/Counter.vue` (if missing), and installs `vue@<ver>`, `@vitejs/plugin-vue`, and `vite` as dev dependencies via `plugin.InstallPackage`.
- **`Remove(root string) error`**: Removes `components/Counter.vue` if it still contains the starter marker `"Vue 3 Composition Island"`, and uninstalls `vue` and `@vitejs/plugin-vue`.

## Usage Example

```go
package main

import (
	"fmt"
	"log"

	"github.com/krewire/hub/vue"
)

func main() {
	v := &vue.Vue{}
	root := "."

	if v.Detect(root) {
		if err := v.Build(root, ".krewire/build"); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Vue assets:", v.Assets(root))
	}
}
```

## Verification

```bash
cd /home/krewire/workspace/krewire/hub/vue
gofmt -l .
go vet ./...
CGO_ENABLED=0 go test -v ./...
CGO_ENABLED=0 go build ./...
```
