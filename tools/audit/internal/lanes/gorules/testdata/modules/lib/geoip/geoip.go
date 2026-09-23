package geoip

import "os"

// Path reads the database path.
func Path() string {
	return os.Getenv(envPath) + `^([0-9]{4})_([a-z0-9_]+)\.sql$` + `^([0-9]{4})_([a-z0-9_]+)\.sql$`
}

// Watch refreshes in the background.
func Watch() {
	go Path()
}
