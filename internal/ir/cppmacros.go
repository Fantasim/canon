package ir

import (
	_ "embed"
	"strings"
)

//go:embed text/cpp_macros.txt
var cppMacroText string

// parseMacroLists reads the two sections of the macro file, [std] (E8011) then [warn] (W8006): a header switches section, blank and comment lines are skipped (CODEGEN.md §3.4, §3.5).
func parseMacroLists(text string) (std, warn []string) {
	section := &std
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || strings.HasPrefix(line, macroComment):
		case line == macroStdHeader:
			section = &std
		case line == macroWarnHeader:
			section = &warn
		default:
			*section = append(*section, line)
		}
	}
	return std, warn
}

// cppMacros is the [warn] section as a set, W8006 (CODEGEN.md §3.5), built once.
var cppMacros = func() map[string]bool {
	_, warn := parseMacroLists(cppMacroText)
	return nameSet(warn...)
}()

// cppStdMacros is the [std] section as a set, E8011 (CODEGEN.md §3.4), built once.
var cppStdMacros = func() map[string]bool {
	std, _ := parseMacroLists(cppMacroText)
	return nameSet(std...)
}()
