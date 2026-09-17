package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/handler"
)

// Models are shared infrastructure (LLM credentials, embeddings, rerankers);
// Viewer+ for reads, PlatformManaged for any mutation — Admin+ normally,
// SystemAdmin-only once governance.centralized_infra is on.
//
// Reads deliberately stay Viewer+ in both modes: the knowledge-base and
// agent editors list models so users can select a platform-provided one at
// point of use. dto.NewModelResponse strips credentials and base URLs from
// built-in rows, so a Viewer sees the capability surface, never the secret.
func RegisterModelRoutes(
	r *gin.RouterGroup,
	handler *handler.ModelHandler,
	credHandler *handler.ModelCredentialsHandler,
	g *rbacGuards,
) {
	// 模型路由组。空间级基础设施：仅完全访问（Owner）API key 可访问。
	models := g.apiKeyGroup(r.Group("/models"), apiKeyManageModels(apiKeyFullAccess()))
	{
		// 获取模型厂商列表 — Viewer+
		models.GET("/providers", g.Viewer(), handler.ListModelProviders)
		// 创建模型 — PlatformManaged
		models.POST("", g.PlatformManaged(), handler.CreateModel)
		// 获取模型列表 — Viewer+
		models.GET("", g.Viewer(), handler.ListModels)
		// 调试已保存模型会发起真实上游调用并产生费用 — PlatformManaged
		models.POST("/:id/debug", g.PlatformManaged(), handler.DebugModel)
		// 获取单个模型 — Viewer+
		models.GET("/:id", g.Viewer(), handler.GetModel)
		// 更新模型 — PlatformManaged；内置模型仍由服务层额外限定为 SystemAdmin。
		models.PUT("/:id", g.PlatformManaged(), handler.UpdateModel)
		// 删除模型 — PlatformManaged
		models.DELETE("/:id", g.PlatformManaged(), handler.DeleteModel)
		// Per-field credential subresource (see internal/handler/model_credentials.go) — PlatformManaged
		models.PUT("/:id/credentials", g.PlatformManaged(), credHandler.Put)
		models.DELETE("/:id/credentials/:field", g.PlatformManaged(), credHandler.DeleteField)
		// Platform sharing lives on its own subresource so an ordinary edit
		// can never toggle it by accident. The service layer enforces
		// SystemAdmin regardless of the guard here.
		models.PUT("/:id/sharing", g.PlatformManaged(), handler.UpdateModelSharing)
	}
}

// Sandbox configs are workspace infrastructure that hold provider credentials.
// Scoped API keys cannot safely receive partial authority over them yet because
// mutation can strand remote sandboxes.
func RegisterSandboxConfigRoutes(
	r *gin.RouterGroup,
	h *handler.SandboxConfigHandler,
	skills *handler.SandboxSkillHandler,
	g *rbacGuards,
) {
	configs := g.apiKeyGroup(r.Group("/sandbox-configs"), apiKeyFullAccess())
	{
		configs.GET("", g.Viewer(), h.List)
		configs.PUT("/workspace-policy", g.PlatformManaged(), h.SetWorkspacePolicy)
		configs.POST("/templates/query", g.PlatformManaged(), h.QueryTemplates)
		configs.POST("", g.PlatformManaged(), h.Create)
		configs.GET("/:id", g.Viewer(), h.Get)
		configs.PUT("/:id", g.PlatformManaged(), h.Update)
		configs.DELETE("/:id", g.PlatformManaged(), h.Delete)
		configs.GET("/:id/sandboxes", g.Admin(), h.Inventory)
		// Skills are Admin+ throughout, reads included: an upload drives a
		// root shell whose output is baked into the image every session of
		// this config boots, and the listing names what that image carries.
		// Writes additionally move to the platform under centralised mode.
		configs.GET("/:id/skills", g.Admin(), skills.List)
		configs.POST("/:id/skills", g.PlatformManaged(), skills.Upload)
		configs.GET("/:id/skills/:skillId", g.Admin(), skills.Get)
		configs.GET("/:id/skills/:skillId/files", g.Admin(), skills.ListFiles)
		configs.GET("/:id/skills/:skillId/files/content", g.Admin(), skills.GetFile)
		configs.PATCH("/:id/skills/:skillId", g.PlatformManaged(), skills.Patch)
		configs.DELETE("/:id/skills/:skillId", g.PlatformManaged(), skills.Delete)
		configs.GET("/:id/skills/:skillId/install-events", g.Admin(), skills.InstallEvents)
		configs.GET("/:id/skills/:skillId/transcript", g.Admin(), skills.InstallTranscript)
	}
}

// RegisterEvaluationRoutes registers evaluation endpoints. Running an
// evaluation drives LLM calls (cost) and reads from KBs across the
// tenant; gate to Admin+ until product asks for a finer-grained
// matrix.
func RegisterEvaluationRoutes(r *gin.RouterGroup, handler *handler.EvaluationHandler, g *rbacGuards) {
	evaluationRoutes := g.apiKeyGroup(r.Group("/evaluation"), apiKeyRunEvaluations(apiKeyFullAccess()))
	{
		evaluationRoutes.POST("", g.Admin(), handler.Evaluation)
		evaluationRoutes.GET("", g.Viewer(), handler.GetEvaluationResult)
	}
}

func RegisterInitializationRoutes(r *gin.RouterGroup, handler *handler.InitializationHandler, g *rbacGuards) {
	// 初始化接口
	// GetCurrentConfigByKB 是只读，Viewer+ 即可（KB 受限 key 可读其范围内的 KB）。
	g.apiKeyRoute(r, http.MethodGet, "/initialization/config/:kbId",
		apiKeyRetrieve(apiKeyFullAccess()), g.Viewer(), g.KBAccessRead("kbId"), handler.GetCurrentConfigByKB)
	// InitializeByKB / UpdateKBConfig 都是改 KB 的核心模型/storage 配置 —
	// 跟 PUT /knowledge-bases/:id 同等敏感，挂同款 OwnedKB 矩阵 + KBAccessWrite
	//（API-key 主体短路 Owned* 守卫，KB allow-list 只能靠 KBAccess 兜底）。
	g.apiKeyRoute(r, http.MethodPost, "/initialization/initialize/:kbId",
		apiKeyManageKnowledgeBases(apiKeyFullAccess()), g.OwnedKBOrAdminFromKbIDParam(), g.KBAccessWrite("kbId"), handler.InitializeByKB)
	g.apiKeyRoute(r, http.MethodPut, "/initialization/config/:kbId",
		apiKeyManageKnowledgeBases(apiKeyFullAccess()), g.OwnedKBOrAdminFromKbIDParam(), g.KBAccessWrite("kbId"), handler.UpdateKBConfig)

	// Ollama / 远程 API / 抽取等系统级检测/下载操作。这些不绑某个 KB，
	// 会改空间级模型配置或拉远端模型；JWT 侧只读探测 Viewer+、变更 Admin+。
	// 对 API key 均为空间级：full-access key 可用，scoped key 需要 manage_models。
	g.apiKeyRoute(r, http.MethodGet, "/initialization/ollama/status", apiKeyManageModels(apiKeyFullAccess()), g.Viewer(), handler.CheckOllamaStatus)
	g.apiKeyRoute(r, http.MethodGet, "/initialization/ollama/models", apiKeyManageModels(apiKeyFullAccess()), g.Viewer(), handler.ListOllamaModels)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/ollama/models/check", apiKeyManageModels(apiKeyFullAccess()), g.PlatformManaged(), handler.CheckOllamaModels)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/ollama/models/download", apiKeyManageModels(apiKeyFullAccess()), g.PlatformManaged(), handler.DownloadOllamaModel)
	g.apiKeyRoute(r, http.MethodGet, "/initialization/ollama/download/progress/:taskId", apiKeyManageModels(apiKeyFullAccess()), g.Viewer(), handler.GetDownloadProgress)
	g.apiKeyRoute(r, http.MethodGet, "/initialization/ollama/download/tasks", apiKeyManageModels(apiKeyFullAccess()), g.Viewer(), handler.ListDownloadTasks)

	// 远程 API 探测：都是「配置某个模型端点是否可用」的动作，会带着凭据打到上游，
	// 因此与模型写操作同权 —— PlatformManaged。
	g.apiKeyRoute(r, http.MethodPost, "/initialization/remote/check", apiKeyManageModels(apiKeyFullAccess()), g.PlatformManaged(), handler.CheckRemoteModel)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/embedding/test", apiKeyManageModels(apiKeyFullAccess()), g.PlatformManaged(), handler.TestEmbeddingModel)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/rerank/check", apiKeyManageModels(apiKeyFullAccess()), g.PlatformManaged(), handler.CheckRerankModel)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/asr/check", apiKeyManageModels(apiKeyFullAccess()), g.PlatformManaged(), handler.CheckASRModel)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/multimodal/test", apiKeyManageModels(apiKeyFullAccess()), g.PlatformManaged(), handler.TestMultimodalFunction)

	// extract/* 作用在用户内容上（抽取关系、打标），属于空间内的业务能力而非
	// 基础设施配置，保持 Admin+ 不随集中管控上收。
	g.apiKeyRoute(r, http.MethodPost, "/initialization/extract/text-relation", apiKeyManageModels(apiKeyFullAccess()), g.Admin(), handler.ExtractTextRelations)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/extract/fabri-tag", apiKeyManageModels(apiKeyFullAccess()), g.Admin(), handler.FabriTag)
	g.apiKeyRoute(r, http.MethodPost, "/initialization/extract/fabri-text", apiKeyManageModels(apiKeyFullAccess()), g.Admin(), handler.FabriText)
}

// RegisterMCPServiceRoutes registers MCP service routes.
//
// MCP services are tenant-level integrations (external tool servers); we
// gate reads to Viewer+ and any mutation/test to Admin+. Tool-approval
// resolution is also Admin+ since approving a pending tool call grants
// the agent permission to execute side-effecting external commands.
// Credential subresource writes are Admin+ as well since secrets are
// tenant-scoped.
func RegisterMCPServiceRoutes(
	r *gin.RouterGroup,
	handler *handler.MCPServiceHandler,
	credHandler *handler.MCPCredentialsHandler,
	oauthHandler *handler.MCPOAuthHandler,
	g *rbacGuards,
) {
	// MCP OAuth provider redirect. Registered OUTSIDE the /mcp-services group
	// to avoid a static-vs-":id" route conflict, and left unauthenticated
	// (allow-listed in middleware/auth.go) because the third-party browser
	// redirect carries no Yuheng bearer — the single-use state authenticates.
	r.GET("/mcp-oauth/callback", oauthHandler.Callback)

	mcpServices := g.apiKeyGroup(r.Group("/mcp-services"), apiKeyManageMCPServices(apiKeyFullAccess()))
	{
		// Create MCP service — PlatformManaged
		mcpServices.POST("", g.PlatformManaged(), handler.CreateMCPService)
		// List MCP services — Viewer+
		mcpServices.GET("", g.Viewer(), handler.ListMCPServices)
		// Get MCP service by ID — Viewer+
		mcpServices.GET("/:id", g.Viewer(), handler.GetMCPService)
		// Update MCP service — PlatformManaged
		mcpServices.PUT("/:id", g.PlatformManaged(), handler.UpdateMCPService)
		// Delete MCP service — PlatformManaged
		mcpServices.DELETE("/:id", g.PlatformManaged(), handler.DeleteMCPService)
		// Test MCP service connection — PlatformManaged (probes external infra)
		mcpServices.POST("/:id/test", g.PlatformManaged(), handler.TestMCPService)
		// Get MCP service tools — Viewer+
		mcpServices.GET("/:id/tools", g.Viewer(), handler.GetMCPServiceTools)
		// Get MCP service resources — Viewer+
		mcpServices.GET("/:id/resources", g.Viewer(), handler.GetMCPServiceResources)
		// Per-field credential subresource: secrets never travel via the main
		// PUT body. See internal/handler/mcp_credentials.go for the contract. — PlatformManaged
		mcpServices.PUT("/:id/credentials", g.PlatformManaged(), credHandler.Put)
		mcpServices.DELETE("/:id/credentials/:field", g.PlatformManaged(), credHandler.DeleteField)
		// MCP tool human approval (issue #1173) — Viewer+ to read; the policy
		// belongs to whoever owns the service, so writes follow it to the
		// platform under centralised mode.
		mcpServices.GET("/:id/tool-approvals", g.Viewer(), handler.ListMCPToolApprovals)
		mcpServices.PUT("/:id/tool-approvals/:tool_name", g.PlatformManaged(), handler.SetMCPToolApproval)
		// Per-user OAuth authorization flow. Viewer+ may authorize/inspect/
		// revoke their own token; the callback is the separate public route
		// registered above.
		mcpServices.POST("/:id/oauth/authorize-url", g.Viewer(), oauthHandler.AuthorizeURL)
		mcpServices.GET("/:id/oauth/status", g.Viewer(), oauthHandler.Status)
		mcpServices.DELETE("/:id/oauth/token", g.Viewer(), oauthHandler.Revoke)
	}

	// /agent tool-approval + OAuth resolution are interactive human flows;
	// not declared for API keys (default-deny).
	agentTool := r.Group("/agent")
	{
		// Resolving a pending tool-approval is gated to tenant members
		// (Viewer+). The approval card surfaces inside an agent chat the
		// caller initiated — restricting it to Admin+ blocks the only
		// people who actually have context to approve, so the gate is
		// kept at "anyone in the tenant" instead.
		agentTool.POST("/tool-approvals/:pending_id", g.Viewer(), handler.ResolveToolApproval)
		// Resume an agent run paused on an in-conversation MCP OAuth prompt.
		// Same tenant-member (Viewer+) gating rationale as tool-approvals.
		agentTool.POST("/mcp-oauth-resolutions/:pending_id", g.Viewer(), oauthHandler.ResolveMCPOAuth)
		agentTool.POST("/mcp-oauth-resolutions/:pending_id/cancel", g.Viewer(), oauthHandler.CancelMCPOAuth)
	}
}

// RegisterWebSearchRoutes registers web search routes
func RegisterWebSearchRoutes(r *gin.RouterGroup, webSearchHandler *handler.WebSearchHandler, g *rbacGuards) {
	// Web search providers — Viewer+ (read-only listing of provider catalog).
	webSearch := r.Group("/web-search")
	{
		webSearch.GET("/providers", g.Viewer(), webSearchHandler.GetProviders)
	}
}

// RegisterWebSearchProviderRoutes registers CRUD routes for web search
// provider configurations.
//
// Provider rows hold external service credentials (Bing, Tavily, Google,
// etc.); reads are Viewer+, all mutations / connection tests (which
// probe external systems with stored credentials) and the per-field
// credential subresource are Admin+.
func RegisterWebSearchProviderRoutes(
	r *gin.RouterGroup,
	h *handler.WebSearchProviderHandler,
	credHandler *handler.WebSearchProviderCredentialsHandler,
	g *rbacGuards,
) {
	providers := g.apiKeyGroup(r.Group("/web-search-providers"), apiKeyManageWebSearch(apiKeyFullAccess()))
	{
		// List available provider types (metadata for UI forms) — Viewer+
		providers.GET("/types", g.Viewer(), h.ListProviderTypes)
		// Test with raw credentials (no persistence) — PlatformManaged
		providers.POST("/test", g.PlatformManaged(), h.TestProviderRaw)
		// CRUD
		providers.POST("", g.PlatformManaged(), h.CreateProvider)
		providers.GET("", g.Viewer(), h.ListProviders)
		providers.GET("/:id", g.Viewer(), h.GetProvider)
		providers.PUT("/:id", g.PlatformManaged(), h.UpdateProvider)
		providers.DELETE("/:id", g.PlatformManaged(), h.DeleteProvider)
		// Per-field credential subresource — PlatformManaged
		providers.PUT("/:id/credentials", g.PlatformManaged(), credHandler.Put)
		providers.DELETE("/:id/credentials/:field", g.PlatformManaged(), credHandler.DeleteField)
		// Test existing saved provider — PlatformManaged
		providers.POST("/:id/test", g.PlatformManaged(), h.TestProviderByID)
		// Platform sharing: system-admin only, enforced in the service layer.
		providers.PUT("/:id/sharing", g.PlatformManaged(), h.UpdateProviderSharing)
	}
}

// RegisterVectorStoreRoutes registers CRUD routes for vector store configurations.
//
// Vector stores are tenant-level infrastructure; reads are Viewer+, all
// writes (and connection tests, which probe external systems with stored
// credentials) are Admin+.
func RegisterVectorStoreRoutes(r *gin.RouterGroup, h *handler.VectorStoreHandler, g *rbacGuards) {
	stores := g.apiKeyGroup(r.Group("/vector-stores"), apiKeyManageVectorStores(apiKeyFullAccess()))
	{
		// List available engine types (metadata for UI forms) — Viewer+
		stores.GET("/types", g.Viewer(), h.ListStoreTypes)
		// Test with raw credentials (no persistence) — PlatformManaged
		stores.POST("/test", g.PlatformManaged(), h.TestStoreRaw)
		// CRUD
		stores.POST("", g.PlatformManaged(), h.CreateStore)
		stores.GET("", g.Viewer(), h.ListStores)
		stores.GET("/:id", g.Viewer(), h.GetStore)
		stores.PUT("/:id", g.PlatformManaged(), h.UpdateStore)
		stores.DELETE("/:id", g.PlatformManaged(), h.DeleteStore)
		// Test existing saved or env store — PlatformManaged
		stores.POST("/:id/test", g.PlatformManaged(), h.TestStoreByID)
		stores.PUT("/:id/sharing", g.PlatformManaged(), h.UpdateStoreSharing)
	}
}

// RegisterStorageBackendRoutes manages concrete object/file storage instances.
func RegisterStorageBackendRoutes(r *gin.RouterGroup, h *handler.StorageBackendHandler, g *rbacGuards) {
	backends := g.apiKeyGroup(r.Group("/storage-backends"), apiKeyManageStorageBackends(apiKeyFullAccess()))
	{
		backends.GET("/types", g.Viewer(), h.Types)
		backends.POST("/test", g.PlatformManaged(), h.TestRaw)
		backends.POST("", g.PlatformManaged(), h.Create)
		backends.GET("", g.Viewer(), h.List)
		backends.GET("/:id", g.Viewer(), h.Get)
		backends.PUT("/:id", g.PlatformManaged(), h.Update)
		backends.DELETE("/:id", g.PlatformManaged(), h.Delete)
		backends.POST("/:id/test", g.PlatformManaged(), h.TestByID)
		backends.PUT("/:id/sharing", g.PlatformManaged(), h.UpdateSharing)
		// SetDefault stays Admin+ in both modes: it writes
		// tenants.default_storage_backend_id, i.e. WHICH available backend
		// this workspace uses — a point-of-use selection among platform
		// resources, not a change to the backend itself. Same rationale as
		// picking a platform embedding model for your knowledge base.
		backends.PUT("/:id/default", g.Admin(), h.SetDefault)
	}
}

// RegisterDataSourceRoutes 注册数据源相关的路由
//
// Data sources hold external service credentials (Feishu/Notion/Yuque)
// and trigger sync jobs that mutate KB content tenant-wide. Reads are
// Viewer+; everything else (CRUD, validation, sync control, credential
// subresource) is Admin+.
func RegisterDataSourceRoutes(
	r *gin.RouterGroup,
	handler *handler.DataSourceHandler,
	credHandler *handler.DataSourceCredentialsHandler,
	g *rbacGuards,
) {
	// Data source routes
	ds := g.apiKeyGroup(r.Group("/datasource"), apiKeyManageDataSources(apiKeyFullAccess()))
	{
		// Get available connector types — Viewer+
		ds.GET("/types", g.Viewer(), handler.GetAvailableConnectors)

		// Validate credentials without persistence (for "Test Connection" button) — Admin+
		ds.POST("/validate-credentials", g.Admin(), handler.ValidateCredentials)

		// CRUD operations
		ds.POST("", g.Admin(), handler.CreateDataSource)
		ds.GET("", g.Viewer(), handler.ListDataSources)
		ds.GET("/:id", g.Viewer(), handler.GetDataSource)
		ds.PUT("/:id", g.Admin(), handler.UpdateDataSource)
		ds.DELETE("/:id", g.Admin(), handler.DeleteDataSource)

		// Credential subresource. Single logical field "credentials" because
		// connector credentials are a per-connector atomic map (see
		// internal/handler/datasource_credentials.go). — Admin+
		ds.PUT("/:id/credentials", g.Admin(), credHandler.Put)
		ds.DELETE("/:id/credentials/:field", g.Admin(), credHandler.DeleteField)

		// Connection and resource management — Admin+
		ds.POST("/:id/validate", g.Admin(), handler.ValidateConnection)
		ds.GET("/:id/resources", g.Admin(), handler.ListAvailableResources)
		ds.POST("/:id/resource-ancestors", g.Admin(), handler.ResolveResourceAncestors)

		// Sync management — Admin+
		ds.POST("/:id/sync", g.Admin(), handler.ManualSync)
		ds.POST("/:id/pause", g.Admin(), handler.PauseDataSource)
		ds.POST("/:id/resume", g.Admin(), handler.ResumeDataSource)

		// Sync logs — Viewer+ (read-only audit trail)
		ds.GET("/:id/logs", g.Viewer(), handler.GetSyncLogs)
		ds.GET("/logs/:log_id", g.Viewer(), handler.GetSyncLog)
	}
}
