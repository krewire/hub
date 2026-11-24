package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	ProjectFilePerm = 0o644
	ProjectDirPerm  = 0o755
)

// DetectPackageManager inspects root lockfiles and PATH to find npm, bun, pnpm, or yarn.
func DetectPackageManager(root string) string {
	if _, err := os.Stat(filepath.Join(root, "bun.lockb")); err == nil {
		if _, err := exec.LookPath("bun"); err == nil {
			return "bun"
		}
	}
	if _, err := os.Stat(filepath.Join(root, "bun.lock")); err == nil {
		if _, err := exec.LookPath("bun"); err == nil {
			return "bun"
		}
	}
	if _, err := os.Stat(filepath.Join(root, "pnpm-lock.yaml")); err == nil {
		if _, err := exec.LookPath("pnpm"); err == nil {
			return "pnpm"
		}
	}
	if _, err := os.Stat(filepath.Join(root, "yarn.lock")); err == nil {
		if _, err := exec.LookPath("yarn"); err == nil {
			return "yarn"
		}
	}
	if _, err := exec.LookPath("npm"); err == nil {
		return "npm"
	}
	if _, err := exec.LookPath("bun"); err == nil {
		return "bun"
	}
	if _, err := exec.LookPath("pnpm"); err == nil {
		return "pnpm"
	}
	if _, err := exec.LookPath("yarn"); err == nil {
		return "yarn"
	}
	return ""
}

// InstallPackage installs an npm package into root using the detected package manager.
func InstallPackage(root, pkg string, dev bool) error {
	pm := DetectPackageManager(root)
	if pm == "" {
		return fmt.Errorf("no package manager (npm, bun, pnpm, yarn) found in PATH")
	}

	var args []string
	switch pm {
	case "bun":
		args = []string{"add", pkg}
		if dev {
			args = []string{"add", "-d", pkg}
		}
	case "pnpm":
		args = []string{"add", pkg}
		if dev {
			args = []string{"add", "-D", pkg}
		}
	case "yarn":
		args = []string{"add", pkg}
		if dev {
			args = []string{"add", "-D", pkg}
		}
	default: // npm
		args = []string{"install", pkg}
		if dev {
			args = []string{"install", "-D", pkg}
		}
	}

	cmd := exec.Command(pm, args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s %s: %v: %s", pm, args[0], pkg, err, string(out))
	}
	return nil
}

// UninstallPackage removes an npm package from root.
func UninstallPackage(root, pkg string) error {
	pm := DetectPackageManager(root)
	if pm == "" {
		return fmt.Errorf("no package manager (npm, bun, pnpm, yarn) found in PATH")
	}

	var args []string
	switch pm {
	case "bun":
		args = []string{"remove", pkg}
	case "pnpm":
		args = []string{"remove", pkg}
	case "yarn":
		args = []string{"remove", pkg}
	default:
		args = []string{"uninstall", pkg}
	}

	cmd := exec.Command(pm, args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s %s: %v: %s", pm, args[0], pkg, err, string(out))
	}
	return nil
}

// HasExtInDirs checks if any file with ext exists within candidate directories.
func HasExtInDirs(root string, dirs []string, ext string) bool {
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

// BuildViteCandidateCommands prepares possible runner invocations for Vite.
func BuildViteCandidateCommands(root, outDir string) [][]string {
	var candidates [][]string
	targetOut := filepath.Join(outDir, "assets")

	candidates = append(candidates,
		[]string{"./node_modules/.bin/vite", "build", "--outDir", targetOut},
	)
	if _, err := os.Stat(filepath.Join(root, "bin/vite")); err == nil {
		candidates = append(candidates, []string{"./bin/vite", "build", "--outDir", targetOut})
	}
	if _, err := exec.LookPath("vite"); err == nil {
		candidates = append(candidates, []string{"vite", "build", "--outDir", targetOut})
	}
	if _, err := exec.LookPath("bun"); err == nil {
		candidates = append(candidates, []string{"bunx", "vite", "build", "--outDir", targetOut})
	}
	if _, err := exec.LookPath("pnpm"); err == nil {
		candidates = append(candidates, []string{"pnpm", "exec", "vite", "build", "--outDir", targetOut})
	}
	if _, err := exec.LookPath("yarn"); err == nil {
		candidates = append(candidates, []string{"yarn", "vite", "build", "--outDir", targetOut})
	}
	candidates = append(candidates, []string{"npx", "vite", "build", "--outDir", targetOut})
	return candidates
}
