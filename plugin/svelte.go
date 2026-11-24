package plugin

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/krewire/krewire/version"
)

func init() {
	Register(&Svelte{})
}

const (
	svelteConfigFile    = "svelte.config.js"
	svelteNpmPackage    = "svelte"
	sveltePluginPackage = "@sveltejs/vite-plugin-svelte"
	svelteAssetJS       = "assets/svelte.js"
	svelteAssetCSS      = "assets/svelte.css"
)

var svelteConfigFiles = []string{
	svelteConfigFile,
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
	// 1. Check for Svelte config files
	for _, name := range svelteConfigFiles {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return true
		}
	}

	// 2. Check for package.json declaring svelte dependency
	pkgPath := filepath.Join(root, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		content := string(data)
		if strings.Contains(content, `"svelte"`) || strings.Contains(content, `"@sveltejs/vite-plugin-svelte"`) {
			return true
		}
	}

	// 3. Check for .svelte component files in standard source directories
	candidateDirs := []string{"components", "src", "web", "pages"}
	return hasExtInDirs(root, candidateDirs, ".svelte")
}

func (s *Svelte) Build(root, outDir string) error {
	if !s.Detect(root) {
		return nil
	}

	runners := buildViteCandidateCommands(root, outDir)
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
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(svelteAssetJS))); err == nil {
		assets = append(assets, svelteAssetJS)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(svelteAssetCSS))); err == nil {
		assets = append(assets, svelteAssetCSS)
	}
	return assets
}

func (s *Svelte) Add(root, ver string) error {
	if ver == "" || ver == "latest" {
		ver = "latest"
	}
	pkg := svelteNpmPackage + "@" + ver
	slog.Info("svelte: installing", "package", pkg, "root", root)

	if err := ensureSvelteConfig(root); err != nil {
		return err
	}
	if err := ensureViteSvelteConfig(root); err != nil {
		return err
	}
	if err := ensureSvelteStarter(root); err != nil {
		return err
	}

	if err := installPackage(root, pkg, true); err != nil {
		slog.Warn("svelte: package install failed, config files created", "err", err)
		return err
	}
	_ = installPackage(root, sveltePluginPackage, true)
	_ = installPackage(root, "vite", true)

	slog.Info("svelte: successfully installed", "package", pkg)
	return nil
}

func (s *Svelte) Remove(root string) error {
	slog.Info("svelte: removing", "root", root)
	for _, name := range svelteConfigFiles {
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

	_ = uninstallPackage(root, svelteNpmPackage)
	_ = uninstallPackage(root, sveltePluginPackage)
	return nil
}

const svelteConfigTemplate = `import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

export default {
  preprocess: vitePreprocess(),
};
`

const viteSvelteConfigTemplate = `import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: '.krewire/build/assets',
    emptyOutDir: false,
    rollupOptions: {
      output: {
        entryFileNames: 'svelte.js',
        assetFileNames: 'svelte.[ext]',
      },
    },
  },
});
`

const svelteStarterTemplate = `<script>
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

func ensureSvelteConfig(root string) error {
	for _, name := range svelteConfigFiles {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return nil
		}
	}
	p := filepath.Join(root, svelteConfigFile)
	return os.WriteFile(p, []byte(svelteConfigTemplate), projectFilePerm)
}

func ensureViteSvelteConfig(root string) error {
	viteConfigs := []string{"vite.config.js", "vite.config.ts", "vite.config.mjs"}
	for _, name := range viteConfigs {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return nil
		}
	}
	p := filepath.Join(root, "vite.config.js")
	return os.WriteFile(p, []byte(viteSvelteConfigTemplate), projectFilePerm)
}

func ensureSvelteStarter(root string) error {
	compDir := filepath.Join(root, "components")
	if err := os.MkdirAll(compDir, projectDirPerm); err != nil {
		return err
	}
	p := filepath.Join(compDir, "Counter.svelte")
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	return os.WriteFile(p, []byte(svelteStarterTemplate), projectFilePerm)
}

func hasExtInDirs(root string, dirs []string, ext string) bool {
	for _, d := range dirs {
		dirPath := filepath.Join(root, d)
		info, err := os.Stat(dirPath)
		if err != nil || !info.IsDir() {
			continue
		}
		found := false
		_ = filepath.WalkDir(dirPath, func(path string, de os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if de.IsDir() {
				if de.Name() == "node_modules" || de.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(strings.ToLower(de.Name()), ext) {
				found = true
				return filepath.SkipAll
			}
			return nil
		})
		if found {
			return true
		}
	}
	return false
}

func buildViteCandidateCommands(root, outDir string) [][]string {
	var candidates [][]string
	targetOut := filepath.Join(outDir, "assets")

	// 1. Local node_modules
	candidates = append(candidates,
		[]string{"./node_modules/.bin/vite", "build", "--outDir", targetOut},
	)

	// 2. Standalone binary in project or PATH
	if _, err := os.Stat(filepath.Join(root, "bin/vite")); err == nil {
		candidates = append(candidates, []string{"./bin/vite", "build", "--outDir", targetOut})
	}
	if _, err := exec.LookPath("vite"); err == nil {
		candidates = append(candidates, []string{"vite", "build", "--outDir", targetOut})
	}

	// 3. Bun runners
	if _, err := exec.LookPath("bun"); err == nil {
		candidates = append(candidates, []string{"bunx", "vite", "build", "--outDir", targetOut})
	}

	// 4. PNPM / Yarn runners
	if _, err := exec.LookPath("pnpm"); err == nil {
		candidates = append(candidates, []string{"pnpm", "exec", "vite", "build", "--outDir", targetOut})
	}
	if _, err := exec.LookPath("yarn"); err == nil {
		candidates = append(candidates, []string{"yarn", "vite", "build", "--outDir", targetOut})
	}

	// 5. NPX fallback
	candidates = append(candidates, []string{"npx", "vite", "build", "--outDir", targetOut})

	return candidates
}
