package config

import "os"

// RequireTS reports whether RequireTSEnv is set.
func RequireTS() bool { return os.Getenv(RequireTSEnv) != "" }
