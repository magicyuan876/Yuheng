package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/events"
)

// EventStream pushes docs events to a browser over Server-Sent Events.
//
// One connection per tab; the client narrows the feed with ?space=<id> and/or
// ?page=<id>. Events for other spaces are dropped server-side so a large
// tenant does not flood every tab. The stream is a refresh hint: clients
// re-fetch what changed, they never apply payloads as truth.
type EventStream struct {
	bus       events.Bus
	heartbeat time.Duration
	// buffer is how many events a slow client may fall behind before we
	// drop events (the client will re-sync on its next refresh anyway).
	buffer int
}

// NewEventStream builds the handler. heartbeat <= 0 uses 25s.
func NewEventStream(bus events.Bus, heartbeat time.Duration) *EventStream {
	if heartbeat <= 0 {
		heartbeat = 25 * time.Second
	}
	return &EventStream{bus: bus, heartbeat: heartbeat, buffer: 64}
}

// Handle godoc
// @Summary      订阅文档事件流（SSE）
// @Description  Server-Sent Events：页面内容/元数据变更、评论、通知、权限变更等
// @Description  只推送调用者当前可见的空间；权限变更后不再可见的页面事件会被过滤
// @Description  可用 space 参数只订阅一个空间；连接期间每 25 秒发一次心跳注释行
// @Tags         在线文档
// @Produce      text/event-stream
// @Param        space  query  string  false  "只订阅该空间的事件"
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
	if s.bus == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "event stream unavailable"})
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
	ctx := c.Request.Context()
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
