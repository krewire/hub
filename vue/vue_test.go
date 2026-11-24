package vue_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/krewire/hub/plugin"
	"github.com/krewire/hub/vue"
)

func TestVuePlugin_FindAndAliases(t *testing.T) {
	p := plugin.Find("vue")
	if p == nil {
		t.Fatal("Find('vue') = nil, expected Vue plugin")
	}
	if p.Name() != "vue" {
		t.Errorf("expected name 'vue', got %q", p.Name())
	}
	if plugin.Find("VUE") == nil {
		t.Error("Find('VUE') must match case-insensitively")
	}
	if plugin.Find("vuejs") == nil {
		t.Error("Find('vuejs') must match alias")
	}
	if plugin.Find("vue3") == nil {
		t.Error("Find('vue3') must match alias")
	}
	if plugin.FindInstaller("vue") == nil {
		t.Error("FindInstaller('vue') must return Installer")
	}
}

func TestVuePlugin_Detect(t *testing.T) {
	dir := t.TempDir()
	p := &vue.Vue{}

	if p.Detect(dir) {
		t.Error("Detect must return false for empty directory")
	}

	confPath := filepath.Join(dir, "vue.config.js")
	if err := os.WriteFile(confPath, []byte("module.exports = {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !p.Detect(dir) {
		t.Error("Detect must return true when vue.config.js exists")
	}
	_ = os.Remove(confPath)

	pkgPath := filepath.Join(dir, "package.json")
	if err := os.WriteFile(pkgPath, []byte(`{"dependencies": {"vue": "^3.5.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !p.Detect(dir) {
		t.Error("Detect must return true when package.json contains vue")
	}
	_ = os.Remove(pkgPath)

	compDir := filepath.Join(dir, "components")
	_ = os.MkdirAll(compDir, 0o755)
	compFile := filepath.Join(compDir, "Modal.vue")
	_ = os.WriteFile(compFile, []byte("<template><div>Modal</div></template>"), 0o644)
	if !p.Detect(dir) {
		t.Error("Detect must return true when .vue file exists in components/")
	}
}

func TestVuePlugin_Assets(t *testing.T) {
	dir := t.TempDir()
	p := &vue.Vue{}

	if len(p.Assets(dir)) != 0 {
		t.Error("Assets must return empty slice when no build artifacts exist")
	}

	assetsDir := filepath.Join(dir, "assets")
	_ = os.MkdirAll(assetsDir, 0o755)
	_ = os.WriteFile(filepath.Join(assetsDir, "vue.js"), []byte("console.log(1)"), 0o644)

	assets := p.Assets(dir)
	if len(assets) != 1 || assets[0] != "assets/vue.js" {
		t.Errorf("expected ['assets/vue.js'], got %v", assets)
	}
}
