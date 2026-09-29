package container

import (
	"io/fs"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/database"
	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
)

// pointEnvAt makes initDatabase connect to the throwaway database behind url.
func pointEnvAt(t *testing.T, migrateURL string) {
	t.Helper()
	u, err := url.Parse(migrateURL)
	if err != nil {
		t.Fatal(err)
	}
	pw, _ := u.User.Password()
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DB_HOST", u.Hostname())
	t.Setenv("DB_PORT", u.Port())
	t.Setenv("DB_USER", u.User.Username())
	t.Setenv("DB_PASSWORD", pw)
	t.Setenv("DB_NAME", strings.TrimPrefix(u.Path, "/"))
	t.Setenv("RETRIEVE_DRIVER", "")
	t.Setenv("AUTO_MIGRATE", "")
	t.Setenv("AUTO_RECOVER_DIRTY", "")
}

// A failed migration stops start-up unless the operator opts out, and the
// opt-out keeps the failure on record for /ready.
func TestInitDatabaseMigrationFailureIsFatalByDefault(t *testing.T) {
	migrateURL, _ := pgtest.NewEmpty(t)
	pointEnvAt(t, migrateURL)

	var broken fs.FS = fstest.MapFS{
		"000001_bad.up.sql": &fstest.MapFile{Data: []byte(`SELECT * FROM no_such_table`)},
	}
	database.RegisterMigrationSource("startup-test", broken, "startup_test_migrations")
	// The registry is process-wide: left behind, this source would make every
	// later test that starts the server fail on its broken migration.
	t.Cleanup(func() { database.UnregisterMigrationSource("startup-test") })

	t.Setenv("MIGRATION_FAIL_FAST", "")
	db, err := initDatabase(&config.Config{})
	if err == nil {
		t.Fatalf("initDatabase started against a failed migration (db=%v)", db)
	}
	wants := []string{"refusing to start", "migrate.sh", "migration-troubleshooting.md", "MIGRATION_FAIL_FAST=false"}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%v", want, err)
		}
	}

	t.Setenv("MIGRATION_FAIL_FAST", "false")
	db, err = initDatabase(&config.Config{})
	if err != nil {
		t.Fatalf("MIGRATION_FAIL_FAST=false should let the server start: %v", err)
	}
	if sqlDB, e := db.DB(); e == nil {
		_ = sqlDB.Close()
	}
	if ok, _ := database.MigrationReady(); ok {
		t.Error("/ready would report ready although the migration failed")
	}
}
