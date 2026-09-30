package database

import (
	"testing"
	"testing/fstest"

	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/migrations"
)

// A dirty migration is retried from the version before it in the source, not
// from v-1: versions have gaps (000089 is followed by 000120), and there is no
// file to migrate up from at a version that does not exist.
func TestVersionBeforeFollowsTheSourceAcrossGaps(t *testing.T) {
	fsys := fstest.MapFS{}
	for _, name := range []string{"000000_init", "000089_last", "000120_docs", "000121_next"} {
		fsys[name+".up.sql"] = &fstest.MapFile{Data: []byte("SELECT 1;")}
		fsys[name+".down.sql"] = &fstest.MapFile{Data: []byte("SELECT 1;")}
	}
	src, err := iofs.New(fsys, ".")
	require.NoError(t, err)

	for v, want := range map[uint]int{120: 89, 121: 120, 89: 0, 0: -1} {
		got, err := versionBefore(src, v)
		require.NoError(t, err, "version %d", v)
		assert.Equal(t, want, got, "version %d", v)
	}
	_, err = versionBefore(src, 100)
	assert.Error(t, err, "a version this build has no migration for is not guessed at")
}

// The real core migrations have such a gap.
func TestVersionBeforeOnTheCoreMigrations(t *testing.T) {
	src, err := iofs.New(migrations.Core(), ".")
	require.NoError(t, err)
	got, err := versionBefore(src, 120)
	require.NoError(t, err)
	assert.Equal(t, 89, got)
}
