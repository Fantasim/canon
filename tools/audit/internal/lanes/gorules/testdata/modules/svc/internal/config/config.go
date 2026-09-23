package config

import (
	"os"
	"slices"
)

// Port reads the listen port.
func Port() string { return os.Getenv(EnvPort) + os.Getenv("SVC_DEBUG") }

// Retired reports a retired id.
func Retired(id int) bool { return slices.Contains(retired, id) }
