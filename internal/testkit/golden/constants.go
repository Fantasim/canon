package golden

import "os"

const (
	wantFile             = "want"
	filePerm os.FileMode = 0o600
	dirPerm  os.FileMode = 0o700

	updateFlag  = "update"
	updateUsage = "rewrite the expected file of every golden case instead of comparing"
)
