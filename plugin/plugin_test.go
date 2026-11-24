package plugin

import (
	"testing"

	"github.com/krewire/krewire/version"
)

// isolateRegistry swaps the global registry for the duration of a test so
// Register calls in one test cannot leak into another.
func isolateRegistry(t *testing.T) {
	t.Helper()
	saved := Registry
	Registry = nil
	t.Cleanup(func() { Registry = saved })
}

// stubPlugin is a minimal Plugin used to exercise the lookup helpers.
type stubPlugin struct {
	name    string
	aliases []string
	version version.Version
}

func (s *stubPlugin) Name() string               { return s.name }
func (s *stubPlugin) Aliases() []string          { return s.aliases }
func (s *stubPlugin) Detect(string) bool         { return true }
func (s *stubPlugin) Build(string, string) error { return nil }
func (s *stubPlugin) Version() version.Version   { return s.version }

// stubInstaller additionally satisfies the Installer contract.
type stubInstaller struct{ stubPlugin }

func (s *stubInstaller) Add(string, string) error { return nil }
func (s *stubInstaller) Remove(string) error      { return nil }

// TestFindMatchesNameAndAliases verifies lookup is case-insensitive,
// trims surrounding space, and matches both the plugin name and its aliases.
func TestFindMatchesNameAndAliases(t *testing.T) {
	isolateRegistry(t)
	Register(&stubPlugin{name: "tailwind", aliases: []string{"twcss"}, version: version.MustParseVersion("1.2.3")})

	for _, query := range []string{
		"tailwind", "TAILWIND", "  tailwind  ", "twcss", "TWCSS",
	} {
		if Find(query) == nil {
			t.Errorf("Find(%q) = nil, want the stub plugin", query)
		}
	}
	if Find("nope") != nil {
		t.Error("Find must return nil for an unknown name")
	}
}

// TestFindMapsTailwindcss verifies the npm package name resolves to
// the Tailwind plugin even when it is not declared as an alias.
func TestFindMapsTailwindcss(t *testing.T) {
	isolateRegistry(t)
	Register(&stubPlugin{name: "tailwind", aliases: []string{"twcss"}, version: version.MustParseVersion("1.2.3")})
	if Find("tailwindcss") == nil {
		t.Error("the npm name tailwindcss must resolve to the tailwind plugin")
	}
}

// TestFindInstaller verifies the Installer narrowing only succeeds
// for plugins that implement the optional contract.
func TestFindInstaller(t *testing.T) {
	isolateRegistry(t)

	Register(&stubPlugin{name: "plain", version: version.MustParseVersion("1.0.0")})
	if FindInstaller("plain") != nil {
		t.Error("a plugin without Add/Remove must not satisfy Installer")
	}

	Register(&stubInstaller{stubPlugin{name: "managed", version: version.MustParseVersion("1.0.0")}})
	if FindInstaller("managed") == nil {
		t.Error("a plugin implementing Installer must be returned")
	}
	if FindInstaller("absent") != nil {
		t.Error("FindInstaller must return nil for an unknown name")
	}
}

// TestFindByVersion verifies the version filter rejects a plugin that
// does not satisfy the requirement.
func TestFindByVersion(t *testing.T) {
	isolateRegistry(t)
	Register(&stubPlugin{name: "tw", version: version.MustParseVersion("1.2.0")})

	if FindByVersion("tw", version.MustParseVersion("1.0.0")) == nil {
		t.Error("a compatible version must be returned")
	}
	if FindByVersion("tw", version.MustParseVersion("9.0.0")) != nil {
		t.Error("an incompatible version must be filtered out")
	}
	if FindByVersion("absent", version.MustParseVersion("1.0.0")) != nil {
		t.Error("an unknown name must return nil")
	}
}

// TestFindInstallerByVersion verifies both the name and version
// filters apply to the Installer lookup.
func TestFindInstallerByVersion(t *testing.T) {
	isolateRegistry(t)
	Register(&stubInstaller{stubPlugin{name: "tw", version: version.MustParseVersion("1.2.0")}})

	if FindInstallerByVersion("tw", version.MustParseVersion("1.0.0")) == nil {
		t.Error("a compatible installer must be returned")
	}
	if FindInstallerByVersion("tw", version.MustParseVersion("9.0.0")) != nil {
		t.Error("an incompatible installer must be filtered out")
	}
	if FindInstallerByVersion("absent", version.MustParseVersion("1.0.0")) != nil {
		t.Error("an unknown name must return nil")
	}
}
