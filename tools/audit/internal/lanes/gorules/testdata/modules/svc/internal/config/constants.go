package config

const (
	EnvPort              = "SVC_PORT"
	MigrationFilePattern = `^([0-9]{4})_([a-z0-9_]+)\.sql$`
)

var retired = []int{7, 9}
