// Package inttest provides isolated integration-test fixtures on shared services.
// make test starts one service set for the entire run. Direct go test invocations
// lazily start one set per package; packages using these helpers must call Main
// from TestMain. Each SetupDB, SetupRedis, SetupS3 and SetupMinIO call owns its data
// until t.Cleanup runs, including all subtests. Stop fixture workers before then.
package inttest
