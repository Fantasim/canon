package main

import (
	"log"

	"example.com/svc/internal/api"
	"example.com/svc/internal/config"
	"example.com/svc/internal/db"
	"example.com/svc/internal/e2e"
)

// Run is exported in main.
func Run() {}

func main() {
	s := api.New(api.Options{})
	s.Start()
	api.Used()
	log.Printf("%s %v", db.Describe(s), config.Port())
	_ = config.Retired(1)
	_ = e2e.Fail
	Run()
}
