package repository

import (
	"context"
	"testing"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"gorm.io/gorm"
)

// openTestDB opens a private PostgreSQL database carrying the production
// schema (every migration, the docs module's included), so every test runs
// against the DDL the server runs rather than a copy of it.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return pgtest.New(t)
}

func ctx() context.Context { return context.Background() }

func ptr[T any](v T) *T { return &v }
