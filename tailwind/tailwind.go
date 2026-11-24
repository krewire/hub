package tailwind

import (
	"github.com/krewire/hub/plugin"
)

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
	plugin.Register(&Tailwind{})
}

// Tailwind implements Plugin for Tailwind CSS via the Tailwind CLI.
// Detection: presence of tailwind.config.js at the project root.
// Build: runs `npx tailwindcss -i assets/tailwind.css -o <outDir>/assets/tailwind.css --minify`
// (or ./node_modules/.bin/tailwindcss if npx is not available). Failures are
// logged as warnings and do not fail the overall site build — Tailwind is
// an optional plugin, not a core requirement.
// Tailwind plugin constants. The config filenames are checked in multiple places
// (Detect, Remove, ensureTailwindConfig), so they live here as one list rather
// than being repeated inline.
const (
	// tailwindConfigFile is the config file created by Add when none exists.
	tailwindConfigFile = "tailwind.config.js"
	// tailwindNpmPackage is the npm package installed by Add and removed by Remove.
	tailwindNpmPackage = "tailwindcss"
	// tailwindAssetPath is the default logical asset path Tailwind emits.
	tailwindAssetPath = "assets/tailwind.css"
	// projectDirPerm is the mode for directories Add and Build create.
	projectDirPerm = 0o755
	// plugin.ProjectFilePerm is the mode for the config and input files Add creates.
)

// tailwindConfigFiles lists every config filename Detect recognises, in the order
// Add prefers to keep an existing one.
var ConfigFiles = []string{
	tailwindConfigFile,
	"tailwind.config.cjs",
	"tailwind.config.mjs",
	"tailwind.config.ts",
	"tailwind.config.json",
}

type Tailwind struct{}

func (t *Tailwind) Name() string { return "tailwind" }

func (t *Tailwind) Aliases() []string { return []string{"twcss", "tailwindcss"} }

func (t *Tailwind) Version() version.Version {
	return version.MustParseVersion("3.4.0") // Tailwind CSS baseline
}

func (t *Tailwind) Detect(root string) bool {
	// 1. Check for standard Tailwind config files (Tailwind v3 & v4 legacy)
	for _, name := range ConfigFiles {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return true
		}
	}

	// 2. Check for package.json declaring tailwindcss dependency
	pkgPath := filepath.Join(root, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		s := string(data)
		if strings.Contains(s, `"tailwindcss"`) || strings.Contains(s, `"@tailwindcss/cli"`) {
			return true
		}
	}

	// 3. Check for Tailwind CSS directives in common stylesheet locations (Tailwind v3 & v4)
	candidateCSS := []string{
		tailwindAssetPath,
		"assets/style.css",
		"assets/app.css",
		"styles/tailwind.css",
	}
	for _, rel := range candidateCSS {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if data, err := os.ReadFile(p); err == nil {
			content := string(data)
			if strings.Contains(content, "@tailwind") ||
				strings.Contains(content, `@import "tailwindcss"`) ||
				strings.Contains(content, `@import 'tailwindcss'`) ||
				strings.Contains(content, "@theme") {
				return true
			}
		}
	}

	return false
}

// resolveInputPath determines the input stylesheet to process.
func (t *Tailwind) resolveInputPath(root string) string {
	defaultInput := filepath.Join(root, filepath.FromSlash(tailwindAssetPath))
	if _, err := os.Stat(defaultInput); err == nil {
		return tailwindAssetPath
	}

	// Check other stylesheet locations for tailwind directives
	candidates := []string{
		"assets/style.css",
		"assets/app.css",
		"styles/tailwind.css",
	}
	for _, rel := range candidates {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if data, err := os.ReadFile(p); err == nil {
			content := string(data)
			if strings.Contains(content, "@tailwind") ||
				strings.Contains(content, `@import "tailwindcss"`) ||
				strings.Contains(content, `@import 'tailwindcss'`) {
				return rel
			}
		}
	}

	return tailwindAssetPath
}

func (t *Tailwind) Build(root, outDir string) error {
	relInput := t.resolveInputPath(root)
	input := filepath.Join(root, filepath.FromSlash(relInput))
	if _, err := os.Stat(input); err != nil {
		slog.Warn("tailwind: "+relInput+" not found, skipping", "root", root)
		return nil
	}
	output := filepath.Join(outDir, filepath.FromSlash(relInput))
	if err := os.MkdirAll(filepath.Dir(output), projectDirPerm); err != nil {
		return fmt.Errorf("tailwind: mkdir: %w", err)
	}

	// Discover candidate CLI runners in priority order:
	// 1. Local project node_modules binaries (v3 & v4)
	// 2. Standalone binary in PATH or project bin (official standalone CLI for Go setups without Node)
	// 3. Alternative package managers (bunx, pnpm, yarn)
	// 4. Fallback npx
	candidates := buildCandidateCommands(root, input, output)

	var lastErr error
	for _, args := range candidates {
		bin := args[0]
		if _, err := exec.LookPath(bin); err != nil && !filepath.IsAbs(bin) && !strings.Contains(bin, "/") {
			continue
		}
		if strings.Contains(bin, "/") {
			if _, err := os.Stat(filepath.Join(root, bin)); err != nil {
				continue
			}
		}

		slog.Info("tailwind: building", "input", input, "output", output, "cmd", args)
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			lastErr = fmt.Errorf("tailwind: %v: %s", err, string(out))
			slog.Warn("tailwind: build attempt failed", "runner", bin, "err", lastErr)
			continue
		}
		slog.Info("tailwind: built", "output", output)
		return nil
	}
	if lastErr != nil {
		slog.Warn("tailwind: CLI not available or build failed, skipping", "err", lastErr, "hint", "install tailwindcss standalone CLI or npm install -D tailwindcss")
	}
	return nil
}

func buildCandidateCommands(root, input, output string) [][]string {
	var candidates [][]string

	// 1. Local node_modules
	candidates = append(candidates,
		[]string{"./node_modules/.bin/" + tailwindNpmPackage, "-i", input, "-o", output, "--minify"},
		[]string{"./node_modules/.bin/@tailwindcss/cli", "-i", input, "-o", output, "--minify"},
	)

	// 2. Standalone binary in project or PATH
	if _, err := os.Stat(filepath.Join(root, "bin/tailwindcss")); err == nil {
		candidates = append(candidates, []string{"./bin/tailwindcss", "-i", input, "-o", output, "--minify"})
	}
	if _, err := os.Stat(filepath.Join(root, "tailwindcss")); err == nil {
		candidates = append(candidates, []string{"./tailwindcss", "-i", input, "-o", output, "--minify"})
	}
	if _, err := exec.LookPath("tailwindcss"); err == nil {
		candidates = append(candidates, []string{"tailwindcss", "-i", input, "-o", output, "--minify"})
	}

	// 3. Bun runners
	if _, err := exec.LookPath("bun"); err == nil {
		candidates = append(candidates,
			[]string{"bunx", "tailwindcss", "-i", input, "-o", output, "--minify"},
			[]string{"bunx", "@tailwindcss/cli", "-i", input, "-o", output, "--minify"},
		)
	}

	// 4. PNPM / Yarn runners
	if _, err := exec.LookPath("pnpm"); err == nil {
		candidates = append(candidates, []string{"pnpm", "exec", "tailwindcss", "-i", input, "-o", output, "--minify"})
		candidates = append(candidates, []string{"pnpm", "dlx", "tailwindcss", "-i", input, "-o", output, "--minify"})
	}
	if _, err := exec.LookPath("yarn"); err == nil {
		candidates = append(candidates, []string{"yarn", "tailwindcss", "-i", input, "-o", output, "--minify"})
	}

	// 5. NPX fallback
	candidates = append(candidates,
		[]string{"npx", tailwindNpmPackage, "-i", input, "-o", output, "--minify"},
		[]string{"npx", "@tailwindcss/cli", "-i", input, "-o", output, "--minify"},
	)

	return candidates
}

// Assets implements plugin.AssetProvider: returns logical asset paths Tailwind emits.
func (t *Tailwind) Assets(root string) []string {
	relInput := t.resolveInputPath(root)
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relInput))); err != nil {
		return nil
	}
	return []string{relInput}
}

const tailwindConfigTemplate = `/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "pages/**/*.{kiw,html,md}",
    "components/**/*.{kiw,html}",
    "layouts/**/*.{kiw,html}",
    "content/**/*.{md,markdown,html}",
    ".krewire/build/**/*.html",
    "**/*.go",
  ],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        primary: "var(--color-primary, var(--ui-primary, #39D353))",
        secondary: "var(--color-secondary, var(--ui-secondary, #00D1C1))",
        accent: "var(--color-accent, var(--ui-accent, #FF3B2E))",
        surface: "var(--color-surface, var(--ui-surface, #ffffff))",
        muted: "var(--color-muted, var(--ui-muted, #57677d))",
      },
    },
  },
  plugins: [],
}
`

const tailwindInputTemplate = `@tailwind base;
@tailwind components;
@tailwind utilities;
`

// Add installs tailwind at version ("" or "latest" means latest) using detected package manager.
func (t *Tailwind) Add(root, version string) error {
	if version == "" || version == "latest" {
		version = "latest"
	}
	pkg := tailwindNpmPackage + "@" + version
	if version == "latest" {
		pkg = tailwindNpmPackage + "@latest"
	}
	slog.Info("tailwind: installing", "package", pkg, "root", root)
	if err := ensureTailwindConfig(root); err != nil {
		return err
	}
	if err := ensureTailwindInput(root); err != nil {
		return err
	}
	if err := plugin.InstallPackage(root, pkg, true); err != nil {
		slog.Warn("tailwind: package install failed, config files still created", "err", err, "hint", "run npm/bun/pnpm install -D "+pkg+" manually")
		return err
	}
	slog.Info("tailwind: installed", "package", pkg)
	return nil
}

// Remove uninstalls tailwind.
func (t *Tailwind) Remove(root string) error {
	slog.Info("tailwind: removing", "root", root)
	removed := 0
	for _, name := range ConfigFiles {
		p := filepath.Join(root, name)
		if _, err := os.Stat(p); err == nil {
			if err := os.Remove(p); err != nil {
				return fmt.Errorf("tailwind: remove %s: %w", name, err)
			}
			removed++
			slog.Info("tailwind: removed", "file", name)
		}
	}
	// do not delete assets/tailwind.css if user customized — only if it matches template
	input := filepath.Join(root, filepath.FromSlash(tailwindAssetPath))
	if data, err := os.ReadFile(input); err == nil {
		if string(data) == tailwindInputTemplate {
			_ = os.Remove(input)
			slog.Info("tailwind: removed", "file", tailwindAssetPath)
		} else {
			slog.Info("tailwind: kept customized " + tailwindAssetPath)
		}
	}
	if err := plugin.UninstallPackage(root, tailwindNpmPackage); err != nil {
		slog.Warn("tailwind: package uninstall failed", "err", err)
		return err
	}
	if removed == 0 {
		slog.Info("tailwind: already not installed")
	}
	return nil
}

func ensureTailwindConfig(root string) error {
	for _, name := range ConfigFiles {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return nil
		}
	}
	path := filepath.Join(root, tailwindConfigFile)
	if err := os.WriteFile(path, []byte(tailwindConfigTemplate), plugin.ProjectFilePerm); err != nil {
		return fmt.Errorf("tailwind: write config: %w", err)
	}
	slog.Info("tailwind: created", "file", tailwindConfigFile)
	return nil
}

func ensureTailwindInput(root string) error {
	path := filepath.Join(root, filepath.FromSlash(tailwindAssetPath))
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), projectDirPerm); err != nil {
		return fmt.Errorf("tailwind: mkdir assets: %w", err)
	}
	if err := os.WriteFile(path, []byte(tailwindInputTemplate), plugin.ProjectFilePerm); err != nil {
		return fmt.Errorf("tailwind: write input: %w", err)
	}
	slog.Info("tailwind: created", "file", tailwindAssetPath)
	return nil
}

// detectPackageManager finds bun, pnpm, yarn, or npm based on lockfile presence and PATH.
