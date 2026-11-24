package tailwind_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/krewire/hub/plugin"
	"github.com/krewire/hub/tailwind"
)

func TestTailwindMetadata(t *testing.T) {
	tw := &tailwind.Tailwind{}
	if tw.Name() != "tailwind" {
		t.Errorf("Name = %q", tw.Name())
	}
	if got := tw.Aliases(); len(got) != 2 || got[0] != "twcss" {
		t.Errorf("Aliases = %v", got)
	}
	if tw.Version().Major != 3 {
		t.Errorf("Version = %+v", tw.Version())
	}
	var _ plugin.Plugin = tw
	var _ plugin.Installer = tw
	var _ plugin.AssetProvider = tw
}

func TestTailwindDetect(t *testing.T) {
	tw := &tailwind.Tailwind{}
	if tw.Detect(t.TempDir()) {
		t.Error("Detect must be false without a config file")
	}
	for _, name := range tailwind.ConfigFiles {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("module.exports={}"), 0o600); err != nil {
			t.Fatal(err)
		}
		if !tw.Detect(dir) {
			t.Errorf("Detect must be true when %s exists", name)
		}
	}
}

func TestTailwindAssets(t *testing.T) {
	tw := &tailwind.Tailwind{}
	root := t.TempDir()
	if got := tw.Assets(root); got != nil {
		t.Errorf("Assets = %v, want nil without the input file", got)
	}
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "tailwind.css"), []byte("@tailwind base;"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := tw.Assets(root)
	if len(got) != 1 || got[0] != "assets/tailwind.css" {
		t.Errorf("Assets = %v", got)
	}
}

func TestTailwindBuildSkipsWithoutInput(t *testing.T) {
	tw := &tailwind.Tailwind{}
	if err := tw.Build(t.TempDir(), t.TempDir()); err != nil {
		t.Errorf("Build without input must succeed as a no-op, got %v", err)
	}
}

func TestRegistrySelfRegistration(t *testing.T) {
	if plugin.Find("tailwind") == nil {
		t.Error("the Tailwind plugin must self-register via init()")
	}
	if plugin.FindInstaller("twcss") == nil {
		t.Error("Tailwind must be reachable as an Installer through its alias")
	}
}
