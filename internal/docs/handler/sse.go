package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// EventStream pushes docs events to a browser over Server-Sent Events.
//
// One connection per tab; the client narrows the feed with ?space=<id> and/or
// ?page=<id>, so a large tenant does not flood every tab. That narrowing is a
// convenience. What a subscriber may see is decided per event by an
// acl.Audience, against their current permissions. The stream is a refresh
// hint: clients re-fetch what changed, they never apply payloads as truth.
type EventStream struct {
	bus       events.Bus
	res       *acl.Resolver
	heartbeat time.Duration
	// buffer is how many events a slow client may fall behind before we
	// drop events (the client will re-sync on its next refresh anyway).
	buffer int
}

// NewEventStream builds the handler. heartbeat <= 0 uses 25s. Without a bus
// or a resolver the stream answers 503: it never serves unfiltered events.
func NewEventStream(bus events.Bus, res *acl.Resolver, heartbeat time.Duration) *EventStream {
	if heartbeat <= 0 {
		heartbeat = 25 * time.Second
	}
	return &EventStream{bus: bus, res: res, heartbeat: heartbeat, buffer: 64}
}

// Handle godoc
// @Summary      订阅文档事件流（SSE）
// @Description  Server-Sent Events：页面内容/元数据变更、评论、通知、权限变更等
// @Description  每个事件按调用者当前的权限过滤：页面事件要能读该页面，空间事件要能读该空间，通知只推给接收人；
// @Description  连接期间失去权限的内容，只会再收到让它消失的那一个事件；调用者被移出工作区时连接结束
// @Description  可用 space / page 参数只订阅一个空间或页面；连接期间每 25 秒发一次心跳注释行
// @Tags         在线文档
// @Produce      text/event-stream
// @Param        space  query  string  false  "只订阅该空间的事件"
// @Param        page   query  string  false  "只订阅该页面的事件"
// @Success      200  {string}  string  "event stream"
// @Security     Bearer
// @Router       /docs/events [get]
//
// The guard's RequireMember must run first so an identity is present.
func (s *EventStream) Handle(c *gin.Context) {
	id, ok := acl.IdentityFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "identity required"})
		return
	}
	if s.bus == nil || s.res == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "event stream unavailable"})
		return
	}
	ctx := c.Request.Context()
	audience, err := s.res.Audience(ctx, id)
	if err != nil {
		logger.Errorf(ctx, "[docs.events] audience for user=%s: %v", id.UserID, err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "permission check unavailable"})
		return
	}
	spaceFilter := c.Query("space")
	pageFilter := c.Query("page")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	ch := make(chan events.Event, s.buffer)
	unsubscribe := s.bus.Subscribe(id.TenantID, func(e events.Event) {
		if spaceFilter != "" && e.SpaceID != "" && e.SpaceID != spaceFilter {
			return
		}
		if pageFilter != "" && e.PageID != "" && e.PageID != pageFilter {
			return
		}
		select {
		case ch <- e:
		default: // slow client: drop, it will resync
		}
	})
	defer unsubscribe()

	w := c.Writer
	// Tell the client we are live before the first event or heartbeat.
	fmt.Fprintf(w, "event: ready\ndata: {\"tenant_id\":%d}\n\n", id.TenantID)
	w.Flush()

	ticker := time.NewTicker(s.heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			w.Flush()
		case e := <-ch:
			// Authorised here, on the stream's own goroutine: the bus calls
			// the subscription synchronously on the publisher's, which must
			// not wait on permission lookups.
			admitted, err := audience.Admits(ctx, e)
			if errors.Is(err, acl.ErrNoLongerMember) {
				return
			}
			if err != nil {
				logger.Warnf(ctx, "[docs.events] dropping %s for user=%s: %v", e.Type, id.UserID, err)
				continue
			}
			if !admitted {
				continue
			}
			payload, err := json.Marshal(e)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, payload); err != nil {
				return
			}
			w.Flush()
		}
	}
}
