// Command testenv starts one set of services for all packages in a go test run.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/dhis2-sre/im-manager/internal/testenv"
)

func main() { os.Exit(run()) }

func run() (code int) {
	flag.Parse()
	if err := testenv.Configure(); err != nil {
		fmt.Fprintln(os.Stderr, "configure test environment:", err)
		return 1
	}
	started := time.Now()
	var setup, tests time.Duration
	e := testenv.New()
	defer func() {
		cleanup := time.Now()
		if err := e.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "test service cleanup:", err)
			code = 1
		}
		cleanupTime, total := time.Since(cleanup), time.Since(started)
		fmt.Fprintf(os.Stderr, "test timing: cleanup=%s total=%s\n", cleanupTime.Round(time.Millisecond), total.Round(time.Millisecond))
		writeTimingSummary(setup, tests, cleanupTime, total, code)
	}()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	c, err := e.StartAll()
	setup = time.Since(started)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	config, err := json.Marshal(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"-race", "-count=1", "./..."}
	}
	cmd := exec.CommandContext(ctx, "go", append([]string{"test"}, args...)...) // #nosec G204 -- forwards explicit developer-supplied go test arguments.
	cmd.Env = append(os.Environ(), testenv.ConfigEnv+"="+string(config))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	// Cancel the entire go test process group before removing its services.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	cmd.WaitDelay = 30 * time.Second
	testsStarted := time.Now()
	err = cmd.Run()
	tests = time.Since(testsStarted)
	fmt.Fprintf(os.Stderr, "test timing: setup=%s tests=%s\n", setup.Round(time.Millisecond), tests.Round(time.Millisecond))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() > 0 {
			return exit.ExitCode()
		}
		return 1
	}
	return 0
}

func writeTimingSummary(setup, tests, cleanup, total time.Duration, code int) {
	if path := os.Getenv("GITHUB_STEP_SUMMARY"); path != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err == nil {
			_, err = fmt.Fprintf(f, "\n### Go test timing\n\n| Phase | Duration |\n|---|---:|\n| Shared service setup | %s |\n| Go test (including compilation) | %s |\n| Service cleanup | %s |\n| Total runner time | %s |\n\nExit code: %d.\n", setup.Round(time.Millisecond), tests.Round(time.Millisecond), cleanup.Round(time.Millisecond), total.Round(time.Millisecond), code)
			_ = f.Close()
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "write test timing summary:", err)
		}
	}
}
