package container

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// retrievalExtensions are the PostgreSQL extensions the built-in retrieval
// engine needs: pgvector for similarity search and pg_search (ParadeDB) for
// BM25 full-text search.
var retrievalExtensions = []string{"vector", "pg_search"}

// missingExtensions returns those of names that the server cannot provide, in
// the order given. "Cannot provide" means not installed on the server, which
// is what pg_available_extensions lists; whether the extension is already
// created in this database does not matter, the migrations create it.
func missingExtensions(ctx context.Context, db *gorm.DB, names []string) ([]string, error) {
	var available []string
	if err := db.WithContext(ctx).
		Raw("SELECT name FROM pg_available_extensions WHERE name IN ?", names).
		Scan(&available).Error; err != nil {
		return nil, fmt.Errorf("listing the extensions the server provides: %w", err)
	}
	have := make(map[string]bool, len(available))
	for _, name := range available {
		have[name] = true
	}
	var missing []string
	for _, name := range names {
		if !have[name] {
			missing = append(missing, name)
		}
	}
	return missing, nil
}

// requireRetrievalExtensions fails, with instructions, when the database server
// lacks an extension the built-in engine needs.
func requireRetrievalExtensions(ctx context.Context, db *gorm.DB) error {
	missing, err := missingExtensions(ctx, db, retrievalExtensions)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"the PostgreSQL server does not provide the extension(s) %s that the built-in retrieval engine needs: "+
			"run the ParadeDB image that docker-compose.yml uses (it bundles pgvector and pg_search), "+
			"or install both on your server; managed PostgreSQL services usually cannot install pg_search",
		strings.Join(missing, ", "))
}
