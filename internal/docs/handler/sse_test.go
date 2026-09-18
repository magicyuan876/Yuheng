package handler

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/stretchr/testify/require"
)

func TestEventStreamDeliversFilteredEvents(t *testing.T) {
	bus := events.NewMemoryBus()
	stream := NewEventStream(bus, 50*time.Millisecond)

	r := gin.New()
	r.GET("/events", func(c *gin.Context) {
		// Stand in for the guard: an identity for tenant 1.
		c.Set(acl.IdentityContextKey, &acl.Identity{TenantID: 1, UserID: "alice", Member: true})
		stream.Handle(c)
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events?space=s1", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	reader := bufio.NewReader(resp.Body)
	readEvent := func() (string, string) {
		var name, data string
		for {
			line, err := reader.ReadString('\n')
			require.NoError(t, err)
			line = strings.TrimRight(line, "\n")
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				data = strings.TrimPrefix(line, "data: ")
			case line == "" && name != "":
				return name, data
			}
		}
	}

	name, data := readEvent()
	require.Equal(t, "ready", name)
	require.Contains(t, data, `"tenant_id":1`)

	// Wait for the subscription to exist, then publish three events: one for
	// another tenant, one for another space, one that should arrive.
	require.Eventually(t, func() bool { return bus.SubscriberCount() == 1 }, time.Second, 5*time.Millisecond)
	require.NoError(t, bus.Publish(ctx, events.New(events.PageCreated, 2).WithSpace("s1").WithPage("other-tenant")))
	require.NoError(t, bus.Publish(ctx, events.New(events.PageCreated, 1).WithSpace("s2").WithPage("other-space")))
	require.NoError(t, bus.Publish(ctx, events.New(events.PageMeta, 1).WithSpace("s1").WithPage("p1").With("title", "T")))

	name, data = readEvent()
	require.Equal(t, string(events.PageMeta), name)
	require.Contains(t, data, `"page_id":"p1"`)
	require.Contains(t, data, `"title":"T"`)

	// Heartbeats are comments and are skipped by the reader loop; ending the
	// request tears the subscription down.
	cancel()
	require.Eventually(t, func() bool { return bus.SubscriberCount() == 0 }, 2*time.Second, 10*time.Millisecond)
}

func TestEventStreamRequiresIdentity(t *testing.T) {
	stream := NewEventStream(events.NewMemoryBus(), time.Second)
	r := gin.New()
	r.GET("/events", stream.Handle)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/events", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
