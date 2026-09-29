package retriever

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/types"
)

func TestNewCatalog_RejectsIncompleteDescriptors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mutate  func(*EngineDescriptor)
		wantErr string
	}{
		{"no type", func(d *EngineDescriptor) { d.Type = "" }, "no type"},
		{"no driver", func(d *EngineDescriptor) { d.Driver = "" }, "no driver"},
		{"no display name", func(d *EngineDescriptor) { d.DisplayName = "" }, "no display name"},
		{"no retrievers", func(d *EngineDescriptor) { d.Capabilities.Retrievers = nil }, "no retrieval kind"},
		{"no env store", func(d *EngineDescriptor) { d.EnvStore = nil }, "no environment store"},
		{"no constructor", func(d *EngineDescriptor) { d.New = nil }, "no constructor"},
		{"registrable without dial addresses", func(d *EngineDescriptor) {
			d.Registrable = true
			d.TestConnection = stubRegistrableDescriptor("x", nil).TestConnection
		}, "dial addresses"},
		{"registrable without connection test", func(d *EngineDescriptor) {
			d.Registrable = true
			d.DialAddresses = func(types.ConnectionConfig) []string { return nil }
		}, "connection test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := stubDescriptor(stubUnitType, ScoreUnit)
			tt.mutate(&d)
			c, err := NewCatalog(d)
			require.Error(t, err)
			assert.Nil(t, c)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNewCatalog_RejectsDuplicates(t *testing.T) {
	t.Parallel()

	t.Run("duplicate type", func(t *testing.T) {
		a := stubDescriptor(stubUnitType, ScoreUnit)
		b := stubDescriptor(stubUnitType, ScoreUnit)
		b.Driver = "another"
		_, err := NewCatalog(a, b)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "described twice")
	})

	t.Run("duplicate driver", func(t *testing.T) {
		a := stubDescriptor(stubUnitType, ScoreUnit)
		b := stubDescriptor(stubSignedType, ScoreUnit)
		b.Driver = a.Driver
		_, err := NewCatalog(a, b)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "claimed by two engines")
	})

	t.Run("empty catalog is valid", func(t *testing.T) {
		c, err := NewCatalog()
		require.NoError(t, err)
		assert.Empty(t, c.All())
	})
}

func TestCatalog_ByTypeAndByDriver(t *testing.T) {
	t.Parallel()
	c := stubCatalog(t)

	d, ok := c.ByType(stubSignedType)
	require.True(t, ok)
	assert.Equal(t, stubSignedType, d.Type)

	_, ok = c.ByType("nosuch")
	assert.False(t, ok)

	d, ok = c.ByDriver("postgres")
	require.True(t, ok)
	assert.Equal(t, types.PostgresRetrieverEngineType, d.Type)

	d, ok = c.ByDriver("  postgres \t")
	require.True(t, ok, "whitespace around a driver token is ignored")
	assert.Equal(t, types.PostgresRetrieverEngineType, d.Type)

	_, ok = c.ByDriver("")
	assert.False(t, ok)
	_, ok = c.ByDriver("nosuch")
	assert.False(t, ok)
}

func TestCatalog_AllIsStableAndACopy(t *testing.T) {
	t.Parallel()
	// Registered in reverse order; the catalog orders by type.
	c, err := NewCatalog(
		stubDescriptor(stubUnitType, ScoreUnit),
		stubDescriptor(stubSignedType, ScoreSignedCosine),
		PostgresDescriptor(),
	)
	require.NoError(t, err)

	want := []types.RetrieverEngineType{types.PostgresRetrieverEngineType, stubSignedType, stubUnitType}
	var got []types.RetrieverEngineType
	for _, d := range c.All() {
		got = append(got, d.Type)
	}
	assert.Equal(t, want, got)

	all := c.All()
	all[0] = stubDescriptor("mutated", ScoreUnit)
	assert.Equal(t, types.PostgresRetrieverEngineType, c.All()[0].Type,
		"mutating the result must not change the catalog")
}

func TestCatalog_Registrable(t *testing.T) {
	t.Parallel()
	dial := func(types.ConnectionConfig) []string { return nil }
	c, err := NewCatalog(
		PostgresDescriptor(),
		stubRegistrableDescriptor(stubSignedType, dial),
		stubDescriptor(stubUnitType, ScoreUnit),
	)
	require.NoError(t, err)

	got := c.Registrable()
	require.Len(t, got, 1)
	assert.Equal(t, stubSignedType, got[0].Type)
}

func TestCatalog_Drivers(t *testing.T) {
	t.Parallel()
	c := stubCatalog(t)

	t.Run("keeps the order the value names", func(t *testing.T) {
		engines, unknown := c.Drivers("unit,postgres,signed")
		assert.Empty(t, unknown)
		require.Len(t, engines, 3)
		assert.Equal(t, stubUnitType, engines[0].Type)
		assert.Equal(t, types.PostgresRetrieverEngineType, engines[1].Type)
		assert.Equal(t, stubSignedType, engines[2].Type)
	})

	t.Run("dedupes and trims", func(t *testing.T) {
		engines, unknown := c.Drivers(" postgres , postgres,unit ")
		assert.Empty(t, unknown)
		require.Len(t, engines, 2)
		assert.Equal(t, types.PostgresRetrieverEngineType, engines[0].Type)
		assert.Equal(t, stubUnitType, engines[1].Type)
	})

	t.Run("reports unknown tokens and keeps the known ones", func(t *testing.T) {
		engines, unknown := c.Drivers("postgres,elasticsearch, typo ,")
		require.Len(t, engines, 1)
		assert.Equal(t, []string{"elasticsearch", "typo"}, unknown)
	})

	t.Run("empty string names nothing", func(t *testing.T) {
		engines, unknown := c.Drivers("")
		assert.Empty(t, engines)
		assert.Empty(t, unknown)
	})
}

func TestCatalog_EnvStoresAndFindEnvStore(t *testing.T) {
	t.Parallel()
	c := stubCatalog(t)
	env := func(string) string { return "" }

	stores := c.EnvStores("postgres,unit,nosuch", env)
	require.Len(t, stores, 2)
	assert.Equal(t, PostgresEnvStoreID, stores[0].ID)
	assert.Equal(t, types.EnvStoreIDPrefix+"unit__", stores[1].ID)

	found := c.FindEnvStore("postgres,unit", env, PostgresEnvStoreID)
	require.NotNil(t, found)
	assert.Equal(t, types.PostgresRetrieverEngineType, found.EngineType)

	assert.Nil(t, c.FindEnvStore("postgres", env, types.EnvStoreIDPrefix+"unit__"),
		"an engine RETRIEVE_DRIVER omits has no env store")
	assert.Nil(t, c.FindEnvStore("", env, PostgresEnvStoreID))
	assert.Empty(t, c.EnvStores("", env))

	t.Run("an engine whose env store is nil is skipped", func(t *testing.T) {
		d := stubDescriptor(stubUnitType, ScoreUnit)
		d.EnvStore = func(types.EnvLookupFunc) *types.VectorStore { return nil }
		cat, err := NewCatalog(d)
		require.NoError(t, err)
		assert.Empty(t, cat.EnvStores("unit", env))
	})
}

func TestCatalog_TypeInfosListsOnlyRegistrableEngines(t *testing.T) {
	t.Parallel()
	reg := stubRegistrableDescriptor(stubSignedType, func(types.ConnectionConfig) []string { return nil })
	reg.ConnectionFields = []types.VectorStoreFieldInfo{{Name: "addr", Type: "string", Required: true}}
	reg.IndexFields = []types.VectorStoreFieldInfo{{Name: "index_name", Type: "string"}}
	c, err := NewCatalog(PostgresDescriptor(), reg, stubDescriptor(stubUnitType, ScoreUnit))
	require.NoError(t, err)

	infos := c.TypeInfos()
	require.Len(t, infos, 1)
	assert.Equal(t, string(stubSignedType), infos[0].Type)
	assert.Equal(t, reg.DisplayName, infos[0].DisplayName)
	assert.Equal(t, reg.ConnectionFields, infos[0].ConnectionFields)
	assert.Equal(t, reg.IndexFields, infos[0].IndexFields)

	empty, err := NewCatalog(PostgresDescriptor())
	require.NoError(t, err)
	assert.NotNil(t, empty.TypeInfos(), "an empty list, not null, on the wire")
	assert.Empty(t, empty.TypeInfos())
}

func TestCatalog_DefaultIndexName(t *testing.T) {
	t.Parallel()
	c := stubCatalog(t)

	assert.Equal(t, "mine", c.DefaultIndexName(stubUnitType, types.IndexConfig{IndexName: "mine"}),
		"the store's own choice wins")
	assert.Equal(t, "stub_unit", c.DefaultIndexName(stubUnitType, types.IndexConfig{}),
		"otherwise the engine's default")
	assert.Equal(t, "embeddings", c.DefaultIndexName(types.PostgresRetrieverEngineType, types.IndexConfig{}))
	assert.Equal(t, "", c.DefaultIndexName("nosuch", types.IndexConfig{}))
	assert.Equal(t, "mine", c.DefaultIndexName("nosuch", types.IndexConfig{IndexName: "mine"}),
		"an unknown engine still honours the store's own name")
}

func TestCatalog_DefaultEnginesExpandsEveryRetrieverKind(t *testing.T) {
	t.Parallel()
	c := stubCatalog(t)

	got := c.DefaultEngines("postgres,unit,nosuch")
	assert.Equal(t, []types.RetrieverEngineParams{
		{RetrieverType: types.KeywordsRetrieverType, RetrieverEngineType: types.PostgresRetrieverEngineType},
		{RetrieverType: types.VectorRetrieverType, RetrieverEngineType: types.PostgresRetrieverEngineType},
		{RetrieverType: types.KeywordsRetrieverType, RetrieverEngineType: stubUnitType},
		{RetrieverType: types.VectorRetrieverType, RetrieverEngineType: stubUnitType},
	}, got)

	assert.Empty(t, c.DefaultEngines(""))
}

func TestCatalog_NilIsSafeForLookups(t *testing.T) {
	t.Parallel()
	var c *Catalog
	env := func(string) string { return "" }

	_, ok := c.ByType(types.PostgresRetrieverEngineType)
	assert.False(t, ok)
	_, ok = c.ByDriver("postgres")
	assert.False(t, ok)
	assert.Empty(t, c.All())
	assert.Empty(t, c.Registrable())
	engines, unknown := c.Drivers("postgres")
	assert.Empty(t, engines)
	assert.Equal(t, []string{"postgres"}, unknown)
	assert.Empty(t, c.EnvStores("postgres", env))
	assert.Nil(t, c.FindEnvStore("postgres", env, PostgresEnvStoreID))
	assert.Empty(t, c.TypeInfos())
	assert.Equal(t, "custom",
		c.DefaultIndexName(types.PostgresRetrieverEngineType, types.IndexConfig{IndexName: "custom"}))
	assert.Equal(t, "", c.DefaultIndexName(types.PostgresRetrieverEngineType, types.IndexConfig{}))
	assert.Empty(t, c.DefaultEngines("postgres"))
}

func TestPostgresDescriptor(t *testing.T) {
	t.Parallel()
	d := PostgresDescriptor()

	assert.Equal(t, types.PostgresRetrieverEngineType, d.Type)
	assert.False(t, d.Registrable, "PostgreSQL is the environment's engine, never a workspace's")
	assert.True(t, d.Capabilities.SupportsACL)
	assert.Equal(t, ScoreUnit, d.Capabilities.ScoreScale)
	assert.True(t, d.Capabilities.Supports(types.KeywordsRetrieverType))
	assert.True(t, d.Capabilities.Supports(types.VectorRetrieverType))
	assert.False(t, d.Capabilities.Supports(types.RetrieverType("nosuch")))
	assert.True(t, strings.HasPrefix(PostgresEnvStoreID, types.EnvStoreIDPrefix))

	_, err := NewCatalog(d)
	require.NoError(t, err, "the built-in descriptor must satisfy the catalog's own checks")

	t.Run("env store uses the default connection", func(t *testing.T) {
		store := d.EnvStore(func(string) string { return "" })
		require.NotNil(t, store)
		assert.Equal(t, PostgresEnvStoreID, store.ID)
		assert.Equal(t, types.PostgresRetrieverEngineType, store.EngineType)
		assert.True(t, store.ConnectionConfig.UseDefaultConnection)
	})

	t.Run("New refuses a non-default connection", func(t *testing.T) {
		svc, err := d.New(context.Background(), types.VectorStore{
			EngineType:       types.PostgresRetrieverEngineType,
			ConnectionConfig: types.ConnectionConfig{Addr: "postgres://elsewhere:5432/db"},
		}, EngineDeps{})
		require.Error(t, err)
		assert.Nil(t, svc)
	})

	t.Run("the connection test needs no server", func(t *testing.T) {
		version, err := d.TestConnection(context.Background(), types.ConnectionConfig{UseDefaultConnection: true})
		assert.NoError(t, err)
		assert.Empty(t, version)
	})
}
