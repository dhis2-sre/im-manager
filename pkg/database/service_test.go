package database

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const completeDump = "--\n-- PostgreSQL database dump\n--\nCOPY userinfo (userinfoid) FROM stdin;\n1\n\\.\n--\n-- PostgreSQL database dump complete\n--\n"

// writerExecutor writes a fixed payload to stdout and returns execErr, standing in for a pod exec
// that streams pg_dump's output.
type writerExecutor struct {
	payload string
	execErr error
}

func (e writerExecutor) Exec(_ context.Context, _, _, _ string, _ []string, stdout, _ io.Writer) error {
	if _, err := io.WriteString(stdout, e.payload); err != nil {
		return err
	}
	return e.execErr
}

// drainPipe consumes pr until EOF or error, mimicking the S3 upload goroutine, and reports what the
// upload side would have seen.
func drainPipe(t *testing.T, pr *io.PipeReader) (<-chan []byte, <-chan error) {
	t.Helper()

	dataCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		data, err := io.ReadAll(pr)
		dataCh <- data
		errCh <- err
	}()
	return dataCh, errCh
}

// gzipped compresses the text the way pg_dump does for a plain dump run with --compress.
func gzipped(t *testing.T, text string) string {
	t.Helper()

	var compressed strings.Builder
	writer := gzip.NewWriter(&compressed)
	_, err := io.WriteString(writer, text)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	return compressed.String()
}

func TestExecPgDumpRejectsTruncatedPlainDump(t *testing.T) {
	whole := gzipped(t, completeDump)

	// A pod exec can report success having delivered only part of pg_dump's stdout.
	payloads := map[string]string{
		"TheGzipStreamIsCutShort":        whole[:len(whole)-4],
		"NothingArrived":                 "",
		"TheTextEndsBeforeItsLastLine":   gzipped(t, completeDump[:len(completeDump)/2]),
		"TheOutputIsNotGzip":             completeDump,
		"OnlyTheStartOfTheStreamArrived": whole[:len(whole)/2],
	}
	for name, payload := range payloads {
		t.Run(name, func(t *testing.T) {
			pr, pw := io.Pipe()
			_, uploadErrCh := drainPipe(t, pr)

			err := execPgDump(context.Background(), writerExecutor{payload: payload}, "ns", "pod", "postgresql", []string{"pg_dump"}, pw, "plain")

			require.ErrorIs(t, err, errDumpTruncated, "a truncated dump must not be reported as a successful save")
			assert.Error(t, <-uploadErrCh, "the upload must see an error rather than a clean EOF, so it aborts")
		})
	}
}

func TestExecPgDumpAcceptsCompletePlainDump(t *testing.T) {
	whole := gzipped(t, completeDump)
	executor := writerExecutor{payload: whole}

	pr, pw := io.Pipe()
	dataCh, uploadErrCh := drainPipe(t, pr)

	err := execPgDump(context.Background(), executor, "ns", "pod", "postgresql", []string{"pg_dump"}, pw, "plain")

	require.NoError(t, err)
	require.NoError(t, <-uploadErrCh)
	assert.Equal(t, whole, string(<-dataCh), "the compressed dump is uploaded as pg_dump produced it")
}

func TestExecPgDumpPropagatesExecError(t *testing.T) {
	executor := writerExecutor{payload: gzipped(t, completeDump), execErr: fmt.Errorf("connection lost")}

	pr, pw := io.Pipe()
	_, uploadErrCh := drainPipe(t, pr)

	err := execPgDump(context.Background(), executor, "ns", "pod", "postgresql", []string{"pg_dump"}, pw, "plain")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection lost")
	assert.NotErrorIs(t, err, errDumpTruncated, "a failed exec is reported as what it is, not retried as a truncated dump")
	assert.Error(t, <-uploadErrCh)
}

func TestExecPgDumpSkipsTrailerCheckForArchiveFormat(t *testing.T) {
	// Custom format archives carry no plain text trailer, pg_restore validates them instead.
	executor := writerExecutor{payload: "PGDMP-binary-ish"}

	pr, pw := io.Pipe()
	dataCh, uploadErrCh := drainPipe(t, pr)

	err := execPgDump(context.Background(), executor, "ns", "pod", "postgresql", []string{"pg_dump"}, pw, "custom")

	require.NoError(t, err)
	require.NoError(t, <-uploadErrCh)
	assert.Equal(t, "PGDMP-binary-ish", string(<-dataCh), "archive output must be uploaded uncompressed and unaltered")
}

func TestTailWriterRetainsOnlyTheLastMaxBytes(t *testing.T) {
	var sink strings.Builder
	tail := &tailWriter{w: &sink, max: 8}

	for _, chunk := range []string{"aaaa", "bbbb", "cccc"} {
		n, err := tail.Write([]byte(chunk))
		require.NoError(t, err)
		require.Equal(t, len(chunk), n)
	}

	assert.Equal(t, "aaaabbbbcccc", sink.String(), "every byte must still reach the underlying writer")
	assert.Equal(t, "bbbbcccc", string(tail.tail))
	assert.True(t, tail.contains("cccc"))
	assert.False(t, tail.contains("aaaa"))
}

func TestPodDumpDoesNotRequireHostPgDump(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	instance := &model.DeploymentInstance{
		Parameters: model.DeploymentInstanceParameters{
			"DATABASE_NAME":     {Value: "dhis2"},
			"DATABASE_USERNAME": {Value: "dhis"},
			"DATABASE_PASSWORD": {Value: "test-password"},
		},
		Group: &model.Group{Namespace: "test"},
	}
	dump, err := newPgDumpConfig(instance, &stack.Stack{HostnamePattern: "%s.%s"})
	require.NoError(t, err)
	command := buildPgDumpCommand(dump, "custom")
	require.GreaterOrEqual(t, len(command), 3)
	assert.Equal(t, []string{"env", "PGPASSWORD=test-password", "pg_dump"}, command[:3])
	assert.Contains(t, command, "--dbname=dhis2")
	assert.Contains(t, command, "--exclude-table=dhis2_chart_seed_complete")
	assert.NotContains(t, command, "--compress=6", "a custom format archive is compressed by pg_dump already")

	assert.Contains(t, buildPgDumpCommand(dump, "plain"), "--compress=6", "a plain dump is compressed in the pod")
}
