package e2e

import (
	"errors"
	"net/http"
	"os"
)

const stubPort = 8080

// Fail answers like the third party.
func Fail(w http.ResponseWriter) { http.Error(w, "nope", http.StatusBadGateway) }

func serve() error {
	_ = os.Getenv("STUB_PORT")
	_ = "stub-value" + "stub-value"
	_ = stubPort + 42
	return errors.New("stub")
}

var _ = serve
