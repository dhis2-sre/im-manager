package database

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

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

func TestExecPgDumpRejectsTruncatedPlainDump(t *testing.T) {
	// A pod exec can report success having delivered only part of pg_dump's stdout.
	truncated := completeDump[:len(completeDump)/2]
	executor := writerExecutor{payload: truncated}

	pr, pw := io.Pipe()
	_, uploadErrCh := drainPipe(t, pr)

	err := execPgDump(context.Background(), executor, "ns", "pod", []string{"pg_dump"}, pw, "plain", "db.gz")

	require.Error(t, err, "a truncated dump must not be reported as a successful save")
	assert.Contains(t, err.Error(), "truncated")
	assert.Error(t, <-uploadErrCh, "the upload must see an error rather than a clean EOF, so it aborts")
}

func TestExecPgDumpAcceptsCompletePlainDump(t *testing.T) {
	executor := writerExecutor{payload: completeDump}

	pr, pw := io.Pipe()
	dataCh, uploadErrCh := drainPipe(t, pr)

	err := execPgDump(context.Background(), executor, "ns", "pod", []string{"pg_dump"}, pw, "plain", "db.gz")

	require.NoError(t, err)
	require.NoError(t, <-uploadErrCh)

	gzReader, err := gzip.NewReader(strings.NewReader(string(<-dataCh)))
	require.NoError(t, err)
	restored, err := io.ReadAll(gzReader)
	require.NoError(t, err)
	assert.Equal(t, completeDump, string(restored))
}

func TestExecPgDumpPropagatesExecError(t *testing.T) {
	executor := writerExecutor{payload: completeDump, execErr: fmt.Errorf("connection lost")}

	pr, pw := io.Pipe()
	_, uploadErrCh := drainPipe(t, pr)

	err := execPgDump(context.Background(), executor, "ns", "pod", []string{"pg_dump"}, pw, "plain", "db.gz")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection lost")
	assert.Error(t, <-uploadErrCh)
}

func TestExecPgDumpSkipsTrailerCheckForArchiveFormat(t *testing.T) {
	// Custom format archives carry no plain text trailer, pg_restore validates them instead.
	executor := writerExecutor{payload: "PGDMP-binary-ish"}

	pr, pw := io.Pipe()
	dataCh, uploadErrCh := drainPipe(t, pr)

	err := execPgDump(context.Background(), executor, "ns", "pod", []string{"pg_dump"}, pw, "custom", "db")

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
