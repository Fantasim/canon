package api

import (
	"log/slog"
	"net/http"
	"time"
)

func logger() *slog.Logger { return slog.Default() }

func quiet(m map[string]int, mux *http.ServeMux) {
	slog.Info("quiet-msg", "quiet-key", 1)
	slog.Info("quiet-msg", "quiet-key", 1)
	logger().Warn("quiet-msg", "quiet-key", 1)
	_ = map[string]int{"quiet-map": 1}
	_ = map[string]int{"quiet-map": 1}
	m["quiet-map"] = m["quiet-map"] + 1
	mux.HandleFunc("/quiet/path", nil)
	mux.Handle("/quiet/path", nil)
	_ = "loud-value" + "loud-value"
	x := len(m)
	_ = x<<12 + x>>4 + x*2
	x <<= 3
	_ = 24 * time.Hour
	_ = 24 * 60 * time.Minute
	_ = time.Duration(30) * time.Second
	_ = x * 7
}

var _ = quiet
