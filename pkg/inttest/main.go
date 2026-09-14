package inttest

import (
	"fmt"
	"os"
	"testing"

	"github.com/dhis2-sre/im-manager/internal/testenv"
)

// Main closes package-local services after all tests and subtest cleanups finish.
// Runner-owned services are closed by the runner after every package has exited.
func Main(m *testing.M) {
	if err := testenv.Configure(); err != nil {
		fmt.Fprintln(os.Stderr, "configure test environment:", err)
		os.Exit(1)
	}
	code := m.Run()
	if err := testenv.CloseLocal(); err != nil {
		fmt.Fprintln(os.Stderr, "test service cleanup:", err)
		code = 1
	}
	os.Exit(code)
}
