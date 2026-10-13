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

// releaseOf is the version, without its "v", of the canon module in info when it is a release tag.
func releaseOf(info *debug.BuildInfo) string {
	version := ""
	if m := CanonModule(info); m != nil {
		version = m.Version
	}
	if tag, ok := strings.CutPrefix(version, tagPrefix); ok && project.IsCompilerVersion(tag) {
		return tag
	}
	return ""
}

// CanonModule is the canon module in info: the main module (&info.Main) when canon is it, else
// the dependency on it that is not replaced; nil for neither.
func CanonModule(info *debug.BuildInfo) *debug.Module {
	if info.Main.Path == ModulePath {
		return &info.Main
	}
	for _, dep := range info.Deps {
		if dep.Path == ModulePath && dep.Replace == nil {
			return dep
		}
	}
	return nil
}
