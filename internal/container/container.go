// Package container implements dependency injection container setup
// Provides centralized configuration for services, repositories, and handlers
// This package is responsible for wiring up all dependencies and ensuring proper lifecycle management
package container

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/panjf2000/ants/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/dig"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/application/repository"
	neo4jRepo "github.com/magicyuan876/yuheng/internal/application/repository/retriever/neo4j"
	"github.com/magicyuan876/yuheng/internal/application/service"
	chatpipeline "github.com/magicyuan876/yuheng/internal/application/service/chat_pipeline"
	"github.com/magicyuan876/yuheng/internal/application/service/file"
	"github.com/magicyuan876/yuheng/internal/application/service/findings"
	"github.com/magicyuan876/yuheng/internal/application/service/retriever"
	"github.com/magicyuan876/yuheng/internal/common"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/database"
	"github.com/magicyuan876/yuheng/internal/datasource"
	"github.com/magicyuan876/yuheng/internal/datasource/connector/feishu/core"
	"github.com/magicyuan876/yuheng/internal/datasource/connector/feishu/drive"
	"github.com/magicyuan876/yuheng/internal/datasource/connector/feishu/wiki"
	gitlabConnector "github.com/magicyuan876/yuheng/internal/datasource/connector/gitlab"
	imaConnector "github.com/magicyuan876/yuheng/internal/datasource/connector/ima"
	notionConnector "github.com/magicyuan876/yuheng/internal/datasource/connector/notion"
	rssConnector "github.com/magicyuan876/yuheng/internal/datasource/connector/rss"
	yuqueConnector "github.com/magicyuan876/yuheng/internal/datasource/connector/yuque"
	"github.com/magicyuan876/yuheng/internal/docs"
	"github.com/magicyuan876/yuheng/internal/event"
	"github.com/magicyuan876/yuheng/internal/extension"
	"github.com/magicyuan876/yuheng/internal/handler"
	"github.com/magicyuan876/yuheng/internal/handler/session"
	"github.com/magicyuan876/yuheng/internal/infrastructure/docparser"
	infra_web_search "github.com/magicyuan876/yuheng/internal/infrastructure/web_search"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/models/chat"
	"github.com/magicyuan876/yuheng/internal/models/embedding"
	"github.com/magicyuan876/yuheng/internal/models/limiter"
	"github.com/magicyuan876/yuheng/internal/models/utils/ollama"
	"github.com/magicyuan876/yuheng/internal/router"
	"github.com/magicyuan876/yuheng/internal/storageallowlist"
	"github.com/magicyuan876/yuheng/internal/stream"
	"github.com/magicyuan876/yuheng/internal/tracing/langfuse"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

// BuildContainer constructs the dependency injection container
// Registers all components, services, repositories and handlers needed by the application
// Creates a fully configured application container with proper dependency resolution
// Parameters:
//   - container: Base dig container to add dependencies to
//
// Returns:
//   - Configured container with all application dependencies registered
func BuildContainer(container *dig.Container) *dig.Container {
	ctx := context.Background()
	logger.Debugf(ctx, "[Container] Starting container initialization...")

	// Register resource cleaner for proper cleanup of resources
	must(container.Provide(NewResourceCleaner, dig.As(new(interfaces.ResourceCleaner))))

	// Core infrastructure configuration
	logger.Debugf(ctx, "[Container] Registering core infrastructure...")
	must(container.Provide(config.LoadConfig))
	must(container.Provide(initLangfuse))
	must(container.Provide(initDatabase))
	must(container.Provide(initFileService))
	must(container.Provide(initRedisClient))
	must(container.Provide(initAntsPool))

	must(container.Invoke(registerLangfuseCleanup))

	// Register goroutine pool cleanup handler
	must(container.Invoke(registerPoolCleanup))

	// Initialize retrieval engine registry for search capabilities
	logger.Debugf(ctx, "[Container] Registering retrieval engine registry...")
	must(container.Provide(NewEngineCatalog))
	// The engine the platform ships. An extension adds its own the same way, into
	// the same group (see the extension package).
	must(container.Provide(retriever.PostgresDescriptor, dig.Group(retriever.EngineGroup)))
	must(container.Provide(initRetrieveEngineRegistry))

	// External service clients
	logger.Debugf(ctx, "[Container] Registering external service clients...")
	must(container.Provide(initDocReaderClient))
	must(container.Provide(docparser.NewImageResolver))
	must(container.Provide(initOllamaService))
	must(container.Provide(initNeo4jClient))
	must(container.Provide(stream.NewStreamManager))
	logger.Debugf(ctx, "[Container] Initializing DuckDB...")
	must(container.Provide(NewDuckDB))
	logger.Debugf(ctx, "[Container] DuckDB registered")

	// Data repositories layer
	logger.Debugf(ctx, "[Container] Registering repositories...")
	must(container.Provide(repository.NewTenantRepository))
	must(container.Provide(repository.NewTenantAPIKeyRepository))
	must(container.Provide(repository.NewTenantMemberRepository))
	must(container.Provide(repository.NewTenantInvitationRepository))
	must(container.Provide(repository.NewAuditLogRepository))
	must(container.Provide(repository.NewKnowledgeBaseRepository))
	must(container.Provide(repository.NewKnowledgeRepository))
	must(container.Provide(repository.NewKnowledgeSpanRepository))
	must(container.Provide(repository.NewChunkRepository))
	must(container.Provide(repository.NewKnowledgeTagRepository))
	must(container.Provide(repository.NewSessionRepository))
	must(container.Provide(repository.NewMessageRepository))
	must(container.Provide(repository.NewMessageSuggestionRepository))
	must(container.Provide(repository.NewModelRepository))
	must(container.Provide(repository.NewUserRepository))
	must(container.Provide(repository.NewAuthTokenRepository))
	must(container.Provide(repository.NewSystemSettingRepository))
	must(container.Provide(neo4jRepo.NewNeo4jRepository))
	must(container.Provide(repository.NewOrganizationRepository))
	must(container.Provide(repository.NewKBShareRepository))
	must(container.Provide(repository.NewUserResourceFavoriteRepository))
	must(container.Provide(service.NewWebSearchStateService))
	must(container.Provide(repository.NewDataSourceRepository))
	must(container.Provide(repository.NewSyncLogRepository))
	must(container.Provide(repository.NewWikiPageRepository))
	must(container.Provide(repository.NewTaskPendingOpsRepository))
	must(container.Provide(repository.NewTaskDeadLetterRepository))
	must(container.Provide(repository.NewKnowledgeFindingRepository))
	must(container.Provide(repository.NewKnowledgeStewardshipRepository))

	// Business service layer
	logger.Debugf(ctx, "[Container] Registering business services...")
	must(container.Provide(service.NewTenantService))
	must(container.Provide(service.NewTenantAPIKeyService))
	must(container.Provide(service.NewTenantMemberService))
	must(container.Provide(service.NewTenantInvitationService))
	must(container.Provide(service.NewAuditLogService))
	must(container.Provide(service.NewAuditLogRetentionRunner))
	must(container.Provide(service.NewKnowledgeBaseService))
	must(container.Provide(service.NewOrganizationService))
	must(container.Provide(service.NewKBShareService)) // KBShareService must be registered before KnowledgeService and KnowledgeTagService
	must(container.Provide(service.NewKnowledgeService))
	must(container.Provide(service.NewSpanTracker))
	must(container.Provide(service.NewChunkService))
	must(container.Provide(service.NewKnowledgeTagService))
	must(container.Provide(embedding.NewBatchEmbedder))
	must(container.Provide(service.NewModelService))
	must(container.Provide(service.NewDatasetService))
	must(container.Provide(service.NewEvaluationService))
	must(container.Provide(service.NewUserService))
	must(container.Provide(service.NewSystemSettingService))
	// Platform layer for parser engine configuration (ENV < platform < workspace).
	// The read half is additionally registered as a process-wide hook in
	// cmd/server/bootstrap.go so the existing DocReader call sites pick it up.
	must(container.Provide(service.NewParserEngineResolver))

	// Extract services - register individual extracters with names
	must(container.Provide(service.NewChunkExtractService, dig.Name("chunkExtractor")))
	must(container.Provide(service.NewDataTableSummaryService, dig.Name("dataTableSummary")))
	must(container.Provide(service.NewImageMultimodalService, dig.Name("imageMultimodal")))
	must(container.Provide(service.NewKnowledgePostProcessService, dig.Name("knowledgePostProcess")))
	must(container.Provide(service.NewKnowledgeAutoTagService, dig.Name("knowledgeAutoTag")))

	// Knowledge health. The core's duplicate detector goes into the detector
	// group the same way an extension adds one (see the findings package);
	// the runner is built from the group after the extension hooks have run.
	must(container.Provide(findings.NewFinderResolver))
	must(container.Provide(newDuplicateDetector, dig.Group(findings.DetectorGroup)))
	must(container.Provide(findings.NewRunnerFromContainer))
	must(container.Provide(findings.NewTrigger, dig.As(new(interfaces.KnowledgeFindingsTrigger))))
	must(container.Provide(findings.NewTaskHandler, dig.Name("knowledgeFindings")))
	must(container.Provide(service.NewKnowledgeFindingService))
	must(container.Provide(service.NewKnowledgeStewardshipService))

	must(container.Provide(service.NewMessageService))
	must(container.Provide(service.NewMessageSuggestionService))
	must(container.Provide(service.NewUserResourceFavoriteService))
	must(container.Provide(service.NewWikiPageService))
	must(container.Provide(service.NewWikiIngestService, dig.Name("wikiIngest")))
	must(container.Provide(service.NewWikiLintService))

	// Web search registry and providers
	logger.Debugf(ctx, "[Container] Registering web search registry and providers...")
	must(container.Provide(infra_web_search.NewRegistry))
	must(container.Invoke(registerWebSearchProviders))
	must(container.Provide(repository.NewWebSearchProviderRepository))
	must(container.Provide(repository.NewVectorStoreRepository))
	must(container.Provide(repository.NewStorageBackendRepository))
	must(container.Provide(repository.NewResourceRepository))
	must(container.Provide(repository.NewTemporaryDocumentRepository))
	must(container.Provide(service.NewResourceCatalog))
	// TenantStoreOwnership adapter used by the retriever factory functions
	// to verify that a resolved VectorStore belongs to the caller's tenant.
	must(container.Provide(retriever.NewVectorStoreRepoOwnership))
	must(container.Provide(service.NewWebSearchService))
	must(container.Provide(service.NewWebSearchProviderService))
	must(container.Provide(NewEngineFactory))
	// StoreRegistry: same instance as RetrieveEngineRegistry, exposed as StoreRegistry interface.
	// NewRetrieveEngineRegistry always returns *retriever.RetrieveEngineRegistry which implements both.
	must(container.Provide(func(r interfaces.RetrieveEngineRegistry) (interfaces.StoreRegistry, error) {
		sr, ok := r.(*retriever.RetrieveEngineRegistry)
		if !ok {
			return nil, fmt.Errorf("registry does not implement StoreRegistry")
		}
		return sr, nil
	}))
	must(container.Provide(service.NewVectorStoreService))
	must(container.Provide(service.NewStorageBackendServiceWithResources))
	must(container.Provide(func(s *service.StorageBackendService) interfaces.StorageBackendService { return s }))
	must(container.Provide(func(s *service.StorageBackendService) interfaces.StorageBackendResolver { return s }))

	// Event bus and session service
	logger.Debugf(ctx, "[Container] Registering event bus and session service...")
	must(container.Provide(event.NewEventBus))

	logger.Debugf(ctx, "[Container] Registering session service...")
	must(container.Provide(service.NewSessionService))

	logger.Debugf(ctx, "[Container] Registering task enqueuer...")
	redisAvailable := os.Getenv("REDIS_ADDR") != ""
	if redisAvailable {
		must(container.Provide(router.NewAsyncqClient, dig.As(new(interfaces.TaskEnqueuer))))
		// Dedicated pools guarantee capacity for each stage. The shared pool
		// additionally subscribes to core/enrichment queues to provide elastic
		// borrowing while post-process and maintenance remain hard-isolated.
		must(container.Provide(router.NewCoreAsynqServer, dig.Name("coreAsynqServer")))
		must(container.Provide(router.NewPostProcessAsynqServer, dig.Name("postProcessAsynqServer")))
		must(container.Provide(router.NewEnrichmentAsynqServer, dig.Name("enrichmentAsynqServer")))
		must(container.Provide(router.NewMaintenanceAsynqServer, dig.Name("maintenanceAsynqServer")))
		must(container.Provide(router.NewSharedAsynqServer, dig.Name("sharedAsynqServer")))
		must(container.Provide(router.NewWikiAsynqServer, dig.Name("wikiAsynqServer")))
		// Asynq inspector for cancel-by-knowledge-id (best-effort
		// dequeue of pending/scheduled/retry tasks + active-task cancel).
		must(container.Provide(router.NewAsynqInspector))
		must(container.Provide(router.NewAsynqTaskInspector))
		// Install the distributed per-model chat concurrency governor. Only
		// available with Redis (the shared semaphore backend); without Redis
		// the deployment is single-process and low-volume, so it runs ungated.
		must(container.Invoke(registerModelConcurrencyLimiter))
	} else {
		syncExec := router.NewSyncTaskExecutor()
		must(container.Provide(func() interfaces.TaskEnqueuer { return syncExec }))
		must(container.Provide(func() *router.SyncTaskExecutor { return syncExec }))
		// No Redis means no asynq inspector. SyncTaskExecutor dispatches
		// inline goroutines that the checkpoint-based abort already handles.
		must(container.Provide(router.NewNoopTaskInspector))
		// Even without Redis, background ingestion/enrichment can burst the
		// worker pool against one provider, so install an in-process governor.
		must(container.Invoke(registerLocalModelConcurrencyLimiter))
	}
	must(container.Provide(service.NewTemporaryDocumentService))
	must(container.Invoke(startTemporaryDocumentCleanup))

	// Chat pipeline components for processing chat requests
	logger.Debugf(ctx, "[Container] Registering chat pipeline plugins...")

	// Data source sync framework
	logger.Debugf(ctx, "[Container] Registering data source sync framework...")
	must(container.Provide(initConnectorRegistry))
	must(container.Provide(datasource.NewScheduler))
	must(container.Provide(service.NewDataSourceService))
	must(container.Invoke(startDataSourceScheduler))
	logger.Debugf(ctx, "[Container] Data source sync framework registered")
	must(container.Invoke(startAuditLogRetention))
	logger.Debugf(ctx, "[Container] Audit log retention runner registered")
	must(container.Provide(service.NewHousekeepingService))
	must(container.Invoke(startHousekeepingService))
	logger.Debugf(ctx, "[Container] Knowledge housekeeping runner registered")
	must(container.Provide(chatpipeline.NewEventManager))
	must(container.Invoke(chatpipeline.NewPluginSearch))
	must(container.Invoke(chatpipeline.NewPluginRerank))
	must(container.Invoke(chatpipeline.NewPluginWebFetch))
	must(container.Invoke(chatpipeline.NewPluginMerge))
	must(container.Invoke(chatpipeline.NewPluginIntoChatMessage))
	must(container.Invoke(chatpipeline.NewPluginChatCompletion))
	must(container.Invoke(chatpipeline.NewPluginChatCompletionStream))
	must(container.Invoke(chatpipeline.NewPluginFilterTopK))
	must(container.Invoke(chatpipeline.NewPluginQueryUnderstand))
	must(container.Invoke(chatpipeline.NewPluginLoadHistory))
	must(container.Invoke(chatpipeline.NewPluginExtractEntity))
	must(container.Invoke(chatpipeline.NewPluginSearchEntity))
	must(container.Invoke(chatpipeline.NewPluginSearchParallel))
	must(container.Invoke(chatpipeline.NewPluginWikiBoost))
	logger.Debugf(ctx, "[Container] Chat pipeline plugins registered")

	// HTTP handlers layer
	logger.Debugf(ctx, "[Container] Registering HTTP handlers...")
	must(container.Provide(docs.NewModule))
	must(container.Provide(handler.NewTenantHandler))
	must(container.Provide(handler.NewTenantMemberHandler))
	must(container.Provide(handler.NewTenantInvitationHandler))
	must(container.Provide(handler.NewAuditLogHandler))
	must(container.Provide(handler.NewKnowledgeBaseHandler))
	must(container.Provide(handler.NewKnowledgeHandler))
	must(container.Provide(handler.NewChunkHandler))
	must(container.Provide(handler.NewFAQHandler))
	must(container.Provide(handler.NewTagHandler))
	must(container.Provide(session.NewHandler))
	must(container.Provide(handler.NewMessageHandler))
	must(container.Provide(handler.NewMessageSuggestionHandler))
	must(container.Provide(handler.NewModelHandler))
	must(container.Provide(handler.NewEvaluationHandler))
	must(container.Provide(handler.NewInitializationHandler))
	must(container.Provide(handler.NewAuthHandler))
	must(container.Provide(handler.NewSystemHandler))
	must(container.Provide(handler.NewModelCredentialsHandler))
	must(container.Provide(handler.NewWebSearchProviderCredentialsHandler))
	must(container.Provide(handler.NewDataSourceCredentialsHandler))
	must(container.Provide(handler.NewWebSearchHandler))
	must(container.Provide(handler.NewWebSearchProviderHandler))
	must(container.Provide(handler.NewVectorStoreHandler))
	must(container.Provide(handler.NewStorageBackendHandler))
	must(container.Provide(handler.NewUserResourceFavoriteHandler))
	must(container.Provide(handler.NewOrganizationHandler))

	// Data source handler
	must(container.Provide(handler.NewDataSourceHandler))
	// Wiki page handler
	must(container.Provide(handler.NewWikiPageHandler))
	must(container.Provide(handler.NewKnowledgeFindingHandler))
	must(container.Provide(handler.NewKnowledgeStewardshipHandler))
	logger.Debugf(ctx, "[Container] HTTP handlers registered")

	// Wire the chat package's local image resolver so multimodal chat can read
	// local:// images that live under a tenant's configured storage PathPrefix
	// (which is not encoded in the local:// URL).
	must(container.Invoke(registerChatLocalImageResolver))

	// Extensions. The core provides a default for what an extension may add, then
	// lets registered hooks add providers and decorate those defaults. This has to
	// come after every provider is registered (a hook may decorate any of them)
	// and before the first Invoke that resolves them, which is the router below.
	logger.Debugf(ctx, "[Container] Applying extensions...")
	must(container.Provide(extension.NewFeatures))
	must(extension.ApplyHooks(container))

	// Router configuration
	logger.Debugf(ctx, "[Container] Registering router and starting task server...")
	must(container.Provide(router.NewRouter))
	if redisAvailable {
		must(container.Invoke(router.RunAsynqServer))
	} else {
		must(container.Invoke(router.RegisterSyncHandlers))
	}
	// Wiki operation rows are durable, while their wake-up triggers may be
	// lost across a process restart (always without Redis, and in Redis mode if
	// persistence succeeded immediately before trigger enqueue failed). Re-arm
	// them only after the matching handlers are ready.
	must(container.Invoke(recoverPendingWikiTasks))

	logger.Infof(ctx, "[Container] Container initialization completed successfully")
	return container
}

// registerChatLocalImageResolver wires the chat package's LocalImageResolver
// hook. Stored local:// URLs are relative to the resolved storage base dir and
// do NOT encode the owning tenant's configured PathPrefix, so resolving them to
// disk bytes requires rebuilding the FileService from that tenant's storage
// config. The owning tenant is parsed from the URL's first path segment, which
// correctly handles cross-tenant shared resources (e.g. shared KB images).
func registerChatLocalImageResolver(
	tenantRepo interfaces.TenantRepository,
	storageResolver interfaces.StorageBackendResolver,
	resourceCatalog interfaces.ResourceCatalog,
) {
	chat.LocalImageResolver = func(storageURL string) ([]byte, bool) {
		ctx := context.Background()
		physicalPath, resource, err := resourceCatalog.ResolvePath(ctx, storageURL)
		if err != nil {
			return nil, false
		}
		tenantID := secutils.ParseTenantIDFromStoragePath(physicalPath)
		if resource != nil {
			tenantID = resource.TenantID
		}
		if tenantID == 0 {
			return nil, false
		}
		tenant, err := tenantRepo.GetTenantByID(ctx, tenantID)
		if err != nil || tenant == nil {
			return nil, false
		}
		baseDir := strings.TrimSpace(os.Getenv("LOCAL_STORAGE_BASE_DIR"))
		backendID, inner, scoped := types.ParseStorageBackendPath(physicalPath)
		if resource != nil && resource.StorageBackendID != "" {
			backendID = resource.StorageBackendID
		}
		providerPath := physicalPath
		if scoped {
			providerPath = inner
		}
		provider := types.ParseProviderScheme(providerPath)
		if provider == "" {
			provider = "local"
		}
		fileSvc, _, err := storageResolver.ResolveFileService(ctx, tenant, backendID, provider, baseDir)
		if err != nil {
			return nil, false
		}
		rc, err := fileSvc.GetFile(ctx, physicalPath)
		if err != nil {
			return nil, false
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, false
		}
		return data, true
	}
}

// must is a helper function for error handling
// Panics if the error is not nil, useful for configuration steps that must succeed
// Parameters:
//   - err: Error to check
func must(err error) {
	if err != nil {
		panic(err)
	}
}

// initLangfuse initializes the Langfuse ingestion client.
// Configuration is read from LANGFUSE_* environment variables (see
// docs/langfuse.md). Returns a disabled manager if credentials are absent —
// never an error — so deployments that don't use Langfuse are unaffected.
func initLangfuse() (*langfuse.Manager, error) {
	cfg := langfuse.LoadConfigFromEnv()
	return langfuse.Init(cfg)
}

// defaultModelMaxConcurrency is the per-model cap on concurrent background
// (ingestion/enrichment) chat calls when YUHENG_MODEL_MAX_CONCURRENCY /
// model.max_concurrency is unset. summary / question / graph enrichment all
// share the same model, so this bounds their combined pressure on one provider
// across every replica. Interactive chat is never gated.
const defaultModelMaxConcurrency = 32

// resolveModelMaxConcurrency reads the per-model background concurrency limit
// from system settings / env, defaulting to defaultModelMaxConcurrency when
// unset. A configured value of 0 (or negative) is honoured and disables the
// governor — that is the supported way to turn throttling off via config/env.
func resolveModelMaxConcurrency(ss interfaces.SystemSettingService) int {
	if ss == nil {
		return defaultModelMaxConcurrency
	}
	return int(ss.GetInt(context.Background(), "model.max_concurrency",
		"YUHENG_MODEL_MAX_CONCURRENCY", int64(defaultModelMaxConcurrency)))
}

// registerModelConcurrencyLimiter builds the Redis-backed per-model background
// concurrency governor (chat + vlm) and installs it. Only available with Redis
// (the shared semaphore backend); no-Redis mode uses registerLocalModelConcurrencyLimiter.
func registerModelConcurrencyLimiter(rdb *redis.Client, ss interfaces.SystemSettingService) {
	limit := resolveModelMaxConcurrency(ss)
	limiter.SetGovernor(limiter.NewRedisLimiter(rdb), limit)
	if limit <= 0 {
		logger.Infof(context.Background(),
			"[ModelLimiter] background concurrency governor DISABLED (model.max_concurrency<=0)")
		return
	}
	logger.Infof(context.Background(),
		"[ModelLimiter] background model concurrency governed per-model, limit=%d (distributed via redis)", limit)
}

// registerLocalModelConcurrencyLimiter installs an in-process per-model governor
// for no-Redis deployments. A single process runs, so an in-process semaphore is
// sufficient to keep a background ingestion storm from bursting the whole worker
// pool against one provider.
func registerLocalModelConcurrencyLimiter(ss interfaces.SystemSettingService) {
	limit := resolveModelMaxConcurrency(ss)
	limiter.SetGovernor(limiter.NewLocalLimiter(), limit)
	if limit <= 0 {
		logger.Infof(context.Background(),
			"[ModelLimiter] background concurrency governor DISABLED (model.max_concurrency<=0)")
		return
	}
	logger.Infof(context.Background(),
		"[ModelLimiter] background model concurrency governed per-model, limit=%d (in-process, no Redis)", limit)
}

func initRedisClient() (*redis.Client, error) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		logger.Warnf(context.Background(), "[Redis] No REDIS_ADDR configured: running in single-process mode. "+
			"Events, caches and background tasks are not shared, so run exactly one instance of the application")
		return nil, nil
	}
	db, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		db = 0
	}

	client := redis.NewClient(&redis.Options{
		Addr:      redisAddr,
		Username:  os.Getenv("REDIS_USERNAME"),
		Password:  os.Getenv("REDIS_PASSWORD"),
		DB:        db,
		TLSConfig: common.RedisTLSConfig(),
	})

	_, err = client.Ping(context.Background()).Result()
	if err != nil {
		return nil, fmt.Errorf("连接Redis失败: %w", err)
	}

	return client, nil
}

// migrationStartupError wraps a migration failure with what the operator can do
// about it. The message goes to the fatal log line that ends the process, so it
// has to stand on its own.
func migrationStartupError(err error) error {
	where := "no version was recorded"
	if v, dirty, ok := database.CachedMigrationVersion(); ok {
		where = fmt.Sprintf("the database is at version %d (dirty: %v)", v, dirty)
	}
	return fmt.Errorf(
		"database migration failed, refusing to start against a half-migrated schema (%s): %w\n\n"+
			"Next steps: run ./scripts/migrate.sh version to see the state, fix the failing migration "+
			"(or restore your pre-upgrade backup) and start again. See docs/migration-troubleshooting.md.\n"+
			"If you migrate out of band and want the server to start anyway, set MIGRATION_FAIL_FAST=false "+
			"(or AUTO_MIGRATE=false to skip migrations entirely)",
		where, err)
}

// initDatabase initializes database connection
// Creates and configures database connection based on environment configuration
// Supports multiple database backends (PostgreSQL)
// Parameters:
//   - cfg: Application configuration
//
// Returns:
//   - Configured database connection
//   - Error if connection fails
func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	var migrateDSN string
	// builtinRetrieval is set when the built-in engine keeps its index in this
	// database, which needs PostgreSQL extensions beyond the stock server.
	var builtinRetrieval bool
	switch os.Getenv("DB_DRIVER") {
	case "postgres":
		// DSN for GORM (key-value format)
		gormDSN := fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
			os.Getenv("DB_HOST"),
			os.Getenv("DB_PORT"),
			os.Getenv("DB_USER"),
			os.Getenv("DB_PASSWORD"),
			os.Getenv("DB_NAME"),
			"disable",
		)
		dialector = postgres.Open(gormDSN)

		// DSN for golang-migrate (URL format)
		// URL-encode password to handle special characters like !@#
		dbPassword := os.Getenv("DB_PASSWORD")
		encodedPassword := url.QueryEscape(dbPassword)

		// Check if postgres is in RETRIEVE_DRIVER to determine skip_embedding
		retrieveDriver := strings.Split(os.Getenv("RETRIEVE_DRIVER"), ",")
		skipEmbedding := "true"
		if slices.Contains(retrieveDriver, "postgres") {
			skipEmbedding = "false"
			builtinRetrieval = true
		}
		logger.Infof(context.Background(), "Skip embedding: %s", skipEmbedding)

		migrateDSN = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=disable&options=-c%%20app.skip_embedding=%s",
			os.Getenv("DB_USER"),
			encodedPassword, // Use encoded password
			os.Getenv("DB_HOST"),
			os.Getenv("DB_PORT"),
			os.Getenv("DB_NAME"),
			skipEmbedding,
		)

		// Debug log (don't log password)
		logger.Infof(context.Background(), "DB Config: user=%s host=%s port=%s dbname=%s",
			os.Getenv("DB_USER"),
			os.Getenv("DB_HOST"),
			os.Getenv("DB_PORT"),
			os.Getenv("DB_NAME"),
		)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", os.Getenv("DB_DRIVER"))
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, err
	}

	// Sanity check: the rest of the code assumes PostgreSQL — its SQL, locks,
	// sequences and JSONB queries — and has no path for any other dialect. A
	// future driver swap that reports a different name (e.g. a wrapper dialect
	// for managed PG) would therefore run Postgres SQL against something that
	// may not accept it. Refusing to start is loud and inexpensive.
	if name := db.Dialector.Name(); name != "postgres" {
		return nil, fmt.Errorf(
			"unsupported gorm dialector %q; expected postgres "+
				"(the application has no code path for any other dialect)", name)
	}

	pool, poolWarning, err := database.PoolConfigFromEnv()
	if err != nil {
		return nil, err
	}
	if poolWarning != "" {
		logger.Warnf(context.Background(), "%s", poolWarning)
	}
	logger.Infof(context.Background(), "DB pool: max_open=%d max_idle=%d conn_max_lifetime=%s",
		pool.MaxOpenConns, pool.MaxIdleConns, pool.ConnMaxLifetime)

	// The embeddings migration creates the extensions the built-in engine
	// needs, and a server without them fails it with an error that says nothing
	// about what to do; refuse to start with a message that does.
	if builtinRetrieval {
		if err := requireRetrievalExtensions(context.Background(), db); err != nil {
			return nil, err
		}
	}

	// Run database migrations automatically (optional, can be disabled via env var)
	// To disable auto-migration, set AUTO_MIGRATE=false (migrate externally with
	// scripts/migrate.sh). A failed migration stops start-up unless
	// MIGRATION_FAIL_FAST=false. AUTO_RECOVER_DIRTY=true opts in to retrying an
	// interrupted migration automatically; see database.MigrationOptions for why
	// that is off by default.
	if os.Getenv("AUTO_MIGRATE") != "false" {
		logger.Infof(context.Background(), "Running database migrations...")

		migrationOpts := database.MigrationOptions{
			AutoRecoverDirty: os.Getenv("AUTO_RECOVER_DIRTY") == "true",
		}

		// Run base migrations (all versioned migrations including embeddings), then any
		// extension sources. The embeddings migration will be conditionally executed based
		// on skip_embedding parameter in DSN
		if err := database.RunMigrationsWithOptions(migrateDSN, migrationOpts); err != nil {
			if os.Getenv("MIGRATION_FAIL_FAST") == "false" {
				// The operator migrates the schema out of band and accepts a server that
				// starts against whatever state that left. /ready stays 503 while the
				// failure is on record, so an orchestrator will not route traffic here.
				logger.Warnf(context.Background(), "Database migration failed: %v", err)
				logger.Warnf(context.Background(),
					"Continuing with application startup because MIGRATION_FAIL_FAST=false. "+
						"Run ./scripts/migrate.sh up to migrate manually.")
			} else {
				return nil, migrationStartupError(err)
			}
		}

		// Post-migration: resolve __pending_env__ storage provider markers for historical KBs.
		// The SQL migration marks KBs that have documents but no provider with "__pending_env__";
		// we replace that with the actual STORAGE_TYPE from the environment.
		resolveStorageProviderPending(db)
		migrateLegacyStorageBackends(db)

		// Post-migration: declarative built-in models from config/builtin_models.yaml (optional).
		if err := types.LoadBuiltinModelsConfig(context.Background(), db, config.ConfigDir()); err != nil {
			logger.Warnf(context.Background(), "Load builtin models config failed: %v", err)
		}
	} else {
		logger.Infof(context.Background(), "Auto-migration is disabled (AUTO_MIGRATE=false)")
	}

	// Get underlying SQL DB object
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Configure connection pool parameters (DB_MAX_OPEN_CONNS et al., validated above).
	sqlDB.SetMaxOpenConns(pool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(pool.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(pool.ConnMaxLifetime)

	return db, nil
}

// resolveStorageProviderPending replaces the "__pending_env__" sentinel in
// knowledge_bases.storage_provider_config with the actual STORAGE_TYPE from the environment.
// This runs once after SQL migrations to bind historical KBs to their real storage provider.
func resolveStorageProviderPending(db *gorm.DB) {
	storageType := strings.TrimSpace(os.Getenv("STORAGE_TYPE"))
	if storageType == "" {
		storageType = "local"
	}
	storageType = strings.ToLower(storageType)

	result := db.Exec(
		`UPDATE knowledge_bases SET storage_provider_config = ? WHERE storage_provider_config IS NOT NULL AND storage_provider_config->>'provider' = '__pending_env__'`,
		fmt.Sprintf(`{"provider":"%s"}`, storageType),
	)
	if result.Error != nil {
		logger.Warnf(context.Background(), "Failed to resolve __pending_env__ storage providers: %v", result.Error)
	} else if result.RowsAffected > 0 {
		logger.Infof(context.Background(), "Resolved %d knowledge bases with __pending_env__ storage provider → %s", result.RowsAffected, storageType)
	}

	// Sync PostgreSQL sequences with actual MAX values to prevent duplicate key
	// errors. The old code assigned seq_id via SELECT MAX()+1 in application
	// code, which could push values past the DB sequence counter.
	syncSequences(db)

	// Reset any pending tasks left over from previous aborted runs (no-Redis mode)
	resetPendingTasks(db)
}

// migrateLegacyStorageBackends backfills the storage_backends table from each
// workspace's legacy StorageEngineConfig (or environment defaults) and binds
// existing knowledge bases to the resulting backend.
//
// The table, columns and indexes are created by the SQL migrations
// (migration 000068_storage_backends); this step only handles data that
// cannot be expressed portably in
// SQL: environment snapshots, JSON→config mapping, AES-encrypted credentials,
// UUID generation and the per-startup refresh of env-backed aliases.
// The migration is idempotent: one legacy_alias row per tenant/provider.
func migrateLegacyStorageBackends(db *gorm.DB) {
	var tenants []*types.Tenant
	if err := db.Find(&tenants).Error; err != nil {
		logger.Warnf(context.Background(), "Failed to load workspaces for storage backend migration: %v", err)
		return
	}
	if len(tenants) == 0 {
		return
	}

	// Load every alias in a single query. Probing each tenant/provider pair with
	// First() makes GORM log "record not found" for every miss, which floods the
	// startup log with workspaces × providers lines on fresh installs.
	var aliases []*types.StorageBackend
	if err := db.Where("legacy_alias = ?", true).Find(&aliases).Error; err != nil {
		logger.Warnf(context.Background(), "Failed to load legacy storage aliases: %v", err)
		return
	}
	existingAliases := make(map[uint64]map[string]*types.StorageBackend, len(aliases))
	for _, alias := range aliases {
		byProvider := existingAliases[alias.TenantID]
		if byProvider == nil {
			byProvider = make(map[string]*types.StorageBackend)
			existingAliases[alias.TenantID] = byProvider
		}
		byProvider[alias.Provider] = alias
	}

	for _, tenant := range tenants {
		legacy := tenant.StorageEngineConfig
		defaultProvider := ""
		if legacy != nil {
			defaultProvider = strings.ToLower(strings.TrimSpace(legacy.DefaultProvider))
		}
		if defaultProvider == "" {
			defaultProvider = strings.ToLower(strings.TrimSpace(os.Getenv("STORAGE_TYPE")))
		}
		if defaultProvider == "" {
			defaultProvider = "local"
		}

		backendIDs := make(map[string]string)
		for _, provider := range storageallowlist.Supported() {
			if existing := existingAliases[tenant.ID][provider]; existing != nil {
				// Environment-backed aliases are snapshots, not user-owned config.
				// Refresh them at every startup so credential rotation does not
				// leave the persisted resolver on stale values. If the workspace
				// later gains an explicit legacy config, promote the alias to user
				// source and stop automatic refreshes.
				if existing.Source == types.StorageBackendSourceEnv {
					desired := types.StorageBackendFromLegacy(tenant.ID, provider, legacy)
					if desired == nil && provider == defaultProvider {
						desired = types.StorageBackendFromEnvironment(tenant.ID)
					}
					if desired != nil && desired.Provider == provider {
						_ = db.Model(&types.StorageBackend{}).Where("id = ?", existing.ID).Updates(map[string]interface{}{
							"name": desired.Name, "config": desired.Config, "source": desired.Source, "status": desired.Status, "updated_at": time.Now(),
						}).Error
					}
				}
				backendIDs[provider] = existing.ID
				continue
			}
			backend := types.StorageBackendFromLegacy(tenant.ID, provider, legacy)
			if backend == nil && provider == defaultProvider {
				backend = types.StorageBackendFromEnvironment(tenant.ID)
			}
			if backend == nil {
				continue
			}
			if err := db.Create(backend).Error; err != nil {
				logger.Warnf(context.Background(), "Failed to migrate %s storage for workspace %d: %v", provider, tenant.ID, err)
				continue
			}
			backendIDs[provider] = backend.ID
		}
		if tenant.DefaultStorageBackendID == nil {
			if id := backendIDs[defaultProvider]; id != "" {
				if err := db.Model(&types.Tenant{}).Where("id = ?", tenant.ID).Update("default_storage_backend_id", id).Error; err != nil {
					logger.Warnf(context.Background(), "Failed to set default storage backend for workspace %d: %v", tenant.ID, err)
				}
			}
		}

		var kbs []*types.KnowledgeBase
		if err := db.Where("tenant_id = ? AND storage_backend_id IS NULL", tenant.ID).Find(&kbs).Error; err != nil {
			continue
		}
		for _, kb := range kbs {
			provider := kb.GetStorageProvider()
			if provider == "" {
				provider = defaultProvider
			}
			if id := backendIDs[provider]; id != "" {
				_ = db.Model(&types.KnowledgeBase{}).Where("id = ? AND storage_backend_id IS NULL", kb.ID).Update("storage_backend_id", id).Error
			}
		}
	}
}

// syncSequences ensures PostgreSQL sequences for auto-increment columns (seq_id)
// are at least as high as the current MAX value in each table. This is needed
// because older code assigned seq_id via application-level MAX()+1, which could
// advance values past the DB sequence counter and cause duplicate key errors.
func syncSequences(db *gorm.DB) {
	pairs := [][2]string{
		{"chunks", "chunks_seq_id_seq"},
		{"knowledge_tags", "knowledge_tags_seq_id_seq"},
	}
	for _, p := range pairs {
		table, seq := p[0], p[1]
		sql := fmt.Sprintf(
			`SELECT setval('%s', GREATEST(nextval('%s'), (SELECT COALESCE(MAX(seq_id), 0) FROM %s)))`,
			seq, seq, table,
		)
		if err := db.Exec(sql).Error; err != nil {
			logger.Warnf(context.Background(), "Failed to sync sequence %s: %v", seq, err)
		} else {
			logger.Infof(context.Background(), "Synced sequence %s with table %s", seq, table)
		}
	}
}

// initFileService initializes file storage service
// Creates the appropriate file storage service based on configuration
// Supports the local filesystem and any S3-compatible object storage.
// Parameters:
//   - cfg: Application configuration
//
// Returns:
//   - Configured file service implementation
//   - Error if initialization fails
func initFileService(cfg *config.Config, catalog interfaces.ResourceCatalog) (interfaces.FileService, error) {
	inner, err := initRawFileService(cfg)
	if err != nil {
		return nil, err
	}
	return file.NewResourceCatalogFileService(inner, catalog), nil
}

func initRawFileService(_ *config.Config) (interfaces.FileService, error) {
	storageType := strings.TrimSpace(os.Getenv("STORAGE_TYPE"))
	if storageType == "" {
		storageType = "local"
	}
	switch storageType {
	case types.StorageProviderS3:
		accessKey, secretKey := os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY")
		if os.Getenv("S3_REGION") == "" ||
			os.Getenv("S3_BUCKET_NAME") == "" ||
			(accessKey == "") != (secretKey == "") {
			return nil, fmt.Errorf("missing S3 configuration")
		}
		pathPrefix := os.Getenv("S3_PATH_PREFIX")
		if pathPrefix == "" {
			pathPrefix = "yuheng/"
		}
		return file.NewS3FileService(file.S3Options{
			Endpoint:        os.Getenv("S3_ENDPOINT"),
			Region:          os.Getenv("S3_REGION"),
			AccessKey:       accessKey,
			SecretKey:       secretKey,
			BucketName:      os.Getenv("S3_BUCKET_NAME"),
			PathPrefix:      pathPrefix,
			UseSSL:          !strings.EqualFold(os.Getenv("S3_USE_SSL"), "false"),
			AddressingStyle: strings.ToLower(strings.TrimSpace(os.Getenv("S3_ADDRESSING_STYLE"))),
		})
	case types.StorageProviderLocal:
		baseDir := os.Getenv("LOCAL_STORAGE_BASE_DIR")
		if baseDir == "" {
			baseDir = "/data/files"
		}
		externalURL := strings.TrimSpace(os.Getenv("APP_EXTERNAL_URL"))
		return file.NewLocalFileService(baseDir, externalURL), nil
	case "dummy":
		return file.NewDummyFileService(), nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}
}

// initAntsPool initializes the goroutine pool
// Creates a managed goroutine pool for concurrent task execution
// Parameters:
//   - cfg: Application configuration
//
// Returns:
//   - Configured goroutine pool
//   - Error if initialization fails
func initAntsPool(cfg *config.Config) (*ants.Pool, error) {
	// Default to 5 if not specified in config
	poolSize := os.Getenv("CONCURRENCY_POOL_SIZE")
	if poolSize == "" {
		poolSize = "5"
	}
	poolSizeInt, err := strconv.Atoi(poolSize)
	if err != nil {
		return nil, err
	}
	// Set up the pool with pre-allocation for better performance
	return ants.NewPool(poolSizeInt, ants.WithPreAlloc(true))
}

// registerPoolCleanup registers the goroutine pool for cleanup
// Ensures proper cleanup of the goroutine pool when application shuts down
// Parameters:
//   - pool: Goroutine pool
//   - cleaner: Resource cleaner
func registerPoolCleanup(pool *ants.Pool, cleaner interfaces.ResourceCleaner) {
	cleaner.RegisterWithName("AntsPool", func() error {
		pool.Release()
		return nil
	})
}

// registerLangfuseCleanup ensures buffered Langfuse events are flushed on
// shutdown. A 5-second timeout matches other external-service cleanups and
// balances data durability against a slow remote endpoint holding up exit.
func registerLangfuseCleanup(mgr *langfuse.Manager, cleaner interfaces.ResourceCleaner) {
	if mgr == nil {
		return
	}
	cleaner.RegisterWithName("Langfuse", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return mgr.Shutdown(ctx)
	})
}

// initDocReaderClient initializes the DocumentReader client (lightweight API).
func initDocReaderClient(cfg *config.Config) (interfaces.DocumentReader, error) {
	addr := strings.TrimSpace(os.Getenv("DOCREADER_ADDR"))
	transport := strings.TrimSpace(os.Getenv("DOCREADER_TRANSPORT"))
	if transport == "" {
		transport = "grpc"
	}
	if addr == "" {
		logger.Infof(context.Background(), "[DocConverter] No DOCREADER_ADDR configured, starting disconnected")
	}
	transport = strings.ToLower(transport)
	switch transport {
	case "http", "https":
		if addr != "" && !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
			addr = "http://" + addr
		}
		return docparser.NewHTTPDocumentReader(addr)
	default:
		return docparser.NewGRPCDocumentReader(addr)
	}
}

// initOllamaService initializes the Ollama service client
// Creates a client for interacting with Ollama API for model inference
// Parameters:
//   - None
//
// Returns:
//   - Configured Ollama service client
//   - Error if initialization fails
func initOllamaService() (*ollama.OllamaService, error) {
	// Get Ollama service from existing factory function
	return ollama.GetOllamaService()
}

func initNeo4jClient() (neo4j.Driver, error) {
	ctx := context.Background()
	if strings.ToLower(os.Getenv("NEO4J_ENABLE")) != "true" {
		logger.Debugf(ctx, "NOT SUPPORT RETRIEVE GRAPH")
		return nil, nil
	}
	uri := os.Getenv("NEO4J_URI")
	username := os.Getenv("NEO4J_USERNAME")
	password := os.Getenv("NEO4J_PASSWORD")

	// Retry configuration
	maxRetries := 30                 // Max retry attempts
	retryInterval := 2 * time.Second // Wait between retries

	var driver neo4j.Driver
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		driver, err = neo4j.NewDriver(uri, neo4j.BasicAuth(username, password, ""))
		if err != nil {
			logger.Warnf(ctx, "Failed to create Neo4j driver (attempt %d/%d): %v", attempt, maxRetries, err)
			time.Sleep(retryInterval)
			continue
		}

		err = driver.VerifyAuthentication(ctx, nil)
		if err == nil {
			if attempt > 1 {
				logger.Infof(ctx, "Successfully connected to Neo4j after %d attempts", attempt)
			}
			return driver, nil
		}

		logger.Warnf(ctx, "Failed to verify Neo4j authentication (attempt %d/%d): %v", attempt, maxRetries, err)
		driver.Close(ctx)
		time.Sleep(retryInterval)
	}

	return nil, fmt.Errorf("failed to connect to Neo4j after %d attempts: %w", maxRetries, err)
}

func NewDuckDB() (*sql.DB, error) {
	sqlDB, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to open duckdb: %w", err)
	}

	// Try to install and load required extensions unless explicitly disabled.
	//   - spatial: used for st_read_meta() to enumerate layer (sheet) names from .xlsx/.xls
	//   - excel:   used for read_xlsx() which gives proper type inference per sheet
	//
	// INSTALL hits extensions.duckdb.org (public internet). In locked-down
	// runtimes with no egress, set DUCKDB_SKIP_EXTENSION_LOAD=1 to avoid a
	// startup hang; xlsx/xls ingest may fail later without these extensions.
	if strings.EqualFold(os.Getenv("DUCKDB_SKIP_EXTENSION_LOAD"), "true") ||
		os.Getenv("DUCKDB_SKIP_EXTENSION_LOAD") == "1" {
		logger.Infof(context.Background(),
			"[DuckDB] Skipping spatial/excel extension install/load "+
				"(DUCKDB_SKIP_EXTENSION_LOAD is set; xlsx ingest may fail without them)")
	} else {
		bgCtx := context.Background()
		for _, ext := range []string{"spatial", "excel"} {
			if _, err := sqlDB.ExecContext(bgCtx, fmt.Sprintf("INSTALL %s;", ext)); err != nil {
				logger.Warnf(bgCtx, "[DuckDB] Failed to install %s extension: %v", ext, err)
			}
			if _, err := sqlDB.ExecContext(bgCtx, fmt.Sprintf("LOAD %s;", ext)); err != nil {
				logger.Warnf(bgCtx, "[DuckDB] Failed to load %s extension: %v", ext, err)
			}
		}
	}

	return sqlDB, nil
}

// registerWebSearchProviders registers all web search provider types to the registry.
// Each provider type is registered with its factory function that accepts parameters.
// Provider instances are created on-demand when tenants configure them.
func registerWebSearchProviders(registry *infra_web_search.Registry) {
	registry.Register("duckduckgo", infra_web_search.NewDuckDuckGoProvider)
	registry.Register("google", infra_web_search.NewGoogleProvider)
	registry.Register("bing", infra_web_search.NewBingProvider)
	registry.Register("tavily", infra_web_search.NewTavilyProvider)
	registry.Register("ollama", infra_web_search.NewOllamaProvider)
	registry.Register("baidu", infra_web_search.NewBaiduProvider)
	registry.Register("searxng", infra_web_search.NewSearxngProvider)
	registry.Register("keenable", infra_web_search.NewKeenableProvider)
	registry.Register("zhipu", infra_web_search.NewZhipuProvider)
	registry.Register("exa", infra_web_search.NewExaProvider)
	registry.Register("metaso", infra_web_search.NewMetasoProvider)
	registry.Register("firecrawl", infra_web_search.NewFirecrawlProvider)
}

// initConnectorRegistry creates and populates the connector registry with all available connectors.
// Aggregates registration errors via errors.Join so a misconfigured or duplicated connector fails
// container initialization loudly instead of silently disabling the feature at runtime.
func initConnectorRegistry() (*datasource.ConnectorRegistry, error) {
	registry := datasource.NewConnectorRegistry()

	var errs error
	if err := registry.Register(wiki.NewConnector(core.RegionFeishu)); err != nil {
		errs = errors.Join(errs, fmt.Errorf("register feishu connector: %w", err))
	}
	// Lark is Feishu's international cloud: same connector, different host/tenant.
	if err := registry.Register(wiki.NewConnector(core.RegionLark)); err != nil {
		errs = errors.Join(errs, fmt.Errorf("register lark connector: %w", err))
	}
	// Feishu/Lark Drive (云盘) mode: different connector type so the registry
	// dispatches to the Drive connector. Shares core.Client/Region/export logic
	// with the wiki connector. See 飞书云盘数据源设计.md / ADR-0001.
	if err := registry.Register(drive.NewDriveConnector(core.RegionFeishuDrive)); err != nil {
		errs = errors.Join(errs, fmt.Errorf("register feishu_drive connector: %w", err))
	}
	if err := registry.Register(drive.NewDriveConnector(core.RegionLarkDrive)); err != nil {
		errs = errors.Join(errs, fmt.Errorf("register lark_drive connector: %w", err))
	}
	if err := registry.Register(notionConnector.NewConnector()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("register notion connector: %w", err))
	}
	if err := registry.Register(yuqueConnector.NewConnector()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("register yuque connector: %w", err))
	}
	if err := registry.Register(imaConnector.NewConnector()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("register ima connector: %w", err))
	}
	if err := registry.Register(rssConnector.NewConnector()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("register rss connector: %w", err))
	}
	if err := registry.Register(gitlabConnector.NewConnector()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("register gitlab connector: %w", err))
	}

	// Future connectors will be registered here:
	// if err := registry.Register(confluenceConnector.NewConnector()); err != nil { ... }
	// if err := registry.Register(githubConnector.NewConnector()); err != nil { ... }

	if errs != nil {
		return nil, errs
	}
	return registry, nil
}

// startDataSourceScheduler starts the data source cron scheduler and registers cleanup.
func startDataSourceScheduler(scheduler *datasource.Scheduler, cleaner interfaces.ResourceCleaner) {
	if err := scheduler.Start(context.Background()); err != nil {
		logger.Warnf(context.Background(), "[Container] data source scheduler start failed: %v", err)
	}

	cleaner.RegisterWithName("DataSourceScheduler", func() error {
		scheduler.Stop()
		return nil
	})
}

// startHousekeepingService starts the knowledge housekeeping cron and registers
// cleanup. This is the safety net that recovers any knowledge stuck in
// "processing" past a configurable threshold (see HousekeepingService for
// rationale). Best-effort: a startup error is logged but does NOT abort the
// container — the rest of the system stays usable.
func startHousekeepingService(svc *service.HousekeepingService, cleaner interfaces.ResourceCleaner) {
	if svc == nil {
		return
	}
	if err := svc.Start(context.Background()); err != nil {
		logger.Warnf(context.Background(), "[Container] housekeeping start failed: %v", err)
	}
	cleaner.RegisterWithName("KnowledgeHousekeeping", func() error {
		svc.Stop()
		return nil
	})
}

// startTemporaryDocumentCleanup removes expired session attachments and their
// extracted images. The durable expiry timestamp is the source of truth; the
// ticker only controls how quickly storage is reclaimed.
func startTemporaryDocumentCleanup(svc interfaces.TemporaryDocumentService, cleaner interfaces.ResourceCleaner) {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := svc.CleanupExpired(context.Background()); err != nil {
					logger.Warnf(context.Background(), "[TemporaryDocument] cleanup failed: %v", err)
				}
			case <-stop:
				return
			}
		}
	}()
	cleaner.RegisterWithName("TemporaryDocumentCleanup", func() error {
		close(stop)
		return nil
	})
}

// startAuditLogRetention spins up the daily audit_logs purge sweep
// and registers shutdown cleanup. Mirrors the data-source-scheduler
// pattern: container init kicks the goroutine, ResourceCleaner stops
// it during graceful shutdown so a SIGTERM during a sweep doesn't
// orphan the goroutine.
//
// retention_days <= 0 is the configured way to disable retention;
// the runner short-circuits Start() on that path so we don't need
// to gate the wiring here.
func startAuditLogRetention(
	runner *service.AuditLogRetentionRunner, cleaner interfaces.ResourceCleaner,
) {
	runner.Start(context.Background())
	cleaner.RegisterWithName("AuditLogRetentionRunner", func() error {
		runner.Stop()
		return nil
	})
}
