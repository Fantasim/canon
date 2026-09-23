package golden

import "os"

const (
	wantFile             = "want"
	filePerm os.FileMode = 0o600

	updateFlag  = "update"
	updateUsage = "rewrite the expected file of every golden case instead of comparing"
)
