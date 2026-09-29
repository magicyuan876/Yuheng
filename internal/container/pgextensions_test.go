package container

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
)

func TestMissingExtensionsReportsWhatTheServerLacks(t *testing.T) {
	db := pgtest.New(t)

	asked := []string{"vector", "no_such_extension", "pg_search", "another_missing"}
	missing, err := missingExtensions(context.Background(), db, asked)

	require.NoError(t, err)
	assert.Equal(t, []string{"no_such_extension", "another_missing"}, missing,
		"in the order asked, only what is absent")
}

// The image docker-compose.yml runs, which the tests run on, must satisfy the
// built-in engine; if this fails, the image and the requirement have drifted.
func TestTheProductionImageProvidesTheRetrievalExtensions(t *testing.T) {
	db := pgtest.New(t)

	require.NoError(t, requireRetrievalExtensions(context.Background(), db))
}

func TestRequireRetrievalExtensionsSaysWhatToDo(t *testing.T) {
	db := pgtest.New(t)
	saved := retrievalExtensions
	retrievalExtensions = []string{"vector", "pg_missing_for_test"}
	t.Cleanup(func() { retrievalExtensions = saved })

	err := requireRetrievalExtensions(context.Background(), db)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pg_missing_for_test")
	assert.Contains(t, err.Error(), "ParadeDB")
}
