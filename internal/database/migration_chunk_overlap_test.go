package database

import (
	"database/sql"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/migrations"
)

// 000129 removes stored chunk overlaps of 0, which were always split with the
// default, so that 0 can mean "no overlap" from now on without changing how
// any existing base or document splits. Overlaps that were set to something
// else stay as they are.
func TestChunkOverlapZeroBecomesUnset(t *testing.T) {
	if testing.Short() {
		t.Skip("migrates a database in two stages; slow")
	}
	const before = 128

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
	for id, cfg := range map[string]string{
		"kb-zero": `{"chunk_size": 512, "chunk_overlap": 0}`,
		"kb-set":  `{"chunk_size": 512, "chunk_overlap": 50}`,
	} {
		mustExec(`INSERT INTO knowledge_bases
		              (id, name, tenant_id, embedding_model_id, summary_model_id, chunking_config)
		          VALUES ($1, $1, 20001, 'e', 's', $2)`, id, cfg)
	}
	for id, meta := range map[string]string{
		"k-zero": `{"process_overrides": {"chunking_config": {"chunk_size": 1024, "chunk_overlap": 0}}}`,
		"k-set":  `{"process_overrides": {"chunking_config": {"chunk_size": 1024, "chunk_overlap": 120}}}`,
	} {
		mustExec(`INSERT INTO knowledges (id, tenant_id, knowledge_base_id, type, title, source, parse_status, metadata)
		          VALUES ($1, 20001, 'kb-set', 'file', $1, 'upload', 'completed', $2)`, id, meta)
	}

	if err := RunMigrationsWithOptions(url, MigrationOptions{}); err != nil {
		t.Fatalf("upgrade to head: %v", err)
	}

	overlap := func(q, id string) sql.NullString {
		t.Helper()
		var v sql.NullString
		if err := db.QueryRow(q, id).Scan(&v); err != nil {
			t.Fatalf("%s [%s]: %v", q, id, err)
		}
		return v
	}
	const kbQ = `SELECT chunking_config->>'chunk_overlap' FROM knowledge_bases WHERE id = $1`
	const kQ = `SELECT metadata #>> '{process_overrides,chunking_config,chunk_overlap}' FROM knowledges WHERE id = $1`
	if v := overlap(kbQ, "kb-zero"); v.Valid {
		t.Errorf("kb-zero overlap = %q; want it unset", v.String)
	}
	if v := overlap(kbQ, "kb-set"); v.String != "50" {
		t.Errorf("kb-set overlap = %q; want 50", v.String)
	}
	if v := overlap(kQ, "k-zero"); v.Valid {
		t.Errorf("k-zero override overlap = %q; want it unset", v.String)
	}
	if v := overlap(kQ, "k-set"); v.String != "120" {
		t.Errorf("k-set override overlap = %q; want 120", v.String)
	}
	// The rest of the configuration is untouched.
	var size string
	if err := db.QueryRow(`SELECT chunking_config->>'chunk_size' FROM knowledge_bases WHERE id = 'kb-zero'`).
		Scan(&size); err != nil || size != "512" {
		t.Errorf("kb-zero chunk_size = %q (%v); want 512", size, err)
	}
}
