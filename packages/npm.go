package packages

import (
	"fmt"
	"os/exec"
)

func npmInstall(root, pkg string, dev bool) error {
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm not found in PATH")
	}
	args := []string{"install", pkg}
	if dev {
		args = []string{"install", "-D", pkg}
	}
	cmd := exec.Command("npm", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("npm install %s: %v: %s", pkg, err, string(out))
	}
	return nil
}

func npmUninstall(root, pkg string) error {
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm not found in PATH")
	}
	cmd := exec.Command("npm", "uninstall", pkg)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("npm uninstall %s: %v: %s", pkg, err, string(out))
	}
	return nil
}
