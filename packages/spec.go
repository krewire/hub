package packages

import (
	"fmt"
	"strings"

	"github.com/krewire/libs/core"
)

// Spec is a parsed package@version reference.
type Spec struct {
	Raw     string       // original input e.g. "twcss@1.2.3" or "pkg@latest"
	Name    string       // package name e.g. "twcss" or "@scope/pkg" or "github.com/foo/bar"
	Version core.Version // parsed semantic version
	RawVer  string       // raw version string from input (e.g. "latest", "1.2.3", "v1.0.0")
}

// ParseSpec parses raw package@version. It supports:
//   - "pkg"                → {Name:"pkg", Version:core.Version{}}
//   - "pkg@1.2.3"           → {Name:"pkg", Version:{Major:1,Minor:2,Patch:3}}
//   - "pkg@latest"          → {Name:"pkg", Version:{}, RawVer:"latest"}
//   - "@scope/pkg@1.0.0"    → {Name:"@scope/pkg", Version:{Major:1,Patch:0}}
//   - "@scope/pkg"          → {Name:"@scope/pkg", Version:{}}
//   - "github.com/foo/bar@v1.2.3" → {Name:"github.com/foo/bar", Version:{Major:1,Minor:2,Patch:3}}
func ParseSpec(raw string) (Spec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Spec{}, fmt.Errorf("empty package spec")
	}
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
			v, err := core.ParseVersion(ver)
			if err != nil {
				return Spec{}, err
			}
			return Spec{Raw: raw, Name: name, Version: v, RawVer: ver}, nil
		}
		return Spec{Raw: raw, Name: raw, Version: core.Version{}}, nil
	}
	// Non-scoped: split at last @ (to handle github.com/foo/bar@v1.2.3)
	if at := strings.LastIndex(raw, "@"); at != -1 {
		name := raw[:at]
		ver := raw[at+1:]
		if name == "" || ver == "" {
			return Spec{}, fmt.Errorf("invalid package spec %q", raw)
		}
		v, err := core.ParseVersion(ver)
		if err != nil {
			// Allow "latest" as special case
			if ver == "latest" {
				return Spec{Raw: raw, Name: name, Version: core.Version{}, RawVer: "latest"}, nil
			}
			return Spec{}, err
		}
		return Spec{Raw: raw, Name: name, Version: v, RawVer: ver}, nil
	}
	return Spec{Raw: raw, Name: raw, Version: core.Version{}}, nil
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
func (s Spec) SemVer() core.Version {
	return s.Version
}

// IsLatest reports whether the version is "latest" or empty.
func (s Spec) IsLatest() bool {
	return s.RawVer == "" || s.RawVer == "latest"
}

// Satisfies reports whether this spec's version satisfies the required version
// per semver caret semantics (core.Version.IsCompatible).
func (s Spec) Satisfies(required core.Version) bool {
	if s.IsLatest() {
		return true // latest satisfies any requirement
	}
	return s.Version.IsCompatible(required)
}
