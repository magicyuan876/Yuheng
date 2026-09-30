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
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/require"
)

// TestEventStreamDeliversFilteredEvents runs the stream end to end over a
// real resolver: alice reads space s1 but holds no grant on its restricted
// page, and nothing of space s2.
func TestEventStreamDeliversFilteredEvents(t *testing.T) {
	bus := events.NewMemoryBus()
	repos := repository.New(pgtest.New(t))
	res := acl.NewResolver(repos, acl.NewTenantMemberRoleSource(tenantTable{
		"1/alice": types.TenantRoleContributor, "1/owner": types.TenantRoleOwner,
	}))
	s1 := &model.Space{
		TenantID: 1, Slug: "s1", Name: "S1", Visibility: model.VisibilityOpen, DefaultRole: model.RoleReader,
	}
	s2 := &model.Space{TenantID: 1, Slug: "s2", Name: "S2", Visibility: model.VisibilityPrivate}
	require.NoError(t, repos.Spaces.Create(context.Background(), s1))
	require.NoError(t, repos.Spaces.Create(context.Background(), s2))
	open := &model.Page{TenantID: 1, SpaceID: s1.ID, Title: "Open", ShortID: "p0000001", Position: "a0"}
	secret := &model.Page{TenantID: 1, SpaceID: s1.ID, Title: "Secret", ShortID: "p0000002", Position: "a1"}
	require.NoError(t, repos.Pages.Create(context.Background(), open))
	require.NoError(t, repos.Pages.Create(context.Background(), secret))
	require.NoError(t, repos.Access.SetRestricted(context.Background(), 1, s1.ID, secret.ID, "owner"))

	stream := NewEventStream(bus, res, 50*time.Millisecond)

	r := gin.New()
	r.GET("/events", func(c *gin.Context) {
		// Stand in for the guard: an identity for tenant 1.
		id, err := res.Identity(c.Request.Context(), 1, "alice")
		require.NoError(t, err)
		c.Set(acl.IdentityContextKey, id)
		stream.Handle(c)
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
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

	// Wait for the subscription to exist, then publish what alice must not
	// see (another tenant, a space she cannot read, a restricted page, someone
	// else's notification) followed by one event she may.
	require.Eventually(t, func() bool { return bus.SubscriberCount() == 1 }, time.Second, 5*time.Millisecond)
	require.NoError(t, bus.Publish(ctx, events.New(events.PageCreated, 2).WithSpace(s1.ID).WithPage(open.ID)))
	require.NoError(t, bus.Publish(ctx, events.New(events.SpaceUpdated, 1).WithSpace(s2.ID)))
	require.NoError(t, bus.Publish(ctx,
		events.New(events.PageMeta, 1).WithSpace(s1.ID).WithPage(secret.ID).With("title", "Secret")))
	require.NoError(t, bus.Publish(ctx,
		events.New(events.NotificationCreated, 1).WithSpace(s1.ID).WithPage(open.ID).With("user_id", "owner")))
	require.NoError(t, bus.Publish(ctx,
		events.New(events.PageMeta, 1).WithSpace(s1.ID).WithPage(open.ID).With("title", "Open")))

	name, data = readEvent()
	require.Equal(t, string(events.PageMeta), name, "the first event through is the only one alice may see")
	require.Contains(t, data, `"page_id":"`+open.ID+`"`)
	require.Contains(t, data, `"title":"Open"`)

	// Heartbeats are comments and are skipped by the reader loop; ending the
	// request tears the subscription down.
	cancel()
	require.Eventually(t, func() bool { return bus.SubscriberCount() == 0 }, 2*time.Second, 10*time.Millisecond)
}

func TestEventStreamRequiresIdentity(t *testing.T) {
	stream := NewEventStream(events.NewMemoryBus(), nil, time.Second)
	r := gin.New()
	r.GET("/events", stream.Handle)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/events", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
