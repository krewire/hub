package packages

import (
	"fmt"
	"strings"

	"github.com/krewire/libs/core"
	"github.com/krewire/hub/plugin"
)

// Installer is the install/uninstall contract for a resolved package.
type Installer interface {
	Kind() string
	Add(root, version string) error
	Remove(root string) error
	Describe() string
}

// Resolved is the result of resolving a Spec.
type Resolved struct {
	Spec      Spec
	Installer Installer
	Kind      string // plugin|npm|gomod
}

// EffectiveVersion returns the version string to use for installation,
// using core.Version compatibility semantics.
func (r *Resolved) EffectiveVersion() string {
	if r.Spec.IsLatest() {
		return "latest"
	}
	return r.Spec.RawVer
}

// SatisfiesRequired reports whether the resolved package's version satisfies
// the required version per semver caret semantics.
func (r *Resolved) SatisfiesRequired(required core.Version) bool {
	return r.Spec.Satisfies(required)
}

// Resolver resolves a Spec to an Installer. It returns nil if it cannot handle the spec.
type Resolver interface {
	Resolve(spec Spec) (Installer, string, error)
}

// Chain is an ordered list of resolvers tried in sequence.
type Chain []Resolver

func (c Chain) Resolve(spec Spec) (*Resolved, error) {
	for _, r := range c {
		ins, kind, err := r.Resolve(spec)
		if err != nil {
			return nil, err
		}
		if ins != nil {
			return &Resolved{Spec: spec, Installer: ins, Kind: kind}, nil
		}
	}
	return nil, fmt.Errorf("no resolver for package %q", spec.Name)
}

// DefaultChain is the scalable resolver chain: plugin → gomod → npm.
// Plugin first so `twcss`, `tailwind`, `tailwindcss` resolve to the Krewire
// plugin instead of generic npm. Go modules (containing "/" and ".") are
// detected before npm to avoid treating `github.com/...` as npm.
func DefaultChain() Chain {
	return Chain{
		&PluginResolver{},
		&GoResolver{},
		&NpmResolver{},
	}
}

// PluginResolver resolves Krewire plugin names via plugin.FindInstaller.
type PluginResolver struct{}

func (r *PluginResolver) Resolve(spec Spec) (Installer, string, error) {
	ins := plugin.FindInstaller(spec.Name)
	if ins == nil {
		return nil, "", nil
	}
	return &pluginAdapter{ins: ins, name: spec.Name}, "plugin", nil
}

type pluginAdapter struct {
	ins  plugin.Installer
	name string
}

func (p *pluginAdapter) Kind() string                   { return "plugin" }
func (p *pluginAdapter) Describe() string               { return "plugin:" + p.ins.Name() }
func (p *pluginAdapter) Add(root, version string) error { return p.ins.Add(root, version) }
func (p *pluginAdapter) Remove(root string) error       { return p.ins.Remove(root) }

// GoResolver handles Go module paths (contains "/" and a dot in first segment).
type GoResolver struct{}

func (r *GoResolver) Resolve(spec Spec) (Installer, string, error) {
	name := spec.Name
	if !strings.Contains(name, "/") {
		return nil, "", nil
	}
	// Heuristic: Go modules have a dot in the first segment (e.g., github.com)
	first := strings.Split(name, "/")[0]
	if !strings.Contains(first, ".") {
		return nil, "", nil
	}
	return &goInstaller{name: name}, "gomod", nil
}

type goInstaller struct{ name string }

func (g *goInstaller) Kind() string                   { return "gomod" }
func (g *goInstaller) Describe() string               { return "go:" + g.name }
func (g *goInstaller) Add(root, version string) error { return goGet(root, g.name, version) }
func (g *goInstaller) Remove(root string) error       { return goRemove(root, g.name) }

// NpmResolver is the fallback for npm packages.
type NpmResolver struct{}

func (r *NpmResolver) Resolve(spec Spec) (Installer, string, error) {
	// Accept any name that is not empty; npm allows wide range.
	if spec.Name == "" {
		return nil, "", nil
	}
	return &npmInstaller{name: spec.Name}, "npm", nil
}

type npmInstaller struct{ name string }

func (n *npmInstaller) Kind() string     { return "npm" }
func (n *npmInstaller) Describe() string { return "npm:" + n.name }
func (n *npmInstaller) Add(root, version string) error {
	pkg := n.name
	if version != "" && version != "latest" {
		pkg = n.name + "@" + version
	} else if version == "latest" {
		pkg = n.name + "@latest"
	}
	return npmInstall(root, pkg, false)
}
func (n *npmInstaller) Remove(root string) error { return npmUninstall(root, n.name) }
