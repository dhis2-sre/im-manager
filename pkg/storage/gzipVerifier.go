package storage

import (
	"compress/gzip"
	"io"
)

// GzipVerifier reads a gzip stream written to it, passes the decompressed bytes on, and tells on
// Close whether the stream was whole. It lets an upload be checked while it runs, since a stream
// that was cut short on its way otherwise looks like a successful transfer.
type GzipVerifier struct {
	writer *io.PipeWriter
	done   chan error
}

func NewGzipVerifier(decompressed io.Writer) *GzipVerifier {
	reader, writer := io.Pipe()
	verifier := &GzipVerifier{writer: writer, done: make(chan error, 1)}

	go func() {
		err := gunzip(decompressed, reader)
		// Keep reading after a failure, so whoever writes to the verifier is never blocked.
		_, _ = io.Copy(io.Discard, reader)
		verifier.done <- err
	}()

	return verifier
}

func gunzip(w io.Writer, r io.Reader) error {
	gzReader, err := gzip.NewReader(r)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, gzReader) //nolint:gosec // the bytes are only checked while they pass, never kept

	return err
}

func (v *GzipVerifier) Write(p []byte) (int, error) {
	return v.writer.Write(p)
}

// Close ends the stream and reports whether it was a whole gzip stream.
func (v *GzipVerifier) Close() error {
	_ = v.writer.Close()
	return <-v.done
}
