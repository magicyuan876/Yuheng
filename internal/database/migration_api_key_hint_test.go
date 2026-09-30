package database

import (
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/migrations"
)

// 000132 stops storing API keys in a recoverable form: api_key goes, key_hint
// arrives, and keys that could only ever have been authenticated through
// api_key (000065 placeholders) go with it. Every other key survives, since
// it authenticates by its hash.
func TestAPIKeysLoseTheirStoredSecret(t *testing.T) {
	if testing.Short() {
		t.Skip("migrates a database in two stages; slow")
	}
	const before = 131

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
	mustExec(`INSERT INTO tenants (id, name, business) VALUES (20002, 'acme', 'test')`)
	mustExec(`INSERT INTO tenant_api_keys (tenant_id, scope_type, name, key_hash, api_key)
	          VALUES (20002, 'tenant', 'placeholder', 'migrated-tenant-20002', 'sk-old'),
	                 (20002, 'tenant', 'hashed', 'real-hash', 'enc:v1:ciphertext')`)

	if err := RunMigrationsWithOptions(url, MigrationOptions{}); err != nil {
		t.Fatalf("upgrade to head: %v", err)
	}

	var hasColumn bool
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
	    WHERE table_name = 'tenant_api_keys' AND column_name = 'api_key')`).Scan(&hasColumn); err != nil {
		t.Fatal(err)
	}
	if hasColumn {
		t.Error("tenant_api_keys.api_key still exists; the key itself must not be stored")
	}
	var names []string
	rows, err := db.Query(`SELECT name || ':' || key_hint FROM tenant_api_keys WHERE tenant_id = 20002 ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	if len(names) != 1 || names[0] != "hashed:" {
		t.Errorf("keys after 000132 = %v; want only the hashed key, with an empty hint", names)
	}
}
