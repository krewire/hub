package packages

import (
	"errors"
	"testing"

	"github.com/krewire/libs/core"
)

// mustVersion parses v, failing the test if it is not valid semver.
func mustVersion(t *testing.T, v string) core.Version {
	t.Helper()
	ver, err := core.ParseVersion(v)
	if err != nil {
		t.Fatalf("ParseVersion(%q) error = %v", v, err)
	}
	return ver
}

// stubInstaller is an Installer that performs no I/O, so chain-resolution
// tests stay hermetic and never shell out to go or npm.
type stubInstaller struct{ kind string }

func (s stubInstaller) Kind() string                   { return s.kind }
func (s stubInstaller) Add(root, version string) error { return nil }
func (s stubInstaller) Remove(root string) error       { return nil }
func (s stubInstaller) Describe() string               { return s.kind + ":stub" }

// stubResolver claims a fixed set of names, or declines (nil names).
type stubResolver struct {
	kind  string
	names map[string]bool
	err   error
}

func (r stubResolver) Resolve(spec Spec) (Installer, string, error) {
	if r.err != nil {
		return nil, "", r.err
	}
	if !r.names[spec.Name] {
		return nil, "", nil
	}
	return stubInstaller{kind: r.kind}, r.kind, nil
}

func mustSpec(t *testing.T, raw string) Spec {
	t.Helper()
	spec, err := ParseSpec(raw)
	if err != nil {
		t.Fatalf("ParseSpec(%q) error = %v", raw, err)
	}
	return spec
}

func TestChainResolvesInOrder(t *testing.T) {
	// Both resolvers claim "twcss"; the first must win.
	chain := Chain{
		stubResolver{kind: "plugin", names: map[string]bool{"twcss": true}},
		stubResolver{kind: "gomod", names: map[string]bool{"twcss": true}},
	}
	resolved, err := chain.Resolve(mustSpec(t, "twcss@1.2.3"))
	if err != nil {
		t.Fatalf("Chain.Resolve error = %v", err)
	}
	if resolved.Kind != "plugin" {
		t.Errorf("Kind = %q, want %q (first match must win)", resolved.Kind, "plugin")
	}
	if resolved.Installer == nil {
		t.Fatal("Installer = nil, want non-nil")
	}
	if got := resolved.EffectiveVersion(); got != "1.2.3" {
		t.Errorf("EffectiveVersion() = %q, want %q", got, "1.2.3")
	}
}

func TestChainFallsThroughToLaterResolver(t *testing.T) {
	chain := Chain{
		stubResolver{kind: "plugin"}, // claims nothing
		stubResolver{kind: "gomod", names: map[string]bool{"github.com/foo/bar": true}},
	}
	resolved, err := chain.Resolve(mustSpec(t, "github.com/foo/bar@v1.2.3"))
	if err != nil {
		t.Fatalf("Chain.Resolve error = %v", err)
	}
	if resolved.Kind != "gomod" {
		t.Errorf("Kind = %q, want %q", resolved.Kind, "gomod")
	}
}

func TestChainNoResolverReturnsError(t *testing.T) {
	if _, err := (Chain{}).Resolve(mustSpec(t, "twcss")); err == nil {
		t.Fatal("Chain.Resolve error = nil, want error when no resolver claims the spec")
	}
}

func TestChainPropagatesResolverError(t *testing.T) {
	sentinel := errors.New("resolver failure")
	chain := Chain{stubResolver{err: sentinel}}
	if _, err := chain.Resolve(mustSpec(t, "twcss")); !errors.Is(err, sentinel) {
		t.Errorf("Chain.Resolve error = %v, want %v", err, sentinel)
	}
}

func TestGoResolver(t *testing.T) {
	r := &GoResolver{}
	cases := []struct {
		name     string
		raw      string
		wantKind string
	}{
		{"dotted host is a Go module", "github.com/foo/bar", "gomod"},
		{"golang.org module", "golang.org/x/net", "gomod"},
		{"slash without dot is not Go", "foo/bar", ""},
		{"bare name is not Go", "twcss", ""},
		{"scoped npm is not Go", "@scope/pkg", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ins, kind, err := r.Resolve(mustSpec(t, c.raw))
			if err != nil {
				t.Fatalf("Resolve(%q) error = %v", c.raw, err)
			}
			if c.wantKind == "" {
				if ins != nil {
					t.Errorf("Resolve(%q) installer = %v, want nil", c.raw, ins)
				}
				return
			}
			if ins == nil {
				t.Fatalf("Resolve(%q) installer = nil, want non-nil", c.raw)
			}
			if kind != c.wantKind {
				t.Errorf("Resolve(%q) kind = %q, want %q", c.raw, kind, c.wantKind)
			}
		})
	}
}

func TestNpmResolverIsFallback(t *testing.T) {
	ins, kind, err := (&NpmResolver{}).Resolve(mustSpec(t, "chalk"))
	if err != nil {
		t.Fatalf("Resolve error = %v", err)
	}
	if ins == nil {
		t.Fatal("Resolve installer = nil, want non-nil (npm is the fallback resolver)")
	}
	if kind != "npm" {
		t.Errorf("kind = %q, want %q", kind, "npm")
	}
}

func TestDefaultChainResolvesTailwindToPlugin(t *testing.T) {
	// "tailwindcss" is an npm package name, but Krewire ships it as a plugin;
	// DefaultChain tries plugin first so it must not fall through to npm.
	resolved, err := DefaultChain().Resolve(mustSpec(t, "tailwindcss"))
	if err != nil {
		t.Fatalf("DefaultChain().Resolve error = %v", err)
	}
	if resolved.Kind != "plugin" {
		t.Errorf("Kind = %q, want %q", resolved.Kind, "plugin")
	}
}

func TestDefaultChainResolvesGoModuleToGomod(t *testing.T) {
	// Go detection must precede npm so "github.com/..." is not read as npm.
	resolved, err := DefaultChain().Resolve(mustSpec(t, "github.com/foo/bar@v1.0.0"))
	if err != nil {
		t.Fatalf("DefaultChain().Resolve error = %v", err)
	}
	if resolved.Kind != "gomod" {
		t.Errorf("Kind = %q, want %q", resolved.Kind, "gomod")
	}
	if got := resolved.EffectiveVersion(); got != "v1.0.0" {
		t.Errorf("EffectiveVersion() = %q, want %q", got, "v1.0.0")
	}
}

func TestDefaultChainFallsBackToNpm(t *testing.T) {
	resolved, err := DefaultChain().Resolve(mustSpec(t, "chalk@5.3.0"))
	if err != nil {
		t.Fatalf("DefaultChain().Resolve error = %v", err)
	}
	if resolved.Kind != "npm" {
		t.Errorf("Kind = %q, want %q", resolved.Kind, "npm")
	}
	if !resolved.SatisfiesRequired(mustVersion(t, "5.0.0")) {
		t.Error("SatisfiesRequired(5.0.0) = false, want true for chalk@5.3.0")
	}
}
