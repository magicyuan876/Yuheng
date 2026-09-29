// Package retriever assembles retrieval: the engines a deployment offers (the
// catalog), the registry of the ones running, and the composite engines that
// fan a query out to them.
package retriever

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// The engine catalog: everything the platform needs to know about a retrieval
// engine, gathered in one value.
//
// Before it existed, adding an engine meant finding and editing a dozen switch
// statements (construction, connection test, address policy, score handling,
// the settings form, the environment shortcut ...) and forgetting one failed
// silently. Now an engine is one EngineDescriptor, and the code that used to
// switch on the engine type asks the Catalog instead. The core registers the
// engines it ships; an extension adds its own by providing a descriptor into
// EngineGroup (see the extension package).

// EngineGroup is the dig value group that engine descriptors are provided
// into.
const EngineGroup = "retrieve_engines"

// ScoreScale says in what range an engine reports similarity, so scores from
// different engines can be put on one scale before they are merged.
type ScoreScale int

const (
	// ScoreUnit is a score already in [0, 1]: the engine or its driver
	// translated it, or L2-normalised embeddings keep it there.
	ScoreUnit ScoreScale = iota
	// ScoreSignedCosine is the raw cosine in [-1, 1].
	ScoreSignedCosine
)

// EngineCapabilities describes what an engine can do.
type EngineCapabilities struct {
	// Retrievers lists the retrieval kinds the engine implements.
	Retrievers []types.RetrieverType
	// ScoreScale is the range of the similarity scores it reports.
	ScoreScale ScoreScale
	// SupportsACL says the engine can filter by item-level permissions
	// (RetrieveParams.Subjects). A knowledge base may only use entry-level
	// permissions when its engine says so.
	SupportsACL bool
}

// Supports reports whether the engine implements the retrieval kind.
func (c EngineCapabilities) Supports(kind types.RetrieverType) bool {
	for _, k := range c.Retrievers {
		if k == kind {
			return true
		}
	}
	return false
}

// EngineDeps is what an engine may draw on when it is constructed. Anything an
// engine needs beyond these it builds itself from its store's configuration.
type EngineDeps struct {
	// DB is the application's own database, for engines that live in it.
	DB *gorm.DB
	// Config is the application configuration.
	Config *config.Config
	// Audit records administrative events; it may be nil.
	Audit interfaces.AuditLogService
}

// EngineDescriptor is one retrieval engine, described.
type EngineDescriptor struct {
	// Type identifies the engine in stores, knowledge bases and results.
	Type types.RetrieverEngineType
	// Driver is the token that selects the engine in RETRIEVE_DRIVER.
	Driver string
	// DisplayName is what the settings screen shows.
	DisplayName string
	// Capabilities describes what the engine can do.
	Capabilities EngineCapabilities

	// Registrable says a workspace may register its own stores of this engine
	// through the vector-store API. An engine that lives in the application's
	// own database is not: it is only ever the environment's.
	Registrable bool
	// ConnectionFields and IndexFields drive the registration form. They are
	// only meaningful for a registrable engine.
	ConnectionFields []types.VectorStoreFieldInfo
	IndexFields      []types.VectorStoreFieldInfo

	// DefaultIndexName is the index (or collection) the engine uses when a
	// store does not name one. Two stores that would share an index and a
	// server are refused as duplicates.
	DefaultIndexName string

	// EnvStore builds the store that RETRIEVE_DRIVER implies for this engine.
	EnvStore func(env types.EnvLookupFunc) *types.VectorStore
	// ValidateConnection checks the fields the engine requires. Nil accepts
	// anything.
	ValidateConnection func(types.ConnectionConfig) error
	// ValidateIndex checks the engine's own index settings, on top of the
	// bounds every engine shares. Nil accepts anything.
	ValidateIndex func(types.IndexConfig) error
	// DialAddresses lists the addresses the driver would connect to. Each one
	// supplied by a user is checked against the SSRF policy before the engine
	// is contacted. A registrable engine must provide it.
	DialAddresses func(types.ConnectionConfig) []string
	// TestConnection probes a server and returns its version if it can tell.
	// A registrable engine must provide it.
	TestConnection func(ctx context.Context, cc types.ConnectionConfig) (string, error)
	// New builds the engine for a store.
	New func(ctx context.Context, store types.VectorStore, deps EngineDeps) (interfaces.RetrieveEngineService, error)
}

// check rejects a descriptor the platform could not use, at registration
// instead of when someone first reaches for the missing piece.
func (d EngineDescriptor) check() error {
	switch {
	case d.Type == "":
		return fmt.Errorf("engine descriptor has no type")
	case d.Driver == "":
		return fmt.Errorf("engine %q has no driver token", d.Type)
	case d.DisplayName == "":
		return fmt.Errorf("engine %q has no display name", d.Type)
	case len(d.Capabilities.Retrievers) == 0:
		return fmt.Errorf("engine %q implements no retrieval kind", d.Type)
	case d.EnvStore == nil:
		return fmt.Errorf("engine %q has no environment store", d.Type)
	case d.New == nil:
		return fmt.Errorf("engine %q has no constructor", d.Type)
	case d.Registrable && (d.DialAddresses == nil || d.TestConnection == nil):
		return fmt.Errorf("registrable engine %q must declare its dial addresses and a connection test", d.Type)
	}
	return nil
}

// Catalog is the set of engines this deployment offers. It is immutable once
// built, so it needs no locking.
type Catalog struct {
	ordered  []EngineDescriptor
	byType   map[types.RetrieverEngineType]EngineDescriptor
	byDriver map[string]EngineDescriptor
}

// NewCatalog builds a catalog. It refuses an incomplete descriptor and two
// descriptors that claim the same type or driver token.
func NewCatalog(descriptors ...EngineDescriptor) (*Catalog, error) {
	c := &Catalog{
		byType:   make(map[types.RetrieverEngineType]EngineDescriptor, len(descriptors)),
		byDriver: make(map[string]EngineDescriptor, len(descriptors)),
	}
	for _, d := range descriptors {
		if err := d.check(); err != nil {
			return nil, err
		}
		if _, dup := c.byType[d.Type]; dup {
			return nil, fmt.Errorf("engine type %q is described twice", d.Type)
		}
		if _, dup := c.byDriver[d.Driver]; dup {
			return nil, fmt.Errorf("driver token %q is claimed by two engines", d.Driver)
		}
		c.byType[d.Type] = d
		c.byDriver[d.Driver] = d
		c.ordered = append(c.ordered, d)
	}
	// A stable order keeps the settings screen and the logs the same from one
	// start to the next, whatever order the extensions registered in.
	sort.SliceStable(c.ordered, func(i, j int) bool { return c.ordered[i].Type < c.ordered[j].Type })
	return c, nil
}

// ByType returns the engine of a type.
func (c *Catalog) ByType(t types.RetrieverEngineType) (EngineDescriptor, bool) {
	if c == nil {
		return EngineDescriptor{}, false
	}
	d, ok := c.byType[t]
	return d, ok
}

// ByDriver returns the engine a RETRIEVE_DRIVER token names.
func (c *Catalog) ByDriver(token string) (EngineDescriptor, bool) {
	if c == nil {
		return EngineDescriptor{}, false
	}
	d, ok := c.byDriver[strings.TrimSpace(token)]
	return d, ok
}

// All returns every engine, in a stable order.
func (c *Catalog) All() []EngineDescriptor {
	if c == nil {
		return nil
	}
	return append([]EngineDescriptor(nil), c.ordered...)
}

// Registrable returns the engines a workspace may register stores of.
func (c *Catalog) Registrable() []EngineDescriptor {
	var out []EngineDescriptor
	for _, d := range c.All() {
		if d.Registrable {
			out = append(out, d)
		}
	}
	return out
}

// Drivers parses a RETRIEVE_DRIVER value into the engines it names, in order,
// and lists the tokens that name none.
func (c *Catalog) Drivers(retrieveDriver string) (engines []EngineDescriptor, unknown []string) {
	seen := map[types.RetrieverEngineType]bool{}
	for _, token := range strings.Split(retrieveDriver, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		d, ok := c.ByDriver(token)
		if !ok {
			unknown = append(unknown, token)
			continue
		}
		if !seen[d.Type] {
			seen[d.Type] = true
			engines = append(engines, d)
		}
	}
	return engines, unknown
}

// EnvStores builds the virtual stores RETRIEVE_DRIVER implies.
func (c *Catalog) EnvStores(retrieveDriver string, env types.EnvLookupFunc) []types.VectorStore {
	engines, _ := c.Drivers(retrieveDriver)
	stores := make([]types.VectorStore, 0, len(engines))
	for _, d := range engines {
		if s := d.EnvStore(env); s != nil {
			stores = append(stores, *s)
		}
	}
	return stores
}

// FindEnvStore returns the virtual store with the given ID.
func (c *Catalog) FindEnvStore(retrieveDriver string, env types.EnvLookupFunc, id string) *types.VectorStore {
	for _, s := range c.EnvStores(retrieveDriver, env) {
		if s.ID == id {
			return &s
		}
	}
	return nil
}

// TypeInfos describes the registrable engines for the registration form.
func (c *Catalog) TypeInfos() []types.VectorStoreTypeInfo {
	registrable := c.Registrable()
	infos := make([]types.VectorStoreTypeInfo, 0, len(registrable))
	for _, d := range registrable {
		infos = append(infos, types.VectorStoreTypeInfo{
			Type:             string(d.Type),
			DisplayName:      d.DisplayName,
			ConnectionFields: d.ConnectionFields,
			IndexFields:      d.IndexFields,
		})
	}
	return infos
}

// DefaultIndexName returns the index a store of the engine uses when it names
// none: the store's own choice, else the engine's default.
func (c *Catalog) DefaultIndexName(t types.RetrieverEngineType, ic types.IndexConfig) string {
	if ic.IndexName != "" {
		return ic.IndexName
	}
	if d, ok := c.ByType(t); ok {
		return d.DefaultIndexName
	}
	return ""
}

// DefaultEngines returns the engine parameters a tenant with none of its own
// uses: every kind of retrieval each RETRIEVE_DRIVER engine implements.
func (c *Catalog) DefaultEngines(retrieveDriver string) []types.RetrieverEngineParams {
	engines, _ := c.Drivers(retrieveDriver)
	var params []types.RetrieverEngineParams
	for _, d := range engines {
		for _, kind := range d.Capabilities.Retrievers {
			params = append(params, types.RetrieverEngineParams{RetrieverType: kind, RetrieverEngineType: d.Type})
		}
	}
	return params
}
