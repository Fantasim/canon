package api

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	"example.com/svc/internal/config"
	"example.com/svc/internal/db"
)

const maxItems = 10

var (
	table = []string{"alpha", "alpha"}
	re    = regexp.MustCompile("a+")
)

var errMisplaced = errors.New("misplaced")

var mu sync.Mutex

var _ = exec.Command

type payload struct {
	Name string `json:"name"`
}

type other struct {
	Name string `json:"name"`
}

// Server serves the API.
type Server struct{ db *db.Store }

// Options configures New.
type Options struct{ Addr string }

// Hidden is only used here.
type Hidden struct{}

// New builds a server.
func New(o Options) *Server { return &Server{} }

// Start starts it.
func (s *Server) Start() { s.Internal() }

// Internal is only called here.
func (s *Server) Internal() { _ = Hidden{} }

// Name satisfies db.Namer.
func (s *Server) Name() string { return "application/json" }

// ServeHTTP answers.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" || r.Header.Get("X-Method") == "GET" {
		http.Error(w, "bad", http.StatusBadRequest)
	}
	_ = fmt.Sprintf("%s: %s", "only-here", "only-here")
	_ = ", " + ", " + "x" + "x" + "unique"
	_ = `^([0-9]{4})_([a-z0-9_]+)\.sql$` + `^([0-9]{4})_([a-z0-9_]+)\.sql$`
	time.Sleep(5 * time.Second)
	a, b, c := 0, 1, -1
	_ = 5 * b
	_ = strconv.FormatInt(int64(a+b+c), 10)
	_ = os.Getenv(config.EnvPort)
	_, _, _, _ = table, re, errMisplaced, &mu
}

// Helper is only used here.
func Helper() error {
	const localMax = 4
	if localMax > maxItems {
		return errors.New("inline")
	}
	if err := errors.Join(); err != nil {
		return fmt.Errorf("wrap: %w", err)
	}
	return fmt.Errorf("no wrap %d", localMax)
}

// Used is used by cmd/server.
func Used() {
	log.Printf("x")
	fmt.Println("y")
	println("z")
	go worker()
	go func() { Helper() }()
	_, _ = payload{}, other{}
	_ = MaxThing
	TestOnly()
}

func worker() {}

// TestOnly is only used by this package's tests.
func TestOnly() {}

// XTestUsed is only used by the external test package.
func XTestUsed() {}

// Dup collides with dup.
func Dup() { dup() }

func dup() {}
