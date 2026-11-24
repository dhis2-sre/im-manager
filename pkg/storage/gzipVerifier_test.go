package storage

import (
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGzipVerifier(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := io.WriteString(writer, strings.Repeat("some content ", 1000))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	whole := compressed.Bytes()

	verify := func(t *testing.T, stream []byte) (string, error) {
		t.Helper()

		var decompressed strings.Builder
		verifier := NewGzipVerifier(&decompressed)
		_, err := verifier.Write(stream)
		require.NoError(t, err, "a verifier must take every write, whatever it makes of the stream")

		err = verifier.Close()

		return decompressed.String(), err
	}

	t.Run("AcceptsAWholeStreamAndPassesItsContentOn", func(t *testing.T) {
		decompressed, err := verify(t, whole)

		require.NoError(t, err)
		assert.Equal(t, strings.Repeat("some content ", 1000), decompressed)
	})

	rejected := map[string][]byte{
		"AStreamCutShortAtItsEnd":   whole[:len(whole)-4],
		"AStreamCutShortHalfway":    whole[:len(whole)/2],
		"NoStreamAtAll":             nil,
		"ContentThatIsNotGzip":      []byte(strings.Repeat("some content ", 1000)),
		"AStreamCutInsideItsHeader": whole[:5],
	}
	for name, stream := range rejected {
		t.Run("Rejects"+name, func(t *testing.T) {
			_, err := verify(t, stream)

			assert.Error(t, err)
		})
	}
}
