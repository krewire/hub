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
	Register(&Vue{})
}

const (
	vueNpmPackage    = "vue"
	vuePluginPackage = "@vitejs/plugin-vue"
	vueAssetJS       = "assets/vue.js"
	vueAssetCSS      = "assets/vue.css"
)

var vueConfigFiles = []string{
	"vue.config.js",
	"vue.config.ts",
	"vue.config.mjs",
	"vue.config.cjs",
}

// Vue implements Plugin, Installer, and AssetProvider for Vue components.
type Vue struct{}

func (v *Vue) Name() string { return "vue" }

func (v *Vue) Aliases() []string { return []string{"vuejs", "vue3"} }

func (v *Vue) Version() version.Version {
	return version.MustParseVersion("3.5.0")
}

func (v *Vue) Detect(root string) bool {
	// 1. Check for Vue-specific config files
	for _, name := range vueConfigFiles {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return true
		}
	}

	// 2. Check for package.json declaring vue dependency
	pkgPath := filepath.Join(root, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		content := string(data)
		if strings.Contains(content, `"vue"`) || strings.Contains(content, `"@vitejs/plugin-vue"`) {
			return true
		}
	}

	// 3. Check for vite config declaring vue plugin
	viteConfigs := []string{"vite.config.js", "vite.config.ts", "vite.config.mjs"}
	for _, name := range viteConfigs {
		if data, err := os.ReadFile(filepath.Join(root, name)); err == nil {
			content := string(data)
			if strings.Contains(content, "plugin-vue") || strings.Contains(content, "vue()") {
				return true
			}
		}
	}

	// 4. Check for .vue component files in standard source directories
	candidateDirs := []string{"components", "src", "web", "pages"}
	return hasExtInDirs(root, candidateDirs, ".vue")
}

func (v *Vue) Build(root, outDir string) error {
	if !v.Detect(root) {
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

		slog.Info("vue: building assets with vite", "cmd", args)
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			lastErr = fmt.Errorf("vue: %v: %s", err, string(out))
			slog.Warn("vue: build attempt failed", "runner", bin, "err", lastErr)
			continue
		}
		slog.Info("vue: build completed")
		return nil
	}

	if lastErr != nil {
		slog.Warn("vue: build skipped or runner failed", "err", lastErr)
	}
	return nil
}

func (v *Vue) Assets(root string) []string {
	var assets []string
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(vueAssetJS))); err == nil {
		assets = append(assets, vueAssetJS)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(vueAssetCSS))); err == nil {
		assets = append(assets, vueAssetCSS)
	}
	return assets
}

func (v *Vue) Add(root, ver string) error {
	if ver == "" || ver == "latest" {
		ver = "latest"
	}
	pkg := vueNpmPackage + "@" + ver
	slog.Info("vue: installing", "package", pkg, "root", root)

	if err := ensureViteVueConfig(root); err != nil {
		return err
	}
	if err := ensureVueStarter(root); err != nil {
		return err
	}

	if err := installPackage(root, pkg, true); err != nil {
		slog.Warn("vue: package install failed, config files created", "err", err)
		return err
	}
	_ = installPackage(root, vuePluginPackage, true)
	_ = installPackage(root, "vite", true)

	slog.Info("vue: successfully installed", "package", pkg)
	return nil
}

func (v *Vue) Remove(root string) error {
	slog.Info("vue: removing", "root", root)
	starter := filepath.Join(root, "components", "Counter.vue")
	if data, err := os.ReadFile(starter); err == nil {
		if strings.Contains(string(data), "Vue 3 Composition Island") {
			_ = os.Remove(starter)
			slog.Info("vue: removed starter component", "file", starter)
		}
	}

	_ = uninstallPackage(root, vueNpmPackage)
	_ = uninstallPackage(root, vuePluginPackage)
	return nil
}

const viteVueConfigTemplate = `import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: '.krewire/build/assets',
    emptyOutDir: false,
    rollupOptions: {
      output: {
        entryFileNames: 'vue.js',
        assetFileNames: 'vue.[ext]',
      },
    },
  },
});
`

const vueStarterTemplate = `<script setup>
import { ref } from 'vue';

const props = defineProps({
  initial: {
    type: Number,
    default: 0
  }
});

const count = ref(props.initial);
</script>

<template>
  <div class="krewire-vue-card p-4 border-2 border-black rounded-lg shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] bg-teal-50">
    <h3 class="font-bold text-lg text-teal-950 mb-2">Vue 3 Composition Island</h3>
    <p class="text-sm text-gray-700 mb-4">Client-side interactive component mounted into Krewire SSR page.</p>
    <button 
      class="px-4 py-2 font-bold text-white bg-teal-600 hover:bg-teal-700 rounded transition border-2 border-black shadow-[2px_2px_0px_0px_rgba(0,0,0,1)] active:translate-x-0.5 active:translate-y-0.5"
      @click="count++"
    >
      Count is {{ count }}
    </button>
  </div>
</template>
`

func ensureViteVueConfig(root string) error {
	viteConfigs := []string{"vite.config.js", "vite.config.ts", "vite.config.mjs"}
	for _, name := range viteConfigs {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return nil
		}
	}
	p := filepath.Join(root, "vite.config.js")
	return os.WriteFile(p, []byte(viteVueConfigTemplate), projectFilePerm)
}

func ensureVueStarter(root string) error {
	compDir := filepath.Join(root, "components")
	if err := os.MkdirAll(compDir, projectDirPerm); err != nil {
		return err
	}
	p := filepath.Join(compDir, "Counter.vue")
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	return os.WriteFile(p, []byte(vueStarterTemplate), projectFilePerm)
}
