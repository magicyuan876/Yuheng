package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/migrations"
)

// isolateSources runs a test with an empty extension registry and restores the
// previous one afterwards, and resets the cached migration state the readiness
// endpoint reads.
func isolateSources(t *testing.T) {
	t.Helper()
	sourcesMu.Lock()
	saved := sources
	sources = nil
	sourcesMu.Unlock()
	t.Cleanup(func() {
		sourcesMu.Lock()
		sources = saved
		sourcesMu.Unlock()
		setMigrationState(0, false, "", false)
	})
}

func mapFS(files map[string]string) fs.FS {
	m := fstest.MapFS{}
	for name, body := range files {
		m[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return m
}

func mustPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s did not panic", name)
		}
	}()
	f()
}

func TestRegisterMigrationSourceRejectsWiringMistakes(t *testing.T) {
	isolateSources(t)
	fsys := mapFS(nil)
	RegisterMigrationSource("a", fsys, "a_migrations")

	mustPanic(t, "duplicate name", func() { RegisterMigrationSource("a", fsys, "other_migrations") })
	mustPanic(t, "duplicate table", func() { RegisterMigrationSource("b", fsys, "a_migrations") })
	mustPanic(t, "the core table", func() { RegisterMigrationSource("c", fsys, "schema_migrations") })
	mustPanic(t, "empty name", func() { RegisterMigrationSource("", fsys, "c_migrations") })
	mustPanic(t, "nil fs", func() { RegisterMigrationSource("d", nil, "d_migrations") })
	mustPanic(t, "unsafe table", func() { RegisterMigrationSource("e", fsys, "e; DROP TABLE tenants") })
}

func TestPoolConfigFromEnv(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "")
	t.Setenv("DB_MAX_IDLE_CONNS", "")
	t.Setenv("DB_CONN_MAX_LIFETIME_MINUTES", "")
	cfg, warn, err := PoolConfigFromEnv()
	if err != nil || warn != "" || cfg.MaxOpenConns != 50 || cfg.MaxIdleConns != 10 ||
		cfg.ConnMaxLifetime.Minutes() != 10 {
		t.Fatalf("defaults = %+v, %q, %v", cfg, warn, err)
	}

	t.Setenv("DB_MAX_OPEN_CONNS", "20")
	t.Setenv("DB_MAX_IDLE_CONNS", "30")
	cfg, warn, err = PoolConfigFromEnv()
	if err != nil || cfg.MaxIdleConns != 20 || warn == "" {
		t.Errorf("idle above open: %+v, %q, %v; want idle lowered to 20 with a warning", cfg, warn, err)
	}

	for _, bad := range []string{"0", "-3", "many", "1.5"} {
		t.Setenv("DB_MAX_OPEN_CONNS", bad)
		if _, _, err := PoolConfigFromEnv(); err == nil {
			t.Errorf("DB_MAX_OPEN_CONNS=%q was accepted", bad)
		}
	}
}

func versionIn(t *testing.T, db *sql.DB, table string) (version int, dirty bool) {
	t.Helper()
	if err := db.QueryRow(fmt.Sprintf(`SELECT version, dirty FROM %s`, table)).Scan(&version, &dirty); err != nil {
		t.Fatalf("read %s: %v", table, err)
	}
	return version, dirty
}

func TestExtensionSourcesRunAfterCoreWithTheirOwnTables(t *testing.T) {
	isolateSources(t)
	url, db := pgtest.NewEmpty(t)

	RegisterMigrationSource("alpha", mapFS(map[string]string{
		"000001_widgets.up.sql": `CREATE TABLE ext_widgets (id int PRIMARY KEY,
			tenant_id int REFERENCES tenants(id))`,
		"000001_widgets.down.sql": `DROP TABLE ext_widgets`,
		"000002_seed.up.sql":      `INSERT INTO ext_widgets (id) VALUES (1)`,
		"000002_seed.down.sql":    `DELETE FROM ext_widgets`,
	}), "alpha_schema_migrations")
	RegisterMigrationSource("beta", mapFS(map[string]string{
		"000001_gadgets.up.sql":   `CREATE TABLE ext_gadgets (id int PRIMARY KEY)`,
		"000001_gadgets.down.sql": `DROP TABLE ext_gadgets`,
	}), "beta_schema_migrations")

	if err := RunMigrationsWithOptions(url, MigrationOptions{}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	// alpha's first migration references a core table, so it ran after the core.
	if v, dirty := versionIn(t, db, "alpha_schema_migrations"); v != 2 || dirty {
		t.Errorf("alpha at %d dirty=%v; want 2 clean", v, dirty)
	}
	if v, dirty := versionIn(t, db, "beta_schema_migrations"); v != 1 || dirty {
		t.Errorf("beta at %d dirty=%v; want 1 clean", v, dirty)
	}
	coreVersion, coreDirty := versionIn(t, db, "schema_migrations")
	if coreDirty || coreVersion < 100 {
		t.Errorf("core at %d dirty=%v; want the head version, clean", coreVersion, coreDirty)
	}
	if ok, reason := MigrationReady(); !ok {
		t.Errorf("not ready after a clean run: %s", reason)
	}

	// A second run changes nothing: no repeated seed row, no version movement.
	if err := RunMigrationsWithOptions(url, MigrationOptions{}); err != nil {
		t.Fatalf("second run: %v", err)
	}
	var rows int
	if err := db.QueryRow(`SELECT count(*) FROM ext_widgets`).Scan(&rows); err != nil || rows != 1 {
		t.Errorf("ext_widgets has %d rows (%v); want 1", rows, err)
	}
	if v, _ := versionIn(t, db, "schema_migrations"); v != coreVersion {
		t.Errorf("core moved from %d to %d on a no-op run", coreVersion, v)
	}
}

func TestExtensionFailureIsFatalAndNamesTheSource(t *testing.T) {
	isolateSources(t)
	url, db := pgtest.NewEmpty(t)

	RegisterMigrationSource("good", mapFS(map[string]string{
		"000001_ok.up.sql": `CREATE TABLE ext_ok (id int)`,
	}), "good_schema_migrations")
	RegisterMigrationSource("broken", mapFS(map[string]string{
		"000001_bad.up.sql": `CREATE TABLE ext_bad (id int); SELECT * FROM no_such_table`,
	}), "broken_schema_migrations")
	RegisterMigrationSource("never", mapFS(map[string]string{
		"000001_after.up.sql": `CREATE TABLE ext_after (id int)`,
	}), "never_schema_migrations")

	err := RunMigrationsWithOptions(url, MigrationOptions{})
	if err == nil {
		t.Fatal("a failing extension migration did not fail the run")
	}
	if !strings.Contains(err.Error(), `"broken"`) {
		t.Errorf("error does not name the source: %v", err)
	}
	if ok, _ := MigrationReady(); ok {
		t.Error("readiness stayed true after an extension migration failed")
	}
	if CachedMigrationError() == "" {
		t.Error("the failure was not recorded for the system info endpoint")
	}

	// The core is intact and the source after the failure was never started.
	if _, dirty := versionIn(t, db, "schema_migrations"); dirty {
		t.Error("the core version table is dirty")
	}
	var exists bool
	_ = db.QueryRow(`SELECT to_regclass('ext_after') IS NOT NULL`).Scan(&exists)
	if exists {
		t.Error("a source registered after the failing one ran anyway")
	}

	// The failing source is dirty; without AUTO_RECOVER_DIRTY the next start
	// refuses with the manual-repair instructions.
	err = RunMigrationsWithOptions(url, MigrationOptions{})
	if err == nil || !strings.Contains(err.Error(), "dirty") || !strings.Contains(err.Error(), "migrate.sh force") {
		t.Errorf("second run error = %v; want the dirty-state instructions", err)
	}
	if strings.Contains(err.Error(), "enable AutoRecoverDirty") {
		t.Error("the error still carries the old misleading advice")
	}
}

// schemaSnapshot describes the public schema by names, types, nullability,
// defaults and index definitions, independent of column order (a column that is
// dropped and added back moves to the end).
func schemaSnapshot(t *testing.T, db *sql.DB) string {
	t.Helper()
	var b strings.Builder
	rows, err := db.Query(`SELECT table_name, column_name, data_type, is_nullable, coalesce(column_default, '')
		FROM information_schema.columns WHERE table_schema = 'public' AND table_name NOT LIKE '%schema_migrations'
		ORDER BY 1, 2`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var a, c, d, e, f string
		if err := rows.Scan(&a, &c, &d, &e, &f); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "col %s.%s %s null=%s default=%s\n", a, c, d, e, f)
	}
	_ = rows.Close()
	rows, err = db.Query(`SELECT indexdef FROM pg_indexes WHERE schemaname = 'public'
		AND tablename NOT LIKE '%schema_migrations' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "idx %s\n", d)
	}
	_ = rows.Close()
	return b.String()
}

// brokenDown lists the down migrations that cannot run. Each is broken in the
// file itself, not by the test; they are reported here rather than skipped
// silently, and fixing one means deleting its line.
//
// The rollback path is not exercised below 000089: that migration's down file
// recreates the agent tables it dropped as empty skeletons ("best-effort
// structural rollback"), so what the older down files then find is not what
// they were written against.
var brokenDown = map[uint]string{
	38: "its down recreates indexes on im_channel_sessions.deleted_at, a column 000089's skeleton of that table lacks",
	21: "drops im_channels while 000089's skeleton of im_channel_sessions still has a foreign key to it",
	1:  "uses UPDATE ... JOIN, which is MySQL syntax and a syntax error in PostgreSQL",
	0:  "drops tenants while users still references it (fk_users_tenant); a knock-on of skipping 000001's down",
}

// Every migration can be undone one step at a time, and undoing the ones above
// 000089 and redoing them restores the schema exactly.
//
// 000044's down refuses to drop audit_logs unless the session sets
// yuheng.allow_destructive_migration (a deliberate guard against a production
// rollback destroying history); this test sets it on its own connection.
func TestMigrationsRoundTripStepByStep(t *testing.T) {
	if testing.Short() {
		t.Skip("walks every migration down and up; slow")
	}
	const skeletonBoundary = 89 // see brokenDown

	url, db := pgtest.NewEmpty(t, "yuheng.allow_destructive_migration=true")
	src, err := iofs.New(migrations.Core(), ".")
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, url)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("up: %v", err)
	}
	head, _, _ := m.Version()
	atHead := schemaSnapshot(t, db)

	// The maintained range: down to 000089 and up again is lossless.
	for {
		v, _, _ := m.Version()
		if v <= skeletonBoundary {
			break
		}
		if err := m.Steps(-1); err != nil {
			t.Fatalf("down from version %d: %v", v, err)
		}
	}
	if err := m.Up(); err != nil {
		t.Fatalf("up from %d to head: %v", skeletonBoundary, err)
	}
	if v, dirty, _ := m.Version(); v != head || dirty {
		t.Fatalf("back at version %d dirty=%v; want %d clean", v, dirty, head)
	}
	if after := schemaSnapshot(t, db); after != atHead {
		t.Errorf("schema after down-to-%d and up differs from the original:\n%s",
			skeletonBoundary, firstDifference(atHead, after))
	}

	// The rest of the way down: every step must run except the known-broken ones.
	var unexpected []string
	steps := 0
	for {
		v, _, verr := m.Version()
		if errors.Is(verr, migrate.ErrNilVersion) {
			break
		}
		if verr != nil {
			t.Fatalf("version: %v", verr)
		}
		if reason, known := brokenDown[v]; known {
			t.Logf("not running down of %d: %s", v, reason)
			if err := m.Force(int(v) - 1); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := m.Steps(-1); err != nil {
			first, _, _ := strings.Cut(err.Error(), "\n")
			unexpected = append(unexpected, fmt.Sprintf("down from %d: %s", v, first))
			if err := m.Force(int(v) - 1); err != nil {
				t.Fatal(err)
			}
			continue
		}
		steps++
	}
	t.Logf("ran %d down migrations after the maintained range", steps)
	for _, u := range unexpected {
		t.Error(u)
	}
	_, _ = m.Close()

	// Whatever the broken steps left behind is not this test's business; start
	// over on an empty schema and check the whole chain still applies.
	if _, err := db.Exec(`DROP SCHEMA public CASCADE; CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	if err := RunMigrationsWithOptions(url, MigrationOptions{}); err != nil {
		t.Fatalf("up again on an emptied schema: %v", err)
	}
	if again := schemaSnapshot(t, db); again != atHead {
		t.Errorf("a second full up produced a different schema:\n%s", firstDifference(atHead, again))
	}
}

func firstDifference(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	seen := map[string]bool{}
	for _, l := range al {
		seen[l] = true
	}
	var diff []string
	for _, l := range bl {
		if !seen[l] {
			diff = append(diff, "+ "+l)
		}
	}
	seen = map[string]bool{}
	for _, l := range bl {
		seen[l] = true
	}
	for _, l := range al {
		if !seen[l] {
			diff = append(diff, "- "+l)
		}
	}
	if len(diff) > 10 {
		diff = append(diff[:10], "...")
	}
	return strings.Join(diff, "\n")
}

// An upgrade across destructive migrations keeps the data they are meant to keep.
func TestUpgradeFromOlderSchemaKeepsData(t *testing.T) {
	if testing.Short() {
		t.Skip("migrates a database in two stages; slow")
	}
	// 000121 drops knowledge_bases.cos_config after copying its provider;
	// 000123 replaces docs_pages.status with exclude_from_knowledge and drops
	// the column; 000134 binds every workspace, knowledge base and docs space
	// to a storage backend. Seed at 000120's schema (with 000121 onwards not
	// yet applied).
	const before = 120

	url, db := pgtest.NewEmpty(t)
	src, err := iofs.New(migrations.Core(), ".")
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, url)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Migrate(before); err != nil {
		t.Fatalf("migrate to %d: %v", before, err)
	}
	if _, err := m.Close(); err != nil {
		t.Fatal(err)
	}

	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustExec(`INSERT INTO tenants (id, name, business) VALUES (20001, 'acme', 'test')`)
	mustExec(`INSERT INTO knowledge_bases (id, name, tenant_id, embedding_model_id, summary_model_id, cos_config)
	          VALUES ('kb-1', 'Handbook', 20001, 'e', 's', '{"provider":"local","secret_id":"old-credential"}')`)
	mustExec(`INSERT INTO docs_spaces (id, tenant_id, slug, name, knowledge_base_id)
	          VALUES ('sp-1', 20001, 'eng', 'Engineering', 'kb-1')`)
	for _, p := range []struct{ id, short, title, status string }{
		{"pg-pub", "pub000000001", "Published page", "published"},
		{"pg-draft", "drf000000001", "Draft page", "draft"},
	} {
		mustExec(`INSERT INTO docs_pages (id, short_id, tenant_id, space_id, title, status, text_content)
		          VALUES ($1, $2, 20001, 'sp-1', $3, $4, $5)`, p.id, p.short, p.title, p.status, "body of "+p.title)
	}

	if err := RunMigrationsWithOptions(url, MigrationOptions{}); err != nil {
		t.Fatalf("upgrade to head: %v", err)
	}

	// Drafts are excluded from the knowledge base, published pages are not.
	rows, err := db.Query(`SELECT id, title, exclude_from_knowledge, text_content FROM docs_pages ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	type page struct {
		title, text string
		excluded    bool
	}
	got := map[string]page{}
	for rows.Next() {
		var id string
		var p page
		if err := rows.Scan(&id, &p.title, &p.excluded, &p.text); err != nil {
			t.Fatal(err)
		}
		got[id] = p
	}
	want := map[string]page{
		"pg-draft": {"Draft page", "body of Draft page", true},
		"pg-pub":   {"Published page", "body of Published page", false},
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("page %s = %+v; want %+v", id, got[id], w)
		}
	}
	var statusColumns int
	_ = db.QueryRow(`SELECT count(*) FROM information_schema.columns
		WHERE table_name='docs_pages' AND column_name='status'`).Scan(&statusColumns)
	if statusColumns != 0 {
		t.Error("docs_pages.status survived the upgrade")
	}

	// The knowledge base kept its row; the credential column 000121 dropped
	// and the provider-name columns 000135/000136 dropped are gone.
	var name string
	if err := db.QueryRow(`SELECT name FROM knowledge_bases WHERE id='kb-1'`).Scan(&name); err != nil {
		t.Fatalf("knowledge base row: %v", err)
	}
	if name != "Handbook" {
		t.Errorf("knowledge base = %q; want Handbook", name)
	}
	for _, column := range []struct{ table, name string }{
		{"knowledge_bases", "cos_config"},
		{"knowledge_bases", "storage_provider_config"},
		{"tenants", "storage_engine_config"},
	} {
		var n int
		_ = db.QueryRow(`SELECT count(*) FROM information_schema.columns
			WHERE table_name=$1 AND column_name=$2`, column.table, column.name).Scan(&n)
		if n != 0 {
			t.Errorf("%s.%s survived the upgrade", column.table, column.name)
		}
	}

	// 000134 makes storage bindings required: the workspace, its knowledge
	// base and its docs space, all unbound before, land on the deployment
	// backend.
	var tenantDefault, kbBackend, spaceBackend string
	if err := db.QueryRow(`SELECT t.default_storage_backend_id, k.storage_backend_id, s.storage_backend_id
		FROM tenants t, knowledge_bases k, docs_spaces s
		WHERE t.id = 20001 AND k.id = 'kb-1' AND s.id = 'sp-1'`).
		Scan(&tenantDefault, &kbBackend, &spaceBackend); err != nil {
		t.Fatalf("storage bindings: %v", err)
	}
	if tenantDefault != "env" || kbBackend != "env" || spaceBackend != "env" {
		t.Errorf("bindings = %q/%q/%q; want env everywhere", tenantDefault, kbBackend, spaceBackend)
	}
}
