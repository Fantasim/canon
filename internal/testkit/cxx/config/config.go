package config

import (
	"os"
	"strconv"
)

// Slots reports SlotsEnv's value as a positive count, and whether it holds one.
func Slots() (int, bool) {
	n, err := strconv.Atoi(os.Getenv(SlotsEnv))
	return n, err == nil && n > 0
}

// RequireCxx reports whether RequireCxxEnv is set.
func RequireCxx() bool { return os.Getenv(RequireCxxEnv) != "" }

// NlohmannInclude reports NlohmannIncludeEnv's value and whether it is set.
func NlohmannInclude() (string, bool) {
	dir := os.Getenv(NlohmannIncludeEnv)
	return dir, dir != ""
}

// RequireMSVC reports whether RequireMSVCEnv is set.
func RequireMSVC() bool { return os.Getenv(RequireMSVCEnv) != "" }
