// Package pgtest gives tests a real PostgreSQL database with the production
// schema.
//
// Tests used to open an in-memory SQLite database instead. That tested SQL
// the product never runs: repositories carried a SQLite branch beside every
// Postgres-specific query, and the tests exercised that branch, so the JSONB
// queries, NOW() and sequence-backed columns that production depends on went
// untested; the docs module kept a hand-written SQLite copy of its schema
// that nothing checked against the real migration; and the two databases
// disagree in ways that make tests wrong rather than merely different —
// SQLite compares timestamps as text, so tests failed on any machine whose
// clock was not on UTC. Postgres is the only database Yuheng runs on, so it
// is the only one it is tested on.
//
// Each test package starts one ParadeDB container — the image
// docker-compose.yml runs — and applies migrations/versioned to a template
// database with the same golang-migrate runner the server uses. Each call to
// New copies that template into a database of its own (CREATE DATABASE …
// TEMPLATE takes milliseconds, where re-running the migrations takes
// seconds), so tests never see each other's rows and may run in parallel.
//
// Environment:
//
//	YUHENG_TEST_POSTGRES_URL    use this server instead of starting a
//	                            container; a superuser URL, e.g.
//	                            postgres://postgres:secret@localhost:5432/postgres
//	YUHENG_TEST_POSTGRES_IMAGE  the image to start (default: the one
//	                            docker-compose.yml pins), for a registry mirror
package pgtest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // migrate's postgres:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql "pgx" driver
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/magicyuan876/yuheng/migrations"
)

// DefaultImage is the image docker-compose.yml runs the database on. Keep the
// two in step: a test that passes on another version proves little.
const DefaultImage = "paradedb/paradedb:v0.22.2-pg17"

// server is the Postgres instance a test binary shares, and the template
// database migrated on it.
type server struct {
	admin    *url.URL // superuser connection to the maintenance database
	template string   // migrated database every test database is copied from
}

var (
	setupOnce sync.Once
	shared    *server
	setupErr  error

	// CREATE DATABASE … TEMPLATE fails if another session is connected to
	// the template, and a concurrent copy counts as one. Copies within a
	// test binary are therefore made one at a time; they are fast.
	createMu sync.Mutex
	dbSeq    atomic.Int64
)

// New returns a connection to a fresh database holding the full production
// schema and no rows. The database is dropped when the test ends.
func New(t testing.TB) *gorm.DB {
	t.Helper()
	db, _ := open(t)
	return db
}

// NewURL is New for tests that start something which opens its own connection,
// such as the server's dependency graph: it also returns the connection URL
// (postgres://user:password@host:port/database) of the fresh database.
func NewURL(t testing.TB) (*gorm.DB, string) {
	t.Helper()
	db, name := open(t)
	return db, shared.dsn(name)
}

// NewSQL is New for code that takes a *sql.DB. The *gorm.DB over the same
// connection pool is returned too, for seeding rows.
func NewSQL(t testing.TB) (*sql.DB, *gorm.DB) {
	t.Helper()
	db, name := open(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("pgtest: database %s: %v", name, err)
	}
	return sqlDB, db
}

// NewEmpty returns the golang-migrate URL of a fresh database with no schema
// at all, and a *sql.DB on it, for tests of the migration runner itself. The
// URL carries app.skip_embedding=false, as the server's own does when the
// built-in engine is in use. settings are further session settings for the
// migrator's connections, each "name=value" (for example a GUC a migration
// checks before it does something destructive). The database is dropped when
// the test ends.
func NewEmpty(t testing.TB, settings ...string) (migrateURL string, db *sql.DB) {
	t.Helper()
	setupOnce.Do(func() { shared, setupErr = start() })
	if setupErr != nil {
		t.Fatalf("pgtest: no test database: %v", setupErr)
	}
	name := fmt.Sprintf("e_%d_%d", os.Getpid(), dbSeq.Add(1))
	if err := shared.exec(fmt.Sprintf(`CREATE DATABASE %q TEMPLATE template0`, name)); err != nil {
		t.Fatalf("pgtest: create database: %v", err)
	}
	target := *shared.admin
	target.Path = "/" + name
	q := target.Query()
	q.Set("sslmode", "disable")
	options := "-c app.skip_embedding=false"
	for _, setting := range settings {
		options += " -c " + setting
	}
	q.Set("options", options)
	target.RawQuery = q.Encode()

	db, err := sql.Open("pgx", shared.dsn(name))
	if err != nil {
		t.Fatalf("pgtest: open database %s: %v", name, err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_ = shared.execUnlocked(fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
	})
	return target.String(), db
}

func open(t testing.TB) (*gorm.DB, string) {
	t.Helper()
	setupOnce.Do(func() { shared, setupErr = start() })
	if setupErr != nil {
		t.Fatalf("pgtest: no test database: %v\n\n"+
			"The tests need Docker (to start %s) or a server named by "+
			"YUHENG_TEST_POSTGRES_URL.", setupErr, image())
	}

	name := fmt.Sprintf("t_%d_%d", os.Getpid(), dbSeq.Add(1))
	if err := shared.exec(fmt.Sprintf(`CREATE DATABASE %q TEMPLATE %q`, name, shared.template)); err != nil {
		t.Fatalf("pgtest: create database: %v", err)
	}

	db, err := gorm.Open(postgres.Open(shared.dsn(name)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("pgtest: open database %s: %v", name, err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		// WITH (FORCE) ends any connection a test leaked, which would
		// otherwise make the drop fail and leave the database behind.
		_ = shared.execUnlocked(fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
	})
	return db, name
}

// start finds or starts the server and migrates the template on it.
func start() (*server, error) {
	ctx := context.Background()

	admin := os.Getenv("YUHENG_TEST_POSTGRES_URL")
	if admin == "" {
		var err error
		if admin, err = startContainer(ctx); err != nil {
			return nil, err
		}
	}
	u, err := url.Parse(admin)
	if err != nil {
		return nil, fmt.Errorf("parse server URL: %w", err)
	}
	s := &server{admin: u, template: fmt.Sprintf("tpl_%d", os.Getpid())}

	if err := s.execUnlocked(fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, s.template)); err != nil {
		return nil, err
	}
	if err := s.execUnlocked(fmt.Sprintf(`CREATE DATABASE %q`, s.template)); err != nil {
		return nil, err
	}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func startContainer(ctx context.Context) (string, error) {
	c, err := tcpostgres.Run(ctx, image(),
		tcpostgres.WithDatabase("postgres"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(wait.ForAll(
			// The entrypoint starts the server twice — once to run its init
			// scripts, then for real — so wait for the second "ready".
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
			// And for Docker to publish the port. With many test packages
			// starting containers at once, the daemon can report the
			// container ready before its port mapping is visible, and the
			// connection string then fails with `port "5432/tcp" not found`.
			wait.ForListeningPort("5432/tcp"),
		).WithDeadline(2*time.Minute)),
	)
	if err != nil {
		return "", fmt.Errorf("start %s: %w", image(), err)
	}
	// The container is not terminated here: testcontainers' reaper removes
	// it when the test binary exits, after every test has used it.
	return c.ConnectionString(ctx, "sslmode=disable")
}

// migrate applies migrations/versioned to the template the way the server
// does at startup (internal/database.RunMigrations), with embeddings on so
// the vector tables exist for the retriever tests.
func (s *server) migrate() error {
	src, err := iofs.New(migrations.Core(), ".")
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	target := *s.admin
	target.Path = "/" + s.template
	q := target.Query()
	q.Set("sslmode", "disable")
	q.Set("options", "-c app.skip_embedding=false")
	target.RawQuery = q.Encode()

	m, err := migrate.NewWithSourceInstance("iofs", src, target.String())
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	upErr := m.Up()
	srcErr, dbErr := m.Close()
	if upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", upErr)
	}
	if srcErr != nil {
		return srcErr
	}
	// Close the migrator's connection before any copy is made: a template
	// with a session on it cannot be copied.
	return dbErr
}

// dsn is a GORM connection string for one database on the server, on UTC as
// the server's own connection is (internal/container).
func (s *server) dsn(database string) string {
	u := *s.admin
	u.Path = "/" + database
	q := u.Query()
	q.Set("sslmode", "disable")
	q.Set("TimeZone", "UTC")
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *server) exec(stmt string) error {
	createMu.Lock()
	defer createMu.Unlock()
	return s.execUnlocked(stmt)
}

func (s *server) execUnlocked(stmt string) error {
	db, err := sql.Open("pgx", s.dsn(strings.TrimPrefix(s.admin.Path, "/")))
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(stmt); err != nil {
		return fmt.Errorf("%s: %w", stmt, err)
	}
	return nil
}

func image() string {
	if img := os.Getenv("YUHENG_TEST_POSTGRES_IMAGE"); img != "" {
		return img
	}
	return DefaultImage
}
