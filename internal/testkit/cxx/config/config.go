package config

import "os"

// RequireCxx reports whether RequireCxxEnv is set.
func RequireCxx() bool { return os.Getenv(RequireCxxEnv) != "" }

// NlohmannInclude reports NlohmannIncludeEnv's value and whether it is set.
func NlohmannInclude() (string, bool) {
	dir := os.Getenv(NlohmannIncludeEnv)
	return dir, dir != ""
}

// RequireMSVC reports whether RequireMSVCEnv is set.
func RequireMSVC() bool { return os.Getenv(RequireMSVCEnv) != "" }
