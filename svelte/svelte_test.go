package svelte_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/krewire/hub/plugin"
	"github.com/krewire/hub/svelte"
)

func TestSveltePlugin_FindAndAliases(t *testing.T) {
	p := plugin.Find("svelte")
	if p == nil {
		t.Fatal("Find('svelte') = nil, expected Svelte plugin")
	}
	if p.Name() != "svelte" {
		t.Errorf("expected name 'svelte', got %q", p.Name())
	}
	if plugin.Find("SVELTE") == nil {
		t.Error("Find('SVELTE') must match case-insensitively")
	}
	if plugin.Find("sveltejs") == nil {
		t.Error("Find('sveltejs') must match alias")
	}
	if plugin.FindInstaller("svelte") == nil {
		t.Error("FindInstaller('svelte') must return Installer")
	}
}

func TestSveltePlugin_Detect(t *testing.T) {
	dir := t.TempDir()
	p := &svelte.Svelte{}

	if p.Detect(dir) {
		t.Error("Detect must return false for empty directory")
	}

	confPath := filepath.Join(dir, "svelte.config.js")
	if err := os.WriteFile(confPath, []byte("export default {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !p.Detect(dir) {
		t.Error("Detect must return true when svelte.config.js exists")
	}
	_ = os.Remove(confPath)

	pkgPath := filepath.Join(dir, "package.json")
	if err := os.WriteFile(pkgPath, []byte(`{"dependencies": {"svelte": "^5.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !p.Detect(dir) {
		t.Error("Detect must return true when package.json contains svelte")
	}
	_ = os.Remove(pkgPath)

	compDir := filepath.Join(dir, "components")
	_ = os.MkdirAll(compDir, 0o755)
	compFile := filepath.Join(compDir, "Button.svelte")
	_ = os.WriteFile(compFile, []byte("<button>Click</button>"), 0o644)
	if !p.Detect(dir) {
		t.Error("Detect must return true when .svelte file exists in components/")
	}
}

func TestSveltePlugin_EnsureConfigsAndStarters(t *testing.T) {
	dir := t.TempDir()

	if err := svelte.EnsureConfig(dir); err != nil {
		t.Fatalf("EnsureConfig failed: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "svelte.config.js"))
	if err != nil || !strings.Contains(string(content), "vitePreprocess") {
		t.Errorf("unexpected svelte.config.js content: %s", string(content))
	}

	if err := svelte.EnsureViteConfig(dir); err != nil {
		t.Fatalf("EnsureViteConfig failed: %v", err)
	}
	viteContent, err := os.ReadFile(filepath.Join(dir, "vite.config.js"))
	if err != nil || !strings.Contains(string(viteContent), "plugin-svelte") {
		t.Errorf("unexpected vite.config.js content: %s", string(viteContent))
	}

	if err := svelte.EnsureStarter(dir); err != nil {
		t.Fatalf("EnsureStarter failed: %v", err)
	}
	starter, err := os.ReadFile(filepath.Join(dir, "components", "Counter.svelte"))
	if err != nil || !strings.Contains(string(starter), "Svelte 5 Reactive Island") {
		t.Errorf("unexpected Counter.svelte content: %s", string(starter))
	}
}

func TestSveltePlugin_Assets(t *testing.T) {
	dir := t.TempDir()
	p := &svelte.Svelte{}

	if len(p.Assets(dir)) != 0 {
		t.Error("Assets must return empty slice when no build artifacts exist")
	}

	assetsDir := filepath.Join(dir, "assets")
	_ = os.MkdirAll(assetsDir, 0o755)
	_ = os.WriteFile(filepath.Join(assetsDir, "svelte.js"), []byte("console.log(1)"), 0o644)

	assets := p.Assets(dir)
	if len(assets) != 1 || assets[0] != "assets/svelte.js" {
		t.Errorf("expected ['assets/svelte.js'], got %v", assets)
	}
}
