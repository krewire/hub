# `plugin` Module Reference

Import: `github.com/krewire/hub/plugin`

## Purpose & Domain Responsibility

`plugin` defines the extensible site-build plugin contract for Krewire. Frontend compilers (`svelte`, `tailwind`, `vue`) implement these interfaces and self-register into `plugin.Registry` via `init()`, allowing `krew build` and `kiw add` / `kiw remove` to discover, install, build, and inject assets for third-party frontend toolchains without coupling the core CLI or framework to any specific compiler.

## Architectural Position

- **Layer**: Layer 2 (Port & Adapter)
- **Module Path**: `plugin/go.mod` (`github.com/krewire/hub/plugin`)
- **Pure-Go Invariant**: Compiles cleanly with `CGO_ENABLED=0`
- **Dependencies**:
  - Third-party: None (Go standard library only)
  - Ecosystem: `github.com/krewire/krewire/version`

## Core Concepts & APIs

### Plugin Contracts (`plugin.go`)

- **`Plugin`**: Core interface every build extension implements:
  - `Name() string` — canonical identifier (e.g., `"tailwind"`, `"svelte"`, `"vue"`)
  - `Aliases() []string` — alternative lookup names (e.g., `"twcss"`)
  - `Detect(root string) bool` — reports whether the plugin applies to the project at `root`
  - `Build(root, outDir string) error` — compiles assets into `outDir`
  - `Version() version.Version` — baseline version for semver compatibility checks
- **`Installer`**: Optional interface embedding `Plugin` for lifecycle management (`Add(root, version string) error`, `Remove(root string) error`).
- **`AssetProvider`**: Optional interface embedding `Plugin` for automatic `<link>`/`<script>` injection (`Assets(root string) []string` returning logical paths such as `"assets/tailwind.css"`).

### Registry & Lookup

- **`Registry []Plugin`** and **`Register(p Plugin)`**: Global slice populated by plugin `init()` functions.
- **`Find(name string) Plugin`**: Case-insensitive lookup matching `Name()`, `Aliases()`, plus the special mapping `"tailwindcss"` → `"tailwind"`.
- **`FindInstaller(name string) Installer`**: Returns the `Installer` if `Find(name)` implements `Installer`.
- **`FindByVersion(name string, required version.Version) Plugin`** and **`FindInstallerByVersion(name string, required version.Version) Installer`**: Filter matches using `p.Version().IsCompatible(required)` semver caret semantics.

### Package Manager & Build Helpers (`helper.go`)

- **Constants**: `ProjectFilePerm` (`0o644`) and `ProjectDirPerm` (`0o755`).
- **`DetectPackageManager(root string) string`**: Checks lockfiles (`bun.lockb`, `bun.lock`, `pnpm-lock.yaml`, `yarn.lock`) paired with `exec.LookPath`, falling back to `npm`, `bun`, `pnpm`, `yarn` on `PATH`.
- **`InstallPackage(root, pkg string, dev bool) error`** and **`UninstallPackage(root, pkg string) error`**: Run the detected package manager's install/add or uninstall/remove command in `root`.
- **`HasExtInDirs(root string, dirs []string, ext string) bool`**: Walks candidate directories (skipping `node_modules` and `.git`) to detect files matching `ext`.
- **`BuildViteCandidateCommands(root, outDir string) [][]string`**: Generates prioritized Vite build invocations (`./node_modules/.bin/vite`, `./bin/vite`, `vite`, `bunx`, `pnpm exec`, `yarn`, `npx`) targeting `<outDir>/assets`.

## Usage Example

```go
package main

import (
	"fmt"
	"log"

	"github.com/krewire/hub/plugin"
	_ "github.com/krewire/hub/tailwind"
)

func main() {
	p := plugin.Find("twcss")
	if p == nil {
		log.Fatal("tailwind plugin not registered")
	}

	root := "."
	if p.Detect(root) {
		if err := p.Build(root, ".krewire/build"); err != nil {
			log.Fatal(err)
		}
		if ap, ok := p.(plugin.AssetProvider); ok {
			fmt.Println("Emitted assets:", ap.Assets(root))
		}
	}
}
```

## Verification

```bash
cd /home/krewire/workspace/krewire/hub/plugin
gofmt -l .
go vet ./...
CGO_ENABLED=0 go test -v ./...
CGO_ENABLED=0 go build ./...
```
