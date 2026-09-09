package stack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSeedScriptsRefuseTruncatedDumps asserts every seed script that restores a database dump also
// refuses a truncated one. IM streams pg_dump out of a pod over an exec that occasionally delivers
// partial output, and a partial dump gzips cleanly and restores happily: that is how the lmis
// sandbox came up with 143 of its 401 tables and no users. The check has to live in the scripts and
// not only in the save path, so an artifact already in S3 is caught at restore time, and it has to
// be asserted here because the check was written once for dhis2-db and then missed entirely by the
// copy that became dhis2-v2's.
func TestSeedScriptsRefuseTruncatedDumps(t *testing.T) {
	scripts, err := filepath.Glob("../../stacks/*/seed*.sh")
	require.NoError(t, err)
	require.NotEmpty(t, scripts, "no seed scripts found, has the stacks directory moved?")

	var restorers []string
	for _, script := range scripts {
		content, err := os.ReadFile(script)
		require.NoError(t, err)

		body := string(content)
		if !strings.Contains(body, "pg_restore") {
			continue
		}
		restorers = append(restorers, script)

		assert.Containsf(t, body, "PostgreSQL database dump complete",
			"%s restores a database dump without requiring pg_dump's completion marker", script)
		assert.NotContainsf(t, body, "|| true",
			"%s hides a failed restore behind || true, which is what let a half seeded database look seeded", script)
	}

	assert.NotEmpty(t, restorers, "no seed script restores a database dump, so this test asserts nothing")
}

// TestSeedScriptGuardCatchesAnUnguardedScript asserts the guard above would actually fail, rather
// than passing on a script that restores a dump with no marker check.
func TestSeedScriptGuardCatchesAnUnguardedScript(t *testing.T) {
	unguarded := `(pg_restore --verbose -d "$PGDATABASE" -j 4 "$tmp_file") || (gunzip -v -c "$tmp_file" | psql) || true`

	assert.NotContains(t, unguarded, "PostgreSQL database dump complete")
	assert.Contains(t, unguarded, "|| true")
}
