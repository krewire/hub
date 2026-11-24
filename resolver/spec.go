package resolver

import (
	"errors"
	"fmt"
	"strings"

	"github.com/krewire/krewire/version"
)

// Spec is a parsed package@version reference.
type Spec struct {
	Raw     string          // original input e.g. "twcss@1.2.3" or "pkg@latest"
	Name    string          // package name e.g. "twcss" or "@scope/pkg" or "github.com/foo/bar"
	Version version.Version // parsed semantic version
	RawVer  string          // raw version string from input (e.g. "latest", "1.2.3", "v1.0.0")
}

// ErrUnsafePackageName reports a package name that must not reach a package
// manager. npm and the Go toolchain both interpret path-like and URL-like
// specifiers ("file:../..", "git+ssh://...") and run package lifecycle scripts,
// so an unvalidated name is a command-execution and path-traversal vector.
var ErrUnsafePackageName = errors.New("packages: unsafe package name")

// unsafeNameChars are characters that must never appear in a package name.
// Whitespace (including newline) is excluded so a name cannot inject additional
// arguments or options into the package manager CLI.
func unsafeNameChar(r rune) bool {
	if r <= 0x20 || r == 0x7f {
		return true
	}
	switch r {
	case '\\', '"', '\'', '`', '$', '<', '>', '|', '&', ';', '*', '?', '!', '#', '%':
		return true
	}
	return false
}

// validatePackageName rejects specifiers that let a caller redirect resolution
// outside the registry or inject CLI options.
//
// Accepted forms mirror what the resolver chain supports: a bare npm name, a
// scoped npm name, and a Go module path. Anything carrying a scheme, a path
// traversal segment, or a parent-directory reference is refused.
func validatePackageName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: empty name", ErrUnsafePackageName)
	}
	if len(name) > 214 {
		// npm's own limit; keeps absurd inputs out of the CLI.
		return fmt.Errorf("%w: name exceeds 214 characters", ErrUnsafePackageName)
	}
	for _, r := range name {
		if unsafeNameChar(r) {
			return fmt.Errorf("%w: %q contains an illegal character", ErrUnsafePackageName, name)
		}
	}
	// A scheme-like prefix ("file:", "git+ssh:", "http:") makes the package
	// manager fetch from an arbitrary location.
	if i := strings.Index(name, ":"); i > 0 && !strings.Contains(name[:i], "/") {
		return fmt.Errorf("%w: %q looks like a URL or alternate source", ErrUnsafePackageName, name)
	}
	// Path traversal in any position, for both separators.
	if strings.Contains(name, "..") || strings.Contains(name, "./") {
		return fmt.Errorf("%w: %q contains a path traversal segment", ErrUnsafePackageName, name)
	}
	if strings.HasPrefix(name, "/") || strings.Contains(name, `:\`) {
		return fmt.Errorf("%w: %q is an absolute path", ErrUnsafePackageName, name)
	}
	return nil
}

// ParseSpec parses raw package@version. It supports:
//   - "pkg"                → {Name:"pkg", Version:version.Version{}}
//   - "pkg@1.2.3"           → {Name:"pkg", Version:{Major:1,Minor:2,Patch:3}}
//   - "pkg@latest"          → {Name:"pkg", Version:{}, RawVer:"latest"}
//   - "@scope/pkg@1.0.0"    → {Name:"@scope/pkg", Version:{Major:1,Patch:0}}
//   - "@scope/pkg"          → {Name:"@scope/pkg", Version:{}}
//   - "github.com/foo/bar@v1.2.3" → {Name:"github.com/foo/bar", Version:{Major:1,Minor:2,Patch:3}}
//
// SECURITY: the name is validated, so a spec that would make npm or the Go
// toolchain fetch from an arbitrary location or escape the project directory is
// rejected before any installer runs.
func ParseSpec(raw string) (Spec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Spec{}, fmt.Errorf("empty package spec")
	}
	spec, err := parseSpecUnchecked(raw)
	if err != nil {
		return Spec{}, err
	}
	if err := validatePackageName(spec.Name); err != nil {
		return Spec{}, err
	}
	return spec, nil
}

func parseSpecUnchecked(raw string) (Spec, error) {
	// Scoped npm package: starts with @
	if strings.HasPrefix(raw, "@") {
		// find second @ after scope/name
		slash := strings.Index(raw, "/")
		if slash == -1 {
			return Spec{}, fmt.Errorf("invalid scoped package %q", raw)
		}
		rest := raw[slash+1:]
		if at := strings.LastIndex(rest, "@"); at != -1 {
			name := raw[:slash+1+at]
			ver := rest[at+1:]
			if ver == "" {
				return Spec{}, fmt.Errorf("invalid version in %q", raw)
			}
			v, err := version.ParseVersion(ver)
			if err != nil {
				return Spec{}, err
			}
			return Spec{Raw: raw, Name: name, Version: v, RawVer: ver}, nil
		}
		return Spec{Raw: raw, Name: raw, Version: version.Version{}}, nil
	}
	// Non-scoped: split at last @ (to handle github.com/foo/bar@v1.2.3)
	if at := strings.LastIndex(raw, "@"); at != -1 {
		name := raw[:at]
		ver := raw[at+1:]
		if name == "" || ver == "" {
			return Spec{}, fmt.Errorf("invalid package spec %q", raw)
		}
		v, err := version.ParseVersion(ver)
		if err != nil {
			// Allow "latest" as special case
			if ver == "latest" {
				return Spec{Raw: raw, Name: name, Version: version.Version{}, RawVer: "latest"}, nil
			}
			return Spec{}, err
		}
		return Spec{Raw: raw, Name: name, Version: v, RawVer: ver}, nil
	}
	return Spec{Raw: raw, Name: raw, Version: version.Version{}}, nil
}

// EffectiveVersion returns version to use for install ("" or "latest" → "latest").
func (s Spec) EffectiveVersion() string {
	if s.RawVer == "" || s.RawVer == "latest" {
		return "latest"
	}
	return s.RawVer
}

// String returns canonical package@version (omits version if latest/empty).
func (s Spec) String() string {
	if s.RawVer == "" || s.RawVer == "latest" {
		return s.Name + "@latest"
	}
	return s.Name + "@" + s.RawVer
}

// SemVer returns the parsed semantic version, zero if not a valid semver.
func (s Spec) SemVer() version.Version {
	return s.Version
}

// IsLatest reports whether the version is "latest" or empty.
func (s Spec) IsLatest() bool {
	return s.RawVer == "" || s.RawVer == "latest"
}

// Satisfies reports whether this spec's version satisfies the required version
// per semver caret semantics (version.Version.IsCompatible).
func (s Spec) Satisfies(required version.Version) bool {
	if s.IsLatest() {
		return true // latest satisfies any requirement
	}
	return s.Version.IsCompatible(required)
}
