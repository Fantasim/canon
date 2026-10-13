package build

import (
	"cmp"
	"runtime/debug"
	"strings"

	"github.com/fantasim/canonlang/internal/project"
)

// Compiler is the running compiler's version: link-time, else a release tag's, else "dev" (DECISIONS 354).
func Compiler() string {
	return cmp.Or(CompilerVersion, moduleRelease(), project.DevVersion)
}

// moduleRelease is the release version the running binary's build information carries, "" for none.
func moduleRelease() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return releaseOf(info)
}

// releaseOf is the version, without its "v", of the canon module in info when it is a release tag: the main module, else a dependency that is not replaced.
func releaseOf(info *debug.BuildInfo) string {
	version := ""
	if info.Main.Path == ModulePath {
		version = info.Main.Version
	}
	for _, dep := range info.Deps {
		if dep.Path == ModulePath && dep.Replace == nil {
			version = dep.Version
		}
	}
	if tag, ok := strings.CutPrefix(version, tagPrefix); ok && project.IsCompilerVersion(tag) {
		return tag
	}
	return ""
}
