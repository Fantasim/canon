package config

import "os"

// RequireCxx reports whether RequireCxxEnv is set.
func RequireCxx() bool { return os.Getenv(RequireCxxEnv) != "" }
