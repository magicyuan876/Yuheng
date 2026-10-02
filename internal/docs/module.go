// Package docs assembles the online documents module: repositories, the
// permission resolver, the event bus, audit recorder and HTTP handlers. It
// is the only thing the DI container and the router need to know about.
package docs

import (
	"context"
	"time"

	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/collab"
	"github.com/magicyuan876/yuheng/internal/docs/embed"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/handler"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/service"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/redis/go-redis/v9"
	"go.uber.org/dig"
	"gorm.io/gorm"
)

// Params are resolved by the DI container. Redis and the audit service are
// optional so a deployment without them still boots.
type Params struct {
	dig.In

	Config        *config.Config
	DB            *gorm.DB
	Redis         *redis.Client `optional:"true"`
	TenantMembers interfaces.TenantMemberRepository
	Users         interfaces.UserRepository
	// Groups is the workspace group service. The module grants space and
	// page access to groups, so it enrols with the service as a dependent:
	// a deleted group takes its grants with it, and any change drops the
	// cached permission decisions. Required, not optional -- a build where
	// groups could be deleted behind this module's back would leave grants
	// to nothing.
	Groups interfaces.TenantGroupService
	// UserService validates the tokens the collaboration service relays.
	UserService interfaces.UserService     `optional:"true"`
	Audit       interfaces.AuditLogService `optional:"true"`
	// Knowledge bases and storage backends are optional so the module still
	// boots in a build that lacks either; binding a space to them is then
	// rejected with a clear message.
	KnowledgeBases  interfaces.KnowledgeBaseRepository `optional:"true"`
	StorageBackends interfaces.StorageBackendService   `optional:"true"`
	// Files and Tenants are what attachments need: where to put and find the
	// bytes, and whose storage quota to charge. Without them the module still
	// boots and uploading is refused with a clear message.
	Files   interfaces.FileStore        `optional:"true"`
	Tenants interfaces.TenantRepository `optional:"true"`
	// Favourites is Yuheng's own starred-resources service, reused for pages
	// and spaces rather than reimplemented. Optional for the same reason as
	// the rest: without it, starring is simply unavailable.
	Favourites interfaces.UserResourceFavoriteService `optional:"true"`
	// KnowledgeBaseService and ModelService serve the drafting feature: the
	// model that writes a draft is the one the knowledge base already names
	// for summaries, so there is no new setting.
	KnowledgeBaseService interfaces.KnowledgeBaseService `optional:"true"`
	ModelService         interfaces.ModelService         `optional:"true"`
	// KnowledgeService mirrors pages into a space's bound knowledge base.
	// Optional: without it a space can still name a knowledge base and
	// nothing is sent to it, which is exactly the state this module was in
	// before T5.2.
	KnowledgeService interfaces.KnowledgeService `optional:"true"`
	// Findings shows a page's knowledge-health findings to its readers.
	// Optional like the rest: without it every page reports none.
	Findings interfaces.KnowledgeFindingService `optional:"true"`
	// Stewardship copies a page's maintainer onto its mirror entry.
	// Optional: without it mirror entries carry no maintainer, and their
	// problems go to the fallbacks.
	Stewardship interfaces.KnowledgeStewardshipService `optional:"true"`
	// Retirers is where the module says how a superseded page mirror leaves
	// the knowledge base. Optional: without it superseding a page is
	// refused from the knowledge base's side, and still works from the page.
	Retirers interfaces.KnowledgeRetirers `optional:"true"`
}

// Module is the assembled docs feature.
type Module struct {
	// Enabled is false when YUHENG_DOCS_ENABLED is not set; the router then
	// registers nothing and the rest of the fields are nil.
	Enabled bool
	Config  *config.DocsConfig

	Repos    *repository.Repositories
	Resolver *acl.Resolver
	Guard    *acl.Guard
	Bus      events.Bus
	Audit    *audit.Recorder
	Services *service.Services
	Handler  *handler.Handler
	// Collab talks to the collaboration service; nil in exclusive-edit mode.
	Collab *collab.Client
	// Cleaner runs the maintenance sweeps on a timer; nil when the module is
	// off. Started by the module and stopped by Close.
	Cleaner *Cleaner
	// Indexer mirrors edited pages into their space's knowledge base.
	Indexer *Indexer
	// Degraded names the optional dependencies the container did not provide.
	// Every one of them makes a feature quietly unavailable (mirroring into the
	// knowledge base, attachments, drafting, ...) instead of failing start-up,
	// so a broken binding is invisible unless somebody asks. The start-up
	// wiring test asserts the list is empty in the shipped configuration.
	Degraded []string
}

// NewModule wires the module from container-provided dependencies.
func NewModule(p Params) *Module {
	cfg := p.Config.Docs
	if !cfg.IsEnabled() {
		return &Module{Enabled: false, Config: cfg}
	}
	repos := repository.New(p.DB)

	var cache acl.Cache
	var bus events.Bus
	var idem handler.IdempotencyStore
	if p.Redis != nil {
		cache = acl.NewRedisCache(p.Redis, "")
		rb := events.NewRedisBus(p.Redis, "")
		rb.Start(context.Background())
		bus = rb
		idem = handler.NewRedisIdempotencyStore(p.Redis)
	} else {
		cache = acl.NewMemoryCache(0)
		bus = events.NewMemoryBus()
		idem = handler.NewMemoryIdempotencyStore()
	}
	ttl := time.Duration(cfg.ACLCacheTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	resolver := acl.NewResolver(repos, acl.NewTenantMemberRoleSource(p.TenantMembers),
		acl.WithCache(cache), acl.WithTTL(ttl))
	guard := acl.NewGuard(resolver)
	rec := audit.NewRecorder(p.Audit)

	// Any event that can change a permission decision drops the tenant's
	// cached decisions on every instance (Redis cache) or this one (memory).
	bus.Subscribe(0, func(e events.Event) {
		switch e.Type {
		case events.SpaceMembersChange, events.PageAccess, events.PageMoved, events.PageDeleted,
			events.PageRestored, events.SpaceUpdated, events.SpaceDeleted, events.GroupChanged, events.PageMeta:
			resolver.Invalidate(context.Background(), e.TenantID)
		}
	})
	// Workspace groups are managed outside this module; the bridge is how
	// their changes reach the grants and the cache above.
	p.Groups.RegisterDependent(newGroupBridge(repos, resolver, bus))

	collabClient := collab.NewClient(cfg.CollabInternalURL(), cfg.CollabSharedSecret, 15*time.Second)
	deps := service.Deps{
		Repos: repos, Resolver: resolver, Bus: bus, Audit: rec, Users: p.Users, Members: p.TenantMembers,
		CollabURL: cfg.CollabURL, MaxYDocBytes: cfg.MaxYDocBytes,
		MaxAttachmentBytes:     cfg.MaxAttachmentBytes,
		Embeds:                 embed.NewRegistry(cfg.EmbedProviders, cfg.EmbedExtraHosts),
		DrawioURL:              cfg.DrawioURL,
		PublicSharing:          cfg.PublicSharing,
		DefaultSpaceQuotaBytes: cfg.DefaultSpaceQuotaBytes,
	}
	if p.Files != nil {
		deps.Storage = p.Files
	}
	if p.Tenants != nil {
		deps.Tenants = p.Tenants
	}
	if p.Favourites != nil {
		deps.Favourites = p.Favourites
	}
	if p.KnowledgeService != nil {
		deps.Knowledge = NewKnowledgeBridge(p.KnowledgeService, p.Tenants, p.Stewardship)
	}
	deps.Drafter = NewDraftBridge(p.KnowledgeService, p.KnowledgeBaseService, p.ModelService)
	if p.UserService != nil {
		deps.Tokens = p.UserService
	}
	if p.Findings != nil {
		deps.Findings = p.Findings
	}
	if collabClient != nil {
		deps.Collab = collabClient
	}
	if p.KnowledgeBases != nil {
		deps.KnowledgeBases = p.KnowledgeBases
	}
	if p.StorageBackends != nil {
		deps.StorageBackends = p.StorageBackends
	}
	degraded := p.degraded()
	if len(degraded) > 0 {
		logger.Warnf(context.Background(), "[docs] running without optional dependencies: %v", degraded)
	}
	services := service.New(deps)
	if p.Retirers != nil {
		p.Retirers.Register(types.KnowledgeOriginDocs, pageRetirer{pages: services.Pages})
	}
	h := handler.New(handler.Deps{
		Config: cfg, Repos: repos, Resolver: resolver, Guard: guard, Bus: bus, Audit: rec, Idempotency: idem,
		Services: services,
	})
	mode := "collaboration service at " + cfg.CollabURL
	if !cfg.CollabEnabled() {
		mode = "exclusive-edit mode (no collaboration service)"
	}
	logger.Infof(context.Background(), "[docs] module enabled, %s, redis=%v", mode, p.Redis != nil)

	indexer := NewIndexer(services.Pages, bus,
		time.Duration(cfg.IndexDebounceSeconds)*time.Second)
	indexer.Start(context.Background())

	cleaner := NewCleaner(services.Pages, services.Files,
		time.Duration(cfg.TrashRetentionDays)*24*time.Hour,
		time.Duration(cfg.CleanupIntervalMinutes)*time.Minute)
	cleaner.Start(context.Background())

	return &Module{
		Enabled: true, Config: cfg, Repos: repos, Resolver: resolver, Guard: guard, Bus: bus, Audit: rec,
		Services: services, Handler: h, Collab: collabClient, Cleaner: cleaner,
		Indexer: indexer, Degraded: degraded,
	}
}

// degraded lists the optional parameters that arrived nil. Redis is left out
// on purpose: running without it is a supported single-process mode, not a
// defect.
func (p Params) degraded() []string {
	var missing []string
	note := func(name string, absent bool) {
		if absent {
			missing = append(missing, name)
		}
	}
	note("UserService", p.UserService == nil)
	note("Audit", p.Audit == nil)
	note("KnowledgeBases", p.KnowledgeBases == nil)
	note("StorageBackends", p.StorageBackends == nil)
	note("Files", p.Files == nil)
	note("Tenants", p.Tenants == nil)
	note("Favourites", p.Favourites == nil)
	note("KnowledgeBaseService", p.KnowledgeBaseService == nil)
	note("ModelService", p.ModelService == nil)
	note("KnowledgeService", p.KnowledgeService == nil)
	note("Findings", p.Findings == nil)
	note("Stewardship", p.Stewardship == nil)
	note("Retirers", p.Retirers == nil)
	return missing
}

// Close releases background resources: the maintenance loop and the Redis
// subscriber.
//
// The cleaner is stopped first and waited for, so a sweep in flight finishes
// its current deletion rather than being cut off between releasing an object
// and removing the row that points at it.
func (m *Module) Close() error {
	if m == nil {
		return nil
	}
	m.Indexer.Stop()
	m.Cleaner.Stop()
	if m.Bus == nil {
		return nil
	}
	return m.Bus.Close()
}
