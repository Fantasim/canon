package check

// goKeywords are Go's keywords, which a Go package name may not be (CODEGEN.md §2.1).
var goKeywords map[string]bool

// cppNamespaces are namespaces generated C++ names, closed to an emit's (log-2026-09-24, round 3).
var cppNamespaces map[string]bool

// cppKeywords are C++20's keywords and alternative tokens, which a namespace may not use.
var cppKeywords map[string]bool

// windowsDevices are the Windows device names, upper case, which no portable file name's stem may be (CODEGEN.md §2.9, DECISIONS 297).
var windowsDevices map[string]bool

func init() {
	goKeywords = map[string]bool{
		"break": true, "case": true, "chan": true, "const": true, "continue": true, "default": true,
		"defer": true, "else": true, "fallthrough": true, "for": true, "func": true, "go": true,
		"goto": true, "if": true, "import": true, "interface": true, "map": true, "package": true,
		"range": true, "return": true, "select": true, "struct": true, "switch": true, "type": true,
		"var": true,
	}
	cppNamespaces = map[string]bool{"canon": true, "std": true, "nlohmann": true}
	windowsDevices = map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}
	cppKeywords = map[string]bool{
		"alignas": true, "alignof": true, "and": true, "and_eq": true, "asm": true, "auto": true,
		"bitand": true, "bitor": true, "bool": true, "break": true, "case": true, "catch": true,
		"char": true, "char8_t": true, "char16_t": true, "char32_t": true, "class": true, "compl": true,
		"concept": true, "const": true, "consteval": true, "constexpr": true, "constinit": true,
		"const_cast": true, "continue": true, "co_await": true, "co_return": true, "co_yield": true,
		"decltype": true, "default": true, "delete": true, "do": true, "double": true,
		"dynamic_cast": true, "else": true, "enum": true, "explicit": true, "export": true,
		"extern": true, "false": true, "float": true, "for": true, "friend": true, "goto": true,
		"if": true, "inline": true, "int": true, "long": true, "mutable": true, "namespace": true,
		"new": true, "noexcept": true, "not": true, "not_eq": true, "nullptr": true, "operator": true,
		"or": true, "or_eq": true, "private": true, "protected": true, "public": true, "register": true,
		"reinterpret_cast": true, "requires": true, "return": true, "short": true, "signed": true,
		"sizeof": true, "static": true, "static_assert": true, "static_cast": true, "struct": true,
		"switch": true, "template": true, "this": true, "thread_local": true, "throw": true,
		"true": true, "try": true, "typedef": true, "typeid": true, "typename": true, "union": true,
		"unsigned": true, "using": true, "virtual": true, "void": true, "volatile": true,
		"wchar_t": true, "while": true, "xor": true, "xor_eq": true,
	}
}
