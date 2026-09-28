package packages

import (
	"fmt"
	"os/exec"
)

func goGet(root, pkg, version string) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("go not found in PATH")
	}
	target := pkg
	if version != "" && version != "latest" {
		if version[0] != 'v' {
			version = "v" + version
		}
		target = pkg + "@" + version
	} else {
		target = pkg + "@latest"
	}
	cmd := exec.Command("go", "get", target)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go get %s: %v: %s", target, err, string(out))
	}
	return nil
}

func goRemove(root, pkg string) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("go not found in PATH")
	}
	// Go 1.21+: go get pkg@none or go mod edit -droprequire
	cmd := exec.Command("go", "get", pkg+"@none")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	// fallback to go mod edit
	cmd2 := exec.Command("go", "mod", "edit", "-droprequire", pkg)
	cmd2.Dir = root
	out2, err2 := cmd2.CombinedOutput()
	if err2 != nil {
		return fmt.Errorf("go remove %s: %v: %s (fallback %v: %s)", pkg, err, string(out), err2, string(out2))
	}
	return nil
}
