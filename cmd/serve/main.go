package main

import (
	"github.com/dhis2-sre/im-manager/internal/di"
	"github.com/dhis2-sre/im-manager/internal/server"
	"log"
)

// @title Instance Manager Manager Service
// @version 0.1.0
func main() {
	environment := di.GetEnvironment()
	r := server.GetEngine(environment)
	if err := r.Run(); err != nil {
		log.Fatal(err)
	}
}
