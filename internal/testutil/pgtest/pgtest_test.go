package pgtest

import (
	"testing"
	"time"
)

// The schema is the production one: the tables of the first and the last
// migration exist, and the migrator recorded the head version as clean.
func TestNewHoldsTheMigratedSchema(t *testing.T) {
	db := New(t)

	for _, table := range []string{"tenants", "chunks", "docs_pages"} {
		var exists bool
		if err := db.Raw(`SELECT to_regclass(?) IS NOT NULL`, table).Scan(&exists).Error; err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Errorf("table %s is missing", table)
		}
	}

	var dirty bool
	if err := db.Raw(`SELECT dirty FROM schema_migrations`).Scan(&dirty).Error; err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Error("the template was migrated dirty")
	}
}

// Every test gets a database of its own: rows written through one are not
// visible through another.
func TestDatabasesAreIsolated(t *testing.T) {
	a, b := New(t), New(t)
	if err := a.Exec(`INSERT INTO tenants (name, business) VALUES ('only in a', 'x')`).Error; err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := b.Raw(`SELECT count(*) FROM tenants`).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("second database sees %d tenant rows written to the first", n)
	}
}

// Sessions run on UTC, as the server's own connection does, and timestamps
// compare as instants whatever the zone they were written in — the property
// SQLite lacked.
func TestTimestampsCompareAsInstants(t *testing.T) {
	db := New(t)
	var tz string
	if err := db.Raw(`SHOW TimeZone`).Scan(&tz).Error; err != nil {
		t.Fatal(err)
	}
	if tz != "UTC" {
		t.Errorf("session time zone = %q, want UTC", tz)
	}

	east := time.FixedZone("UTC+8", 8*60*60)
	at := time.Date(2026, 9, 29, 12, 40, 0, 0, east) // 04:40 UTC
	var before bool
	if err := db.Raw(`SELECT ?::timestamptz < ?::timestamptz`, at, at.UTC().Add(time.Hour)).
		Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	if !before {
		t.Error("12:40+08:00 did not compare as earlier than 05:40Z")
	}
}

// A test's database is gone once the test is.
func TestDatabaseIsDroppedAfterTheTest(t *testing.T) {
	var name string
	t.Run("uses one", func(t *testing.T) {
		db := New(t)
		if err := db.Raw(`SELECT current_database()`).Scan(&name).Error; err != nil {
			t.Fatal(err)
		}
	})
	check := New(t)
	var exists bool
	if err := check.Raw(`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = ?)`, name).
		Scan(&exists).Error; err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Errorf("database %s outlived its test", name)
	}
}
