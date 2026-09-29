package database

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/migrations"
)

var (
	migrationStateMu        sync.RWMutex
	currentMigrationVersion uint
	currentMigrationDirty   bool
	migrationVersionSet     bool
	currentMigrationError   string
)

// CachedMigrationVersion returns the migration version captured at startup.
// Returns (version, dirty, ok). ok is false if the version was never captured.
//
// Note: when migrations fail mid-way, the cache may still be populated via a
// best-effort m.Version() call inside RunMigrationsWithOptions so the system
// info endpoint can surface the partial state. Check CachedMigrationError() to
// distinguish a clean version reading from a recorded-after-failure one.
func CachedMigrationVersion() (uint, bool, bool) {
	migrationStateMu.RLock()
	defer migrationStateMu.RUnlock()
	return currentMigrationVersion, currentMigrationDirty, migrationVersionSet
}

// CachedMigrationError returns the error message captured when the most recent
// migration attempt failed at startup. Empty string means migrations either
// succeeded or were never run.
func CachedMigrationError() string {
	migrationStateMu.RLock()
	defer migrationStateMu.RUnlock()
	return currentMigrationError
}

// MigrationReady reports whether the schema state allows serving traffic: the
// last migration run did not fail and left no dirty version behind. A process
// that never ran migrations (AUTO_MIGRATE=false, migrations applied by an
// operator) has nothing recorded and is reported ready; the readiness endpoint
// cannot vouch for a schema it never looked at. reason is empty when ok.
func MigrationReady() (ok bool, reason string) {
	migrationStateMu.RLock()
	defer migrationStateMu.RUnlock()
	if currentMigrationError != "" {
		return false, "migration failed"
	}
	if migrationVersionSet && currentMigrationDirty {
		return false, fmt.Sprintf("schema is dirty at version %d", currentMigrationVersion)
	}
	return true, ""
}

// setMigrationState records the latest known migration state. Unlike the old
// sync.Once-based setter, this is intentionally idempotent-overwrite so the
// failure path (which runs after Up() errored) can replace the pre-migration
// snapshot taken from the initial m.Version() call.
func setMigrationState(version uint, dirty bool, errMsg string, versionKnown bool) {
	migrationStateMu.Lock()
	defer migrationStateMu.Unlock()
	if versionKnown {
		currentMigrationVersion = version
		currentMigrationDirty = dirty
		migrationVersionSet = true
	}
	currentMigrationError = errMsg
}

// captureMigrationFailure best-effort queries m for the current version so the
// system info endpoint can show "N (failed)" instead of vanishing the row, and
// stores the human-readable error message. Always returns the original error.
func captureMigrationFailure(m *migrate.Migrate, err error) error {
	versionKnown := false
	var ver uint
	var dirty bool
	if m != nil {
		v, d, vErr := m.Version()
		if vErr == nil {
			versionKnown = true
			ver, dirty = v, d
		}
	}
	setMigrationState(ver, dirty, err.Error(), versionKnown)
	return err
}

// RunMigrations executes all pending database migrations
// This should be called during application startup
func RunMigrations(dsn string) error {
	return RunMigrationsWithOptions(dsn, MigrationOptions{AutoRecoverDirty: false})
}

// MigrationOptions configures migration behavior
type MigrationOptions struct {
	// AutoRecoverDirty, when true, clears a dirty flag by forcing the version
	// table back one step and running the interrupted migration again.
	//
	// That is only safe when the interrupted migration can run twice: every
	// statement guarded (IF NOT EXISTS, IF EXISTS, ON CONFLICT DO NOTHING) or
	// wrapped so a rerun changes nothing. golang-migrate does not run a
	// migration in a transaction by itself, so a migration that died halfway
	// has applied some of its statements and not others; replaying an
	// unguarded ALTER TABLE ... ADD COLUMN, or a data rewrite that is not
	// idempotent, either fails again (leaving the database dirty once more) or
	// silently applies twice. Many migrations in this repository are not
	// guarded that way. It therefore defaults to off everywhere; leave it off
	// unless you have read the failed migration and know a rerun is harmless.
	AutoRecoverDirty bool
}

// migrationSource is a set of migrations with a version table of its own.
type migrationSource struct {
	name  string
	fsys  fs.FS
	table string
}

// coreMigrationsTable is the version table of the core migrations, golang-migrate's default.
const coreMigrationsTable = "schema_migrations"

var (
	sourcesMu sync.Mutex
	sources   []migrationSource

	// A table name is spliced into the DSN and then into SQL by the driver, so
	// it is restricted to a plain lower-case identifier.
	tableNameRe = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)
)

// RegisterMigrationSource adds a set of migrations that runs after the core
// ones at every start-up, with its own version table so its numbering is
// independent of the core's. It is the seam an extension uses to ship schema
// without editing migrations/versioned.
//
// fsys holds NNNNNN_name.up.sql / .down.sql files at its root. name identifies
// the source in logs and error messages. table is its version table (for
// example "acme_schema_migrations"); it must differ from the core's
// "schema_migrations" and from every other source's.
//
// Like extension.RegisterHook it is meant to be called from an init function or
// early in main, before the container is built, and it panics on a duplicate
// name, a duplicate table or an unusable argument: a wiring mistake should stop
// the process at start-up, not corrupt a version table later.
func RegisterMigrationSource(name string, fsys fs.FS, table string) {
	if name == "" {
		panic("database: RegisterMigrationSource with an empty name")
	}
	if fsys == nil {
		panic(fmt.Sprintf("database: RegisterMigrationSource(%q) with a nil file system", name))
	}
	if !tableNameRe.MatchString(table) {
		panic(fmt.Sprintf("database: RegisterMigrationSource(%q): table %q must match %s", name, table, tableNameRe))
	}
	if table == coreMigrationsTable {
		panic(fmt.Sprintf("database: RegisterMigrationSource(%q): table %q is the core migrations' own", name, table))
	}

	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	for _, s := range sources {
		if s.name == name {
			panic(fmt.Sprintf("database: migration source %q registered twice", name))
		}
		if s.table == table {
			panic(fmt.Sprintf("database: migration sources %q and %q share the version table %q", s.name, name, table))
		}
	}
	sources = append(sources, migrationSource{name: name, fsys: fsys, table: table})
}

// UnregisterMigrationSource removes a source added by RegisterMigrationSource.
// The registry is process-wide, so a test that registers a deliberately broken
// source must remove it again, or every later test in the binary that starts
// the server fails on it. Production code has no use for this: sources are
// registered once, at start-up.
func UnregisterMigrationSource(name string) {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	sources = slices.DeleteFunc(sources, func(s migrationSource) bool { return s.name == name })
}

func registeredSources() []migrationSource {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	return append([]migrationSource(nil), sources...)
}

// withMigrationsTable points a golang-migrate URL at a version table. The
// postgres driver strips x-migrations-table before it connects. The name has
// been validated as a plain identifier, so it needs no escaping.
func withMigrationsTable(dsn, table string) string {
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "x-migrations-table=" + table
}

// RunMigrationsWithOptions applies the core migrations, then every registered
// source in registration order. The first failure stops the run and is
// returned; later sources are not attempted, because an extension's schema may
// depend on the core's.
func RunMigrationsWithOptions(dsn string, opts MigrationOptions) error {
	ctx := context.Background()

	logger.Infof(ctx, "Starting database migration...")

	src, err := iofs.New(migrations.Core(), ".")
	if err != nil {
		wrapped := fmt.Errorf("failed to open the embedded core migrations: %w", err)
		setMigrationState(0, false, wrapped.Error(), false)
		return wrapped
	}
	if err := runSource(ctx, "core", src, dsn, opts, true); err != nil {
		return err
	}

	for _, s := range registeredSources() {
		xsrc, err := iofs.New(s.fsys, ".")
		if err != nil {
			wrapped := fmt.Errorf("migration source %q: failed to open its migrations: %w", s.name, err)
			setMigrationState(0, false, wrapped.Error(), false)
			return wrapped
		}
		if err := runSource(ctx, s.name, xsrc, withMigrationsTable(dsn, s.table), opts, false); err != nil {
			return err
		}
	}
	return nil
}

// runSource applies one source. core selects whether the version it ends on is
// the one reported by CachedMigrationVersion (the core's), or only its failure
// is recorded (an extension's).
func runSource(
	ctx context.Context, label string, src source.Driver, dsn string, opts MigrationOptions, core bool,
) error {
	// fail records a failure and prefixes an extension's error with its name so
	// the operator knows which version table the numbers below belong to.
	fail := func(m *migrate.Migrate, err error) error {
		if !core {
			err = fmt.Errorf("migration source %q: %w", label, err)
			setMigrationState(0, false, err.Error(), false)
			return err
		}
		return captureMigrationFailure(m, err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		logger.Errorf(ctx, "Failed to create migrate instance (%s): %v", label, err)
		wrapped := fmt.Errorf("failed to create migrate instance: %w", err)
		if !core {
			wrapped = fmt.Errorf("migration source %q: %w", label, wrapped)
		}
		setMigrationState(0, false, wrapped.Error(), false)
		return wrapped
	}
	defer m.Close()

	// Check current version and dirty state before migration
	oldVersion, oldDirty, versionErr := m.Version()
	if versionErr != nil && versionErr != migrate.ErrNilVersion {
		logger.Errorf(ctx, "Failed to get migration version (%s): %v", label, versionErr)
		return fail(m, fmt.Errorf("failed to get migration version: %w", versionErr))
	}

	if versionErr == migrate.ErrNilVersion {
		logger.Infof(ctx, "[%s] Database has no migration history, will start from version 0", label)
	} else {
		logger.Infof(ctx, "[%s] Current migration version: %d, dirty: %v", label, oldVersion, oldDirty)
	}

	// If database is in dirty state, try to recover or return error
	if oldDirty {
		logger.Warnf(ctx, "[%s] Database is in dirty state at version %d", label, oldVersion)
		if !opts.AutoRecoverDirty {
			return fail(m, dirtyError(label, oldVersion, false))
		}
		logger.Infof(ctx, "[%s] AUTO_RECOVER_DIRTY is enabled, attempting recovery...", label)
		if err := recoverFromDirtyState(ctx, m, oldVersion); err != nil {
			return fail(m, err)
		}
		// Update oldVersion after recovery
		oldVersion, _, _ = m.Version()
	}

	// Run all pending migrations
	logger.Infof(ctx, "[%s] Running pending migrations...", label)
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Errorf(ctx, "[%s] Migration failed: %v", label, err)
		// Check if error is due to dirty state (in case it became dirty during migration)
		currentVersion, currentDirty, versionCheckErr := m.Version()
		if versionCheckErr != nil || !currentDirty {
			return fail(m, fmt.Errorf("failed to run migrations: %w", err))
		}
		logger.Warnf(ctx, "[%s] Migration caused dirty state at version %d", label, currentVersion)
		if !opts.AutoRecoverDirty {
			return fail(m, fmt.Errorf("%w\n%w", err, dirtyError(label, currentVersion, true)))
		}
		logger.Infof(ctx, "[%s] Attempting to recover from dirty state...", label)
		if recoverErr := recoverFromDirtyState(ctx, m, currentVersion); recoverErr != nil {
			return fail(m, recoverErr)
		}
		logger.Infof(ctx, "[%s] Retrying migration after recovery...", label)
		if retryErr := m.Up(); retryErr != nil && retryErr != migrate.ErrNoChange {
			logger.Errorf(ctx, "[%s] Migration failed after recovery attempt: %v", label, retryErr)
			return fail(m, fmt.Errorf("migration failed after recovery attempt: %w", retryErr))
		}
	}

	// Get current version after migration
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fail(m, fmt.Errorf("failed to get migration version: %w", err))
	}

	if core {
		setMigrationState(version, dirty, "", true)
	}

	if oldVersion != version {
		logger.Infof(ctx, "[%s] Database migrated from version %d to %d", label, oldVersion, version)
	} else {
		logger.Infof(ctx, "[%s] Database is up to date (version: %d)", label, version)
	}

	if dirty {
		logger.Warnf(ctx, "[%s] Database is in dirty state! Manual intervention may be required.", label)
	}

	return nil
}

// dirtyError explains a dirty version table and how to repair it by hand.
// failedNow says the migration has only just failed in this run, as opposed to
// having been left dirty by an earlier one.
func dirtyError(label string, version uint, failedNow bool) error {
	forceVersion := int(version) - 1
	if forceVersion < 0 {
		forceVersion = 0
	}
	what := "a previous migration run was interrupted or failed part-way"
	if failedNow {
		what = "the migration above failed part-way"
	}
	scope := ""
	if label != "core" {
		scope = fmt.Sprintf(" (extension source %q; use its own migrations and version table)", label)
	}
	return fmt.Errorf(
		"database is in a dirty state at version %d%s: %s, so the schema may be half-changed and Yuheng will "+
			"not guess how to continue. To repair it by hand:\n"+
			"1. Read the error of the failing migration in the log above and look at "+
			"migrations/versioned/%06d_*.up.sql: which of its statements were applied?\n"+
			"2. Undo or finish those statements yourself (or restore the backup you took before upgrading).\n"+
			"3. Set the version table to the last migration that fully applied (usually %d):\n"+
			"   ./scripts/migrate.sh force %d   (or: make migrate-force version=%d)\n"+
			"4. Restart the application so the migration is retried.\n"+
			"AUTO_RECOVER_DIRTY=true would make the server do step 3 and retry by itself, but that is only safe "+
			"when the failed migration can run twice without harm; it is off by default for that reason.\n"+
			"See docs/migration-troubleshooting.md",
		version, scope, what, version, forceVersion, forceVersion, forceVersion,
	)
}

// recoverFromDirtyState attempts to recover from a dirty migration state
// by forcing to the previous version and allowing the migration to be retried
func recoverFromDirtyState(ctx context.Context, m *migrate.Migrate, dirtyVersion uint) error {
	// Special case: if dirty at version 0 (init migration), we cannot go back further
	// The only option is to force to version 0 and retry, but this requires the migration to be idempotent
	if dirtyVersion == 0 {
		logger.Warnf(ctx, "Database is in dirty state at version 0 (init migration). "+
			"This is the initial migration, cannot rollback further. "+
			"Will attempt to clear dirty flag and retry. "+
			"Note: This only works if the init migration uses IF NOT EXISTS clauses.")

		// Force to version -1 (no version) to allow re-running version 0
		// This effectively tells migrate that no migrations have been applied
		if err := m.Force(-1); err != nil {
			return fmt.Errorf(
				"failed to recover from dirty state at version 0. "+
					"Manual intervention required:\n"+
					"1. Check what was partially created in the database\n"+
					"2. Either drop all created objects and retry, or\n"+
					"3. Manually complete the migration and run: ./scripts/migrate.sh force 0\n"+
					"Error: %w", err)
		}

		logger.Infof(ctx, "Cleared migration state, will retry from version 0")
		return nil
	}

	forceVersion := int(dirtyVersion) - 1

	logger.Warnf(ctx, "Database is in dirty state at version %d, attempting auto-recovery by forcing to version %d",
		dirtyVersion, forceVersion)

	// Force to previous version to clear dirty state
	if err := m.Force(forceVersion); err != nil {
		return fmt.Errorf("failed to force migration version during recovery: %w", err)
	}

	logger.Infof(ctx, "Successfully forced migration to version %d, migration will be retried", forceVersion)
	return nil
}

// GetMigrationVersion returns the current migration version
func GetMigrationVersion() (uint, bool, error) {
	dbURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	src, err := iofs.New(migrations.Core(), ".")
	if err != nil {
		return 0, false, fmt.Errorf("failed to open the embedded core migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	version, dirty, err := m.Version()
	if err != nil {
		return 0, false, err
	}

	return version, dirty, nil
}
