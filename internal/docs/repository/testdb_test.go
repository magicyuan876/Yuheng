package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var testDBSeq atomic.Int64

// openTestDB opens a private in-memory SQLite database and applies the docs
// module's schema, so every test runs against the real DDL rather than
// against whatever AutoMigrate would infer.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("file:docs-repo-%d?mode=memory&cache=shared&_foreign_keys=1", testDBSeq.Add(1))
	db, err := gorm.Open(sqlite.Open(name), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// Shared-cache memory databases vanish when the last connection closes;
	// keep one open for the duration of the test.
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	ddl, err := os.ReadFile(filepath.FromSlash("../testdata/schema_sqlite.sql"))
	require.NoError(t, err, "the SQLite migration must exist")
	require.NoError(t, db.Exec(string(ddl)).Error, "the SQLite migration must apply cleanly")
	return db
}

func ctx() context.Context { return context.Background() }

func ptr[T any](v T) *T { return &v }
