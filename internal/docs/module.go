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
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/handler"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/redis/go-redis/v9"
	"go.uber.org/dig"
	"gorm.io/gorm"
)

// Params are resolved by the DI container. Redis and the audit service are
// optional so the Lite edition (no Redis, possibly no audit) still boots.
type Params struct {
	dig.In

	Config        *config.Config
	DB            *gorm.DB
	Redis         *redis.Client `optional:"true"`
	TenantMembers interfaces.TenantMemberRepository
	Audit         interfaces.AuditLogService `optional:"true"`
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
	Handler  *handler.Handler
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

	h := handler.New(handler.Deps{
		Config: cfg, Repos: repos, Resolver: resolver, Guard: guard, Bus: bus, Audit: rec, Idempotency: idem,
	})
	mode := "collaboration service at " + cfg.CollabURL
	if !cfg.CollabEnabled() {
		mode = "exclusive-edit mode (no collaboration service)"
	}
	logger.Infof(context.Background(), "[docs] module enabled, %s, redis=%v", mode, p.Redis != nil)
	return &Module{
		Enabled: true, Config: cfg, Repos: repos, Resolver: resolver, Guard: guard, Bus: bus, Audit: rec, Handler: h,
	}
}

// Close releases background resources (the Redis subscriber loop).
func (m *Module) Close() error {
	if m == nil || m.Bus == nil {
		return nil
	}
	return m.Bus.Close()
}
