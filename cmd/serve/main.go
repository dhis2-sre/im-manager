package main

import (
	"github.com/dhis2-sre/im-manager/internal/di"
	"github.com/dhis2-sre/im-manager/internal/server"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"log"
)

// @title Instance Manager Manager Service
// @version 0.1.0
func main() {
	environment := di.GetEnvironment()

	stack.LoadStacks(environment.StackService)

	r := server.GetEngine(environment)
	if err := r.Run(); err != nil {
		log.Fatal(err)
	}
}
