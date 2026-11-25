# `resolver` Module Reference

Import: `github.com/krewire/hub/resolver`

## Purpose & Domain Responsibility

`resolver` implements cascading package and plugin resolution for `package@version` specifiers. It parses and validates package inputs against injection and path-traversal attacks, then routes each `Spec` through an ordered `Chain` of resolvers (`PluginResolver` → `GoResolver` → `NpmResolver`) to produce a uniform `Installer`.

Importing `resolver` also blank-imports `github.com/krewire/hub/svelte`, `github.com/krewire/hub/tailwind`, and `github.com/krewire/hub/vue` so all built-in Krewire plugins are automatically registered in `plugin.Registry`.

## Architectural Position

- **Layer**: Layer 2 (Port & Adapter)
- **Module Path**: `resolver/go.mod` (`github.com/krewire/hub/resolver`)
- **Pure-Go Invariant**: Compiles cleanly with `CGO_ENABLED=0`
- **Dependencies**:
  - Third-party: None (shells out to `go` or `npm` when installing `gomod`/`npm` packages)
  - Ecosystem: `github.com/krewire/hub/plugin`, `github.com/krewire/hub/svelte`, `github.com/krewire/hub/tailwind`, `github.com/krewire/hub/vue`, `github.com/krewire/krewire/version`

## Core Concepts & APIs

### Spec Parsing & Security Validation (`spec.go`)

- **`Spec`**: `{ Raw string, Name string, Version version.Version, RawVer string }`.
- **`ErrUnsafePackageName`**: Sentinel error returned when a package name fails `validatePackageName`.
- **`ParseSpec(raw string) (Spec, error)`**: Parses bare names (`"pkg"`), versioned names (`"pkg@1.2.3"`), `"pkg@latest"`, scoped npm packages (`"@scope/pkg@1.0.0"`), and Go module paths (`"github.com/foo/bar@v1.2.3"`). Before returning, it rejects:
  - Empty names or names exceeding 214 characters
  - Control characters, whitespace, or shell/CLI metacharacters (`\`, `"`, `'`, `` ` ``, `$`, `<`, `>`, `|`, `&`, `;`, `*`, `?`, `!`, `#`, `%`)
  - URL/scheme prefixes (`file:`, `git+ssh:`, `http:`)
  - Path traversal segments (`..`, `./`) or absolute paths (`/`, `:\`)
- **`Spec` Methods**: `EffectiveVersion()`, `String()`, `SemVer()`, `IsLatest()`, and `Satisfies(required version.Version) bool` (treats `"latest"` as satisfying any requirement).

### Cascading Resolution (`resolver.go`)

- **`Installer`**: `{ Kind() string, Add(root, version string) error, Remove(root string) error, Describe() string }`.
- **`Resolved`**: `{ Spec Spec, Installer Installer, Kind string }` (`"plugin"`, `"npm"`, or `"gomod"`), with `EffectiveVersion()` and `SatisfiesRequired(required)`.
- **`Resolver`** & **`Chain []Resolver`**: `Chain.Resolve(spec Spec) (*Resolved, error)` re-validates `spec.Name` at the boundary and tries each resolver in order until one returns a non-nil `Installer`.
- **`DefaultChain() Chain`**: Returns `Chain{&PluginResolver{}, &GoResolver{}, &NpmResolver{}}`:
  1. **`PluginResolver`**: Checks `plugin.FindInstaller(spec.Name)` first so `twcss`, `tailwind`, `svelte`, or `vue` resolve to Krewire plugins rather than raw npm packages.
  2. **`GoResolver`**: Matches `@krewire/<pkg>` (expanding to `github.com/krewire/<pkg>`) or module paths containing `/` with a `.` in the first path segment (e.g., `github.com/...`), delegating to `go get` / `go mod edit -droprequire`.
  3. **`NpmResolver`**: Fallback for any remaining valid package name, delegating to `npm install` / `npm uninstall`.

## Usage Example

```go
package main

import (
	"fmt"
	"log"

	"github.com/krewire/hub/resolver"
)

func main() {
	spec, err := resolver.ParseSpec("twcss@latest")
	if err != nil {
		log.Fatal(err)
	}

	resolved, err := resolver.DefaultChain().Resolve(spec)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Resolved %s as kind=%s (%s)\n",
		resolved.Spec.String(), resolved.Kind, resolved.Installer.Describe())
}
```

## Verification

```bash
cd /home/krewire/workspace/krewire/hub/resolver
gofmt -l .
go vet ./...
CGO_ENABLED=0 go test -v ./...
CGO_ENABLED=0 go build ./...
```
