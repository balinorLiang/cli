package start

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/supabase/cli/internal/highgobase"
)

func TestHighgoBaseBinds(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HIGHGOBASE", "1")
	t.Setenv("HIGHGOBASE_HOME", root)
	license := filepath.Join(root, "docker", "volumes", "db", "license")
	require.NoError(t, os.MkdirAll(license, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(license, "license.dat"), []byte("license"), 0o644))

	assert.Equal(t, []string{
		highgobase.DbAssetsDir() + ":/hgbase:ro",
		license + ":/home/highgo/license",
	}, HighgoBaseBinds())
}
