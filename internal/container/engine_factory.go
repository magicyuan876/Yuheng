package container

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.uber.org/dig"
	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/application/service/retriever"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/extension"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/magicyuan876/yuheng/internal/utils"
)

// engineCatalogParams collects every engine descriptor provided into
// retriever.EngineGroup: the core's own, and those an extension adds.
type engineCatalogParams struct {
	dig.In

	Descriptors []retriever.EngineDescriptor `group:"retrieve_engines"`
}

// NewEngineCatalog builds the catalog of engines this deployment offers.
//
// It refuses to be built before the extension hooks have run: a catalog built
// earlier would silently lack the engines an extension provides.
func NewEngineCatalog(in engineCatalogParams) (*retriever.Catalog, error) {
	if err := extension.RequireApplied("engine catalog"); err != nil {
		return nil, err
	}
	return retriever.NewCatalog(in.Descriptors...)
}

// NewEngineFactory returns the EngineFactory that builds an engine from a
// stored VectorStore row. It is registered in dig and injected into
// VectorStoreService for dynamic registry updates.
func NewEngineFactory(
	db *gorm.DB, cfg *config.Config, auditSvc interfaces.AuditLogService, catalog *retriever.Catalog,
) interfaces.EngineFactory {
	deps := retriever.EngineDeps{DB: db, Config: cfg, Audit: auditSvc}
	return func(ctx context.Context, store types.VectorStore) (interfaces.RetrieveEngineService, error) {
		engine, ok := catalog.ByType(store.EngineType)
		if !ok {
			return nil, fmt.Errorf("vector store engine %q is not available in this deployment", store.EngineType)
		}
		// Stored rows are user-controlled data: the API validates them on the
		// way in, but construction must not assume every row came that way.
		// Fail closed for an engine that was never meant to be registered.
		if !engine.Registrable {
			return nil, fmt.Errorf("vector store engine %q cannot be registered by a workspace", store.EngineType)
		}
		for _, addr := range engine.DialAddresses(store.ConnectionConfig) {
			addr = strings.TrimSpace(addr)
			if addr == "" {
				continue
			}
			if err := utils.ValidateURLForSSRF(addr); err != nil {
				return nil, fmt.Errorf("vector store address failed SSRF validation: %w", err)
			}
		}
		return engine.New(ctx, store, deps)
	}
}

// initRetrieveEngineRegistry builds the retrieval engine registry from the
// engines RETRIEVE_DRIVER names, then adds the stores workspaces registered.
//
// storeRepo and engineFactory let the registry rebuild a store engine that is
// absent from this process, which happens when startup skipped it after a
// construction failure or when another instance registered it.
func initRetrieveEngineRegistry(
	db *gorm.DB, cfg *config.Config, auditSvc interfaces.AuditLogService,
	storeRepo interfaces.VectorStoreRepository, engineFactory interfaces.EngineFactory,
	catalog *retriever.Catalog,
) (interfaces.RetrieveEngineRegistry, error) {
	ctx := context.Background()
	log := logger.GetLogger(ctx)
	registry := retriever.NewRetrieveEngineRegistry(storeRepo, engineFactory)
	deps := retriever.EngineDeps{DB: db, Config: cfg, Audit: auditSvc}

	retrieveDriver := os.Getenv("RETRIEVE_DRIVER")
	// Tenants that configure no engines of their own use what the deployment
	// names here.
	types.SetDefaultRetrieverEngines(catalog.DefaultEngines(retrieveDriver))
	engines, unknown := catalog.Drivers(retrieveDriver)
	for _, token := range unknown {
		// A typo here would otherwise leave the deployment without search and
		// with nothing in the log to say why.
		log.Errorf("RETRIEVE_DRIVER names %q, which is not an engine of this deployment", token)
	}

	// Environment engines are the operator's own configuration and trusted, so
	// they are built directly and not through the workspace-facing factory.
	for _, engine := range engines {
		store := engine.EnvStore(os.Getenv)
		if store == nil {
			continue
		}
		svc, err := engine.New(ctx, *store, deps)
		if err != nil {
			log.Errorf("Create %s retrieve engine failed: %v", engine.Type, err)
			continue
		}
		if err := registry.Register(svc); err != nil {
			log.Errorf("Register %s retrieve engine failed: %v", engine.Type, err)
			continue
		}
		log.Infof("Register %s retrieve engine success", engine.Type)
	}

	if storeRegistry, ok := registry.(interfaces.StoreRegistry); ok {
		loadDBStoresIntoRegistry(storeRegistry, db, engineFactory)
	}
	return registry, nil
}

// loadDBStoresIntoRegistry registers the VectorStore rows workspaces created.
// A store that fails to build is logged and skipped: one unreachable cluster
// must not keep the application from starting.
func loadDBStoresIntoRegistry(registry interfaces.StoreRegistry, db *gorm.DB, factory interfaces.EngineFactory) {
	ctx := context.Background()
	log := logger.GetLogger(ctx)

	var stores []types.VectorStore
	// GORM soft delete automatically adds "deleted_at IS NULL" condition
	if err := db.Find(&stores).Error; err != nil {
		log.Warnf("Failed to load vector stores from DB: %v", err)
		return
	}
	if len(stores) == 0 {
		return
	}

	log.Infof("Loading %d vector store(s) from database", len(stores))
	for _, store := range stores {
		svc, err := factory(ctx, store)
		if err != nil {
			log.Errorf("Failed to create engine for store %s (%s): %v", store.ID, store.Name, err)
			continue
		}
		registry.RegisterWithStoreID(store.ID, svc)
		log.Infof("Registered DB vector store: id=%s, name=%s, engine=%s", store.ID, store.Name, store.EngineType)
	}
}
