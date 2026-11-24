package svelte

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/krewire/hub/plugin"
	"github.com/krewire/krewire/version"
)

func init() {
	plugin.Register(&Svelte{})
}

const (
	ConfigFile    = "svelte.config.js"
	NpmPackage    = "svelte"
	PluginPackage = "@sveltejs/vite-plugin-svelte"
	AssetJS       = "assets/svelte.js"
	AssetCSS      = "assets/svelte.css"
)

var ConfigFiles = []string{
	ConfigFile,
	"svelte.config.mjs",
	"svelte.config.ts",
	"svelte.config.cjs",
}

// Svelte implements Plugin, Installer, and AssetProvider for Svelte components.
type Svelte struct{}

func (s *Svelte) Name() string { return "svelte" }

func (s *Svelte) Aliases() []string { return []string{"sveltejs"} }

func (s *Svelte) Version() version.Version {
	return version.MustParseVersion("5.0.0")
}

func (s *Svelte) Detect(root string) bool {
	for _, name := range ConfigFiles {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return true
		}
	}

	pkgPath := filepath.Join(root, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		content := string(data)
		if strings.Contains(content, `"svelte"`) || strings.Contains(content, `"@sveltejs/vite-plugin-svelte"`) {
			return true
		}
	}

	candidateDirs := []string{"components", "src", "web", "pages"}
	return plugin.HasExtInDirs(root, candidateDirs, ".svelte")
}

func (s *Svelte) Build(root, outDir string) error {
	if !s.Detect(root) {
		return nil
	}

	runners := plugin.BuildViteCandidateCommands(root, outDir)
	var lastErr error
	for _, args := range runners {
		bin := args[0]
		if _, err := exec.LookPath(bin); err != nil && !filepath.IsAbs(bin) && !strings.Contains(bin, "/") {
			continue
		}
		if strings.Contains(bin, "/") {
			if _, err := os.Stat(filepath.Join(root, bin)); err != nil {
				continue
			}
		}

		slog.Info("svelte: building assets with vite", "cmd", args)
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			lastErr = fmt.Errorf("svelte: %v: %s", err, string(out))
			slog.Warn("svelte: build attempt failed", "runner", bin, "err", lastErr)
			continue
		}
		slog.Info("svelte: build completed")
		return nil
	}

	if lastErr != nil {
		slog.Warn("svelte: build skipped or runner failed", "err", lastErr)
	}
	return nil
}

func (s *Svelte) Assets(root string) []string {
	var assets []string
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(AssetJS))); err == nil {
		assets = append(assets, AssetJS)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(AssetCSS))); err == nil {
		assets = append(assets, AssetCSS)
	}
	return assets
}

func (s *Svelte) Add(root, ver string) error {
	if ver == "" || ver == "latest" {
		ver = "latest"
	}
	pkg := NpmPackage + "@" + ver
	slog.Info("svelte: installing", "package", pkg, "root", root)

	if err := EnsureConfig(root); err != nil {
		return err
	}
	if err := EnsureViteConfig(root); err != nil {
		return err
	}
	if err := EnsureStarter(root); err != nil {
		return err
	}

	if err := plugin.InstallPackage(root, pkg, true); err != nil {
		slog.Warn("svelte: package install failed, config files created", "err", err)
		return err
	}
	_ = plugin.InstallPackage(root, PluginPackage, true)
	_ = plugin.InstallPackage(root, "vite", true)

	slog.Info("svelte: successfully installed", "package", pkg)
	return nil
}

func (s *Svelte) Remove(root string) error {
	slog.Info("svelte: removing", "root", root)
	for _, name := range ConfigFiles {
		p := filepath.Join(root, name)
		if _, err := os.Stat(p); err == nil {
			_ = os.Remove(p)
			slog.Info("svelte: removed config", "file", name)
		}
	}

	starter := filepath.Join(root, "components", "Counter.svelte")
	if data, err := os.ReadFile(starter); err == nil {
		if strings.Contains(string(data), "Svelte 5 Reactive Island") {
			_ = os.Remove(starter)
			slog.Info("svelte: removed starter component", "file", starter)
		}
	}

	_ = plugin.UninstallPackage(root, NpmPackage)
	_ = plugin.UninstallPackage(root, PluginPackage)
	return nil
}

const ConfigTemplate = `import { vitePreprocess } from @sveltejs/vite-plugin-svelte;

export default {
  preprocess: vitePreprocess(),
};
`

const ViteConfigTemplate = `import { defineConfig } from vite;
import { svelte } from @sveltejs/vite-plugin-svelte;

export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: .krewire/build/assets,
    emptyOutDir: false,
    rollupOptions: {
      output: {
        entryFileNames: svelte.js,
        assetFileNames: svelte.[ext],
      },
    },
  },
});
`

const StarterTemplate = `<script>
  let count = $state(0);
</script>

<div class="krewire-svelte-card p-4 border-2 border-black rounded-lg shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] bg-emerald-50">
  <h3 class="font-bold text-lg text-emerald-950 mb-2">Svelte 5 Reactive Island</h3>
  <p class="text-sm text-gray-700 mb-4">Client-side interactive component mounted into Krewire SSR page.</p>
  <button 
    class="px-4 py-2 font-bold text-white bg-emerald-600 hover:bg-emerald-700 rounded transition border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:translate-x-0.5 active:translate-y-0.5"
    onclick={() => count++}
  >
    Count is {count}
  </button>
</div>
`

func EnsureConfig(root string) error {
	for _, name := range ConfigFiles {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return nil
		}
	}
	p := filepath.Join(root, ConfigFile)
	return os.WriteFile(p, []byte(ConfigTemplate), plugin.ProjectFilePerm)
}

func EnsureViteConfig(root string) error {
	viteConfigs := []string{"vite.config.js", "vite.config.ts", "vite.config.mjs"}
	for _, name := range viteConfigs {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return nil
		}
	}
	p := filepath.Join(root, "vite.config.js")
	return os.WriteFile(p, []byte(ViteConfigTemplate), plugin.ProjectFilePerm)
}

func EnsureStarter(root string) error {
	compDir := filepath.Join(root, "components")
	if err := os.MkdirAll(compDir, plugin.ProjectDirPerm); err != nil {
		return err
	}
	p := filepath.Join(compDir, "Counter.svelte")
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	return os.WriteFile(p, []byte(StarterTemplate), plugin.ProjectFilePerm)
}
