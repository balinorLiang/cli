package highgobase

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/supabase/cli/pkg/config"
)

func TestEnabled(t *testing.T) {
	t.Run("environment override", func(t *testing.T) {
		t.Setenv("HIGHGOBASE", "1")
		assert.True(t, Enabled())
		t.Setenv("HIGHGOBASE", "false")
		assert.False(t, Enabled())
	})

	t.Run("build flag", func(t *testing.T) {
		t.Setenv("HIGHGOBASE", "")
		BuildEnabled = "true"
		t.Cleanup(func() { BuildEnabled = "" })
		assert.True(t, Enabled())
	})
}

func TestApply(t *testing.T) {
	cfg := config.NewConfig()
	Apply(&cfg)

	assert.Equal(t, uint(14), cfg.Db.MajorVersion)
	assert.Equal(t, "hgdb:9.0.5.1-1-5837-31bd189-20260827-pg_partman-pgmq-refactor.linux.x86_64", cfg.Db.Image)
	assert.Equal(t, "hgbase-auth:latest", cfg.Auth.Image)
	assert.Equal(t, "hgbase-studio:latest", cfg.Studio.Image)
	assert.Equal(t, "hgbase-supavisor:latest", cfg.Db.Pooler.Image)
}

func TestHome(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docker"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docker", "docker-compose.hgdb.yml"), []byte("name: hgbase\n"), 0o644))
	t.Setenv("HIGHGOBASE_HOME", root)

	assert.Equal(t, filepath.Clean(root), Home())
	assert.NoError(t, ValidateHomeForTest(root))
}

func ValidateHomeForTest(root string) error {
	for _, path := range RequiredFiles() {
		if filepath.Dir(path) == filepath.Dir(filepath.Join(root, "docker", "docker-compose.hgdb.yml")) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte("-- test\n"), 0o644); err != nil {
			return err
		}
	}
	return ValidateHome()
}

func TestValidateHome(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HIGHGOBASE_HOME", root)

	err := ValidateHome()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HIGHGOBASE_HOME")
}

func TestLocalImages(t *testing.T) {
	t.Setenv("HIGHGOBASE_DB_IMAGE", "registry.local/hgdb:test")
	images := LocalImages()
	assert.Contains(t, images, "registry.local/hgdb:test")
	assert.Contains(t, images, "hgbase-kong:latest")
}

func TestImageRequired(t *testing.T) {
	cfg := config.NewConfig()
	cfg.Api.Enabled = true
	cfg.Db.Pooler.Enabled = false
	cfg.Inbucket.Enabled = false
	cfg.Storage.ImageTransformation = nil

	assert.True(t, ImageRequired(ImageDB(), &cfg))
	assert.True(t, ImageRequired("hgbase-rest:latest", &cfg))
	assert.False(t, ImageRequired("hgbase-supavisor:latest", &cfg))
	assert.False(t, ImageRequired("axllent/mailpit:v1.22.3", &cfg))
	assert.False(t, ImageRequired("hgbase-imgproxy:latest", &cfg))

	cfg.Db.Pooler.Enabled = true
	assert.True(t, ImageRequired("hgbase-supavisor:latest", &cfg))
}

func TestFilterUserSchemas(t *testing.T) {
	schemas := []string{"public", "app", "hg_security", "sys", "hg_custom", "legal_kb"}

	assert.Equal(t, []string{"public", "app", "legal_kb"}, FilterUserSchemas(schemas))
}

func TestLicenseDir(t *testing.T) {
	root := t.TempDir()
	license := filepath.Join(root, "docker", "volumes", "db", "license")
	require.NoError(t, os.MkdirAll(license, 0o755))
	t.Setenv("HIGHGOBASE_HOME", root)

	_, ok := LicenseDir()
	assert.False(t, ok)

	require.NoError(t, os.WriteFile(filepath.Join(license, "client_info.dat"), []byte("client"), 0o644))
	_, ok = LicenseDir()
	assert.False(t, ok)

	require.NoError(t, os.WriteFile(filepath.Join(license, "license.dat"), []byte("license"), 0o644))
	dir, ok := LicenseDir()
	assert.True(t, ok)
	assert.Equal(t, filepath.Clean(license), dir)
}
