package ship

import "github.com/krewire/libs/core"

// Version is the ship module version.
var Version = core.MustParseVersion("0.0.0")

// EcosystemRequires declares the minimum versions of other modules this version is compatible with.
var EcosystemRequires = map[core.ModuleName]core.Version{
	core.ModuleLibs: core.MustParseVersion("0.4.0"),
}
