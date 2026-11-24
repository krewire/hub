package redis_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestRedisDependencies asserts that pkg/redis adheres strictly to the
// downward-only dependency pyramid rule. It may only depend on standard library
// packages and Level 1 packages (pkg/errs, pkg/model).
func TestRedisDependencies(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse dir: %v", err)
	}

	for pkgName, pkg := range pkgs {
		for filename, file := range pkg.Files {
			for _, imp := range file.Imports {
				path, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					t.Fatalf("unquote %s: %v", imp.Path.Value, err)
				}

				if strings.HasPrefix(path, "github.com/krewire/krewire/") {
					sub := strings.TrimPrefix(path, "github.com/krewire/krewire/")
					if sub != "errs" && sub != "model" {
						t.Errorf("%s (%s): illegal upward dependency on %s (redis may only depend on errs and model)",
							filepath.Base(filename), pkgName, path)
					}
				}
			}
		}
	}
}
