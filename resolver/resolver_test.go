package resolver

import (
	"errors"
	"testing"

	"github.com/krewire/krewire/version"
)

// mustVersion parses v, failing the test if it is not valid semver.
func mustVersion(t *testing.T, v string) version.Version {
	t.Helper()
	ver, err := version.ParseVersion(v)
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
		t.Fatalf("chain must propagate the resolver error, got %v", err)
	}
}

// TestPluginResolverWins verifies a registered plugin name resolves
// through the plugin resolver rather than falling through to npm, so `kiw add
// twcss` reaches the Krewire plugin instead of the npm package of the same name.
func TestPluginResolverWins(t *testing.T) {
	resolved, err := DefaultChain().Resolve(Spec{Name: "twcss", Raw: "twcss"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Kind != "plugin" {
		t.Errorf("Kind = %q, want plugin", resolved.Kind)
	}
	if resolved.Installer.Kind() != "plugin" {
		t.Errorf("Installer.Kind = %q", resolved.Installer.Kind())
	}
	if !containsPrefix(resolved.Installer.Describe(), "plugin:") {
		t.Errorf("Describe = %q", resolved.Installer.Describe())
	}
}

// TestGoResolverHandlesModulePaths verifies a module path is claimed
// by the Go resolver rather than npm.
func TestGoResolverHandlesModulePaths(t *testing.T) {
	for _, name := range []string{"github.com/krewire/krewire/pkg", "golang.org/x/net"} {
		resolved, err := DefaultChain().Resolve(Spec{Name: name, Raw: name})
		if err != nil {
			t.Fatalf("Resolve(%q): %v", name, err)
		}
		if resolved.Kind != "gomod" {
			t.Errorf("Resolve(%q).Kind = %q, want gomod", name, resolved.Kind)
		}
	}
}

// TestNpmFallback verifies an ordinary package name falls through to
// the npm resolver.
func TestNpmFallback(t *testing.T) {
	resolved, err := DefaultChain().Resolve(Spec{Name: "left-pad", Raw: "left-pad"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Kind != "npm" {
		t.Errorf("Kind = %q, want npm", resolved.Kind)
	}
	if resolved.Installer.Describe() != "npm:left-pad" {
		t.Errorf("Describe = %q", resolved.Installer.Describe())
	}
}

// TestGoResolverHeuristic documents the first-segment dot heuristic:
// a slash without a dotted host is not a module path, so it falls to npm.
func TestGoResolverHeuristic(t *testing.T) {
	resolver := &GoResolver{}
	ins, kind, err := resolver.Resolve(Spec{Name: "scope/package"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ins != nil || kind != "" {
		t.Errorf("a scope-style name must not be claimed as a Go module, got %v/%q", ins, kind)
	}
	if _, _, err := (&GoResolver{}).Resolve(Spec{Name: "github.com/x/y"}); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
}

// TestEmptySpecResolvesToNothing verifies a blank name is declined by
// each resolver rather than producing an installer that would run a package
// manager with an empty argument.
func TestEmptySpecResolvesToNothing(t *testing.T) {
	if ins, _, _ := (&NpmResolver{}).Resolve(Spec{}); ins != nil {
		t.Error("the npm resolver must decline an empty name")
	}
	if ins, _, _ := (&GoResolver{}).Resolve(Spec{}); ins != nil {
		t.Error("the Go resolver must decline an empty name")
	}
	if ins, _, _ := (&PluginResolver{}).Resolve(Spec{Name: "definitely-not-registered"}); ins != nil {
		t.Error("the plugin resolver must decline an unknown plugin")
	}
}

// TestChainReportsUnresolvable verifies a chain with no capable
// resolver returns a clear error naming the package.
func TestChainReportsUnresolvable(t *testing.T) {
	empty := Chain{}
	if _, err := empty.Resolve(Spec{Name: "twcss"}); err == nil {
		t.Error("an empty chain must report that no resolver handled the package")
	}
}

// TestResolvedAccessors verifies the version helpers on a resolved
// package.
func TestResolvedAccessors(t *testing.T) {
	latest, err := ParseSpec("left-pad@latest")
	if err != nil {
		t.Fatal(err)
	}
	r := &Resolved{Spec: latest}
	if r.EffectiveVersion() != "latest" {
		t.Errorf("EffectiveVersion = %q", r.EffectiveVersion())
	}
	if !r.SatisfiesRequired(mustVersion(t, "9.9.9")) {
		t.Error("latest must satisfy any requirement")
	}

	pinned, err := ParseSpec("left-pad@1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	r2 := &Resolved{Spec: pinned}
	if r2.EffectiveVersion() != "1.2.3" {
		t.Errorf("EffectiveVersion = %q", r2.EffectiveVersion())
	}
	if r2.SatisfiesRequired(mustVersion(t, "2.0.0")) {
		t.Error("1.2.3 must not satisfy a 2.0.0 requirement")
	}
}

// containsPrefix reports whether s starts with prefix.
func containsPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
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
		{"krewire scope maps to Go module", "@krewire/ui", "gomod"},
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

func TestDefaultChainResolvesSvelteAndVueToPlugin(t *testing.T) {
	// "svelte" and "vue" resolve to their respective modular plugins
	resolvedSvelte, err := DefaultChain().Resolve(mustSpec(t, "svelte"))
	if err != nil {
		t.Fatalf("DefaultChain().Resolve(svelte) error = %v", err)
	}
	if resolvedSvelte.Kind != "plugin" {
		t.Errorf("Kind = %q, want 'plugin'", resolvedSvelte.Kind)
	}

	resolvedVue, err := DefaultChain().Resolve(mustSpec(t, "vue"))
	if err != nil {
		t.Fatalf("DefaultChain().Resolve(vue) error = %v", err)
	}
	if resolvedVue.Kind != "plugin" {
		t.Errorf("Kind = %q, want 'plugin'", resolvedVue.Kind)
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
