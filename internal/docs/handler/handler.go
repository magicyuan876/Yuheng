package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/service"
)

// Deps are the collaborators every docs handler may need.
type Deps struct {
	Config   *config.DocsConfig
	Repos    *repository.Repositories
	Resolver *acl.Resolver
	Guard    *acl.Guard
	Bus      events.Bus
	Audit    *audit.Recorder
	// Idempotency backs the Idempotency-Key middleware; nil disables it.
	Idempotency IdempotencyStore
	// Services carries the business layer; nil leaves feature routes at 501.
	Services *service.Services
}

// Handler groups the module's gin handlers. Feature handlers are added by
// their work packages; until then the corresponding routes answer 501.
type Handler struct {
	deps   Deps
	Events *EventStream
	Spaces *SpaceHandler
	Groups *GroupHandler
	Pages  *PageHandler
}

// New builds the handler set.
func New(deps Deps) *Handler {
	h := &Handler{
		deps: deps, Events: NewEventStream(deps.Bus, 0),
		Spaces: &SpaceHandler{}, Groups: &GroupHandler{}, Pages: &PageHandler{},
	}
	if deps.Services != nil {
		h.Spaces.svc = deps.Services.Spaces
		h.Groups.svc = deps.Services.Groups
		h.Pages.svc = deps.Services.Pages
	}
	return h
}

// Idempotency returns the module's Idempotency-Key middleware (a no-op when
// no store is configured).
func (h *Handler) Idempotency() gin.HandlerFunc {
	if h == nil || h.deps.Idempotency == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return Idempotency(h.deps.Idempotency, 24*time.Hour)
}

// Guard exposes the ACL guard for route registration.
func (h *Handler) Guard() *acl.Guard {
	if h == nil {
		return nil
	}
	return h.deps.Guard
}
