// Package highgobase holds distribution defaults and filesystem contracts for
// running the HighgoBase local stack.
package highgobase

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/go-errors/errors"
	"github.com/supabase/cli/pkg/config"
)

// Version is the upstream Supabase CLI release this distribution is based on.
const Version = "2.76.14"

const (
	// DbPort is the database port exposed to other containers and tools.
	DbPort = uint16(5432)
	// DbUser is the OS user that owns the HighgoBase data directory.
	DbUser = "highgo"
	// PgDataDir is where the hgdb image keeps its cluster files.
	PgDataDir = "/home/highgo/data"
	// InitPort is the port the hgdb entrypoint uses for temporary init servers
	// and for the psql client it hardcodes during first-boot scripts.
	InitPort = "5866"
	// SharedPreloadLibraries mirrors docker-compose.hgdb.yml.
	SharedPreloadLibraries = "pg_net, pg_stat_statements, gb18030_2022, restricted_dba, credcheck, hg_sysmac, sqlite_fdw, ora_synonym, liboracle_parser, ivorysql_ora, pg_object, timescaledb, pg_search, pgsodium, pg_cron, hsh, worker_pg_pdr"
)

// BuildEnabled is set to "true" by release builds of the highgobase binary.
var BuildEnabled string

// Enabled reports whether this process should behave as the HighgoBase CLI.
func Enabled() bool {
	if value := strings.TrimSpace(os.Getenv("HIGHGOBASE")); len(value) > 0 {
		if enabled, err := strconv.ParseBool(value); err == nil {
			return enabled
		}
		return true
	}
	if strings.EqualFold(strings.TrimSpace(BuildEnabled), "true") {
		return true
	}
	name := strings.ToLower(filepath.Base(os.Args[0]))
	name = strings.TrimSuffix(name, ".exe")
	return name == "highgobase"
}

// SystemSchemas are namespaces owned by the HGDB runtime rather than the project.
var SystemSchemas = []string{
	"ag_catalog",
	"credcheck",
	"graphql",
	"graphql_public",
	"hg_misc",
	"hg_security",
	"paradedb",
	"pdb",
	"pgagent",
	"sys",
}

// FilterUserSchemas removes schemas managed by the HGDB runtime from a schema list.
func FilterUserSchemas(schemas []string) []string {
	filtered := make([]string, 0, len(schemas))
	for _, name := range schemas {
		if IsSystemSchema(name) {
			continue
		}
		filtered = append(filtered, name)
	}
	return filtered
}

// IsSystemSchema reports whether a schema belongs to the HGDB runtime.
func IsSystemSchema(name string) bool {
	if strings.HasPrefix(name, "hg_") {
		return true
	}
	for _, system := range SystemSchemas {
		if name == system {
			return true
		}
	}
	return false
}

// CommandName is the root command name shown in help output.
func CommandName() string {
	if Enabled() {
		return "highgobase"
	}
	return "supabase"
}

// Home resolves the higobase checkout containing the Docker stack assets.
func Home() string {
	if value := strings.TrimSpace(os.Getenv("HIGHGOBASE_HOME")); len(value) > 0 {
		return absolute(value)
	}
	var candidates []string
	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(dir, "..", "higobase"),
			filepath.Join(dir, "..", "..", "higobase"),
		)
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "..", "higobase"),
			filepath.Join(wd, "..", "..", "higobase"),
		)
	}
	for _, candidate := range candidates {
		if isHome(candidate) {
			return absolute(candidate)
		}
	}
	return absolute(filepath.Join("..", "higobase"))
}

func absolute(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(path)
}

func isHome(path string) bool {
	_, err := os.Stat(filepath.Join(path, "docker", "docker-compose.hgdb.yml"))
	return err == nil
}

// DockerDir is the self-hosted stack directory inside the higobase checkout.
func DockerDir() string {
	return filepath.Join(Home(), "docker")
}

// InitScriptsDir is the HGDB initialisation scripts directory.
func InitScriptsDir() string {
	return filepath.Join(DockerDir(), "volumes", "db", "hgdb_init_scripts")
}

// DbAssetsDir is the host directory mounted read-only into the database container.
func DbAssetsDir() string {
	return filepath.Join(DockerDir(), "volumes", "db")
}

// EntrypointPath is the HighgoBase database entrypoint.
func EntrypointPath() string {
	return filepath.Join(DockerDir(), "volumes", "db", "docker-entrypoint.hgdb.sh")
}

// PgHbaPath is the HighgoBase host based authentication configuration.
func PgHbaPath() string {
	return filepath.Join(DockerDir(), "volumes", "db", "pg_hba.conf")
}

// PgsodiumGetkeyPath is the key provider mounted for the pgsodium extension.
func PgsodiumGetkeyPath() string {
	return filepath.Join(DockerDir(), "volumes", "db", "pgsodium_getkey.sh")
}

// LicenseDir returns the mounted HGDB license directory when it contains license.dat.
func LicenseDir() (string, bool) {
	dir := strings.TrimSpace(os.Getenv("HIGHGOBASE_LICENSE_DIR"))
	if len(dir) == 0 {
		dir = filepath.Join(DockerDir(), "volumes", "db", "license")
	}
	dir = absolute(dir)
	if _, err := os.Stat(filepath.Join(dir, "license.dat")); err != nil {
		return dir, false
	}
	return dir, true
}

// RequiredFiles lists the assets the local stack cannot start without.
func RequiredFiles() []string {
	return []string{
		filepath.Join(DockerDir(), "docker-compose.hgdb.yml"),
		EntrypointPath(),
		PgHbaPath(),
		PgsodiumGetkeyPath(),
		filepath.Join(InitScriptsDir(), "migrate.sh"),
		filepath.Join(InitScriptsDir(), "init-scripts", "00000000000000-initial-schema.sql"),
		filepath.Join(InitScriptsDir(), "migrations", "97-_supabase.sql"),
	}
}

// ValidateHome verifies that the higobase stack assets are available.
func ValidateHome() error {
	var missing []string
	for _, path := range RequiredFiles() {
		if _, err := os.Stat(path); err != nil {
			missing = append(missing, path)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return errors.Errorf("HighgoBase stack assets not found under %s.\nSet HIGHGOBASE_HOME to the higobase checkout that contains docker/docker-compose.hgdb.yml.\nMissing:\n  %s", Home(), strings.Join(missing, "\n  "))
}

func imageFromEnv(env, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(env)); len(value) > 0 {
		return value
	}
	return fallback
}

// ImageDB returns the configured HighgoBase database image.
func ImageDB() string {
	return imageFromEnv("HIGHGOBASE_DB_IMAGE", "hgdb:9.0.5.1-1-5837-31bd189-20260827-pg_partman-pgmq-refactor.linux.x86_64")
}

// LocalImages lists every image the local stack may start.
func LocalImages() []string {
	return []string{
		ImageDB(),
		imageFromEnv("HIGHGOBASE_KONG_IMAGE", "hgbase-kong:latest"),
		imageFromEnv("HIGHGOBASE_REST_IMAGE", "hgbase-rest:latest"),
		imageFromEnv("HIGHGOBASE_AUTH_IMAGE", "hgbase-auth:latest"),
		imageFromEnv("HIGHGOBASE_REALTIME_IMAGE", "realtime-dev.hgbase-realtime:latest"),
		imageFromEnv("HIGHGOBASE_STORAGE_IMAGE", "hgbase-storage:latest"),
		imageFromEnv("HIGHGOBASE_IMGPROXY_IMAGE", "hgbase-imgproxy:latest"),
		imageFromEnv("HIGHGOBASE_META_IMAGE", "hgbase-meta:latest"),
		imageFromEnv("HIGHGOBASE_STUDIO_IMAGE", "hgbase-studio:latest"),
		imageFromEnv("HIGHGOBASE_EDGE_FUNCTIONS_IMAGE", "hgbase-edge-functions:latest"),
		imageFromEnv("HIGHGOBASE_ANALYTICS_IMAGE", "hgbase-analytics:latest"),
		imageFromEnv("HIGHGOBASE_VECTOR_IMAGE", "hgbase-vector:latest"),
		imageFromEnv("HIGHGOBASE_POOLER_IMAGE", "hgbase-supavisor:latest"),
		imageFromEnv("HIGHGOBASE_MAILPIT_IMAGE", "axllent/mailpit:v1.22.3"),
	}
}

// ImageRequired reports whether an image is needed by the current local config.
func ImageRequired(image string, cfg config.Config) bool {
	if cfg == nil {
		return true
	}
	switch image {
	case ImageDB():
		return true
	case imageFromEnv("HIGHGOBASE_KONG_IMAGE", "hgbase-kong:latest"),
		imageFromEnv("HIGHGOBASE_REST_IMAGE", "hgbase-rest:latest"):
		return cfg.Api.Enabled
	case imageFromEnv("HIGHGOBASE_AUTH_IMAGE", "hgbase-auth:latest"):
		return cfg.Auth.Enabled
	case imageFromEnv("HIGHGOBASE_REALTIME_IMAGE", "realtime-dev.hgbase-realtime:latest"):
		return cfg.Realtime.Enabled
	case imageFromEnv("HIGHGOBASE_STORAGE_IMAGE", "hgbase-storage:latest"):
		return cfg.Storage.Enabled
	case imageFromEnv("HIGHGOBASE_IMGPROXY_IMAGE", "hgbase-imgproxy:latest"):
		return cfg.Storage.Enabled && cfg.Storage.ImageTransformation != nil && cfg.Storage.ImageTransformation.Enabled
	case imageFromEnv("HIGHGOBASE_META_IMAGE", "hgbase-meta:latest"),
		imageFromEnv("HIGHGOBASE_STUDIO_IMAGE", "hgbase-studio:latest"):
		return cfg.Studio.Enabled
	case imageFromEnv("HIGHGOBASE_EDGE_FUNCTIONS_IMAGE", "hgbase-edge-functions:latest"):
		return cfg.EdgeRuntime.Enabled
	case imageFromEnv("HIGHGOBASE_ANALYTICS_IMAGE", "hgbase-analytics:latest"),
		imageFromEnv("HIGHGOBASE_VECTOR_IMAGE", "hgbase-vector:latest"):
		return cfg.Analytics.Enabled
	case imageFromEnv("HIGHGOBASE_POOLER_IMAGE", "hgbase-supavisor:latest"):
		return cfg.Db.Pooler.Enabled
	case imageFromEnv("HIGHGOBASE_MAILPIT_IMAGE", "axllent/mailpit:v1.22.3"):
		return cfg.Inbucket.Enabled
	default:
		return true
	}
}

// Apply overrides a loaded CLI config with HighgoBase images and database defaults.
func Apply(config config.Config) {
	config.Db.MajorVersion = 14
	config.Db.Image = ImageDB()
	config.Api.KongImage = imageFromEnv("HIGHGOBASE_KONG_IMAGE", "hgbase-kong:latest")
	config.Api.Image = imageFromEnv("HIGHGOBASE_REST_IMAGE", "hgbase-rest:latest")
	config.Auth.Image = imageFromEnv("HIGHGOBASE_AUTH_IMAGE", "hgbase-auth:latest")
	config.Realtime.Image = imageFromEnv("HIGHGOBASE_REALTIME_IMAGE", "realtime-dev.hgbase-realtime:latest")
	config.Storage.Image = imageFromEnv("HIGHGOBASE_STORAGE_IMAGE", "hgbase-storage:latest")
	config.Storage.ImgProxyImage = imageFromEnv("HIGHGOBASE_IMGPROXY_IMAGE", "hgbase-imgproxy:latest")
	config.Studio.Image = imageFromEnv("HIGHGOBASE_STUDIO_IMAGE", "hgbase-studio:latest")
	config.Studio.PgmetaImage = imageFromEnv("HIGHGOBASE_META_IMAGE", "hgbase-meta:latest")
	config.EdgeRuntime.Image = imageFromEnv("HIGHGOBASE_EDGE_FUNCTIONS_IMAGE", "hgbase-edge-functions:latest")
	config.Analytics.Image = imageFromEnv("HIGHGOBASE_ANALYTICS_IMAGE", "hgbase-analytics:latest")
	config.Analytics.VectorImage = imageFromEnv("HIGHGOBASE_VECTOR_IMAGE", "hgbase-vector:latest")
	config.Db.Pooler.Image = imageFromEnv("HIGHGOBASE_POOLER_IMAGE", "hgbase-supavisor:latest")
	config.Inbucket.Image = imageFromEnv("HIGHGOBASE_MAILPIT_IMAGE", "axllent/mailpit:v1.22.3")
}

// ImageSuggestion is shown when a local HighgoBase image is missing.
func ImageSuggestion() string {
	return "Load the HighgoBase images referenced by " + filepath.Join(DockerDir(), "docker-compose.hgdb.yml") + " before starting the stack."
}
