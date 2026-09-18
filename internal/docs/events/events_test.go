package events

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestMemoryBusRoutesByTenant(t *testing.T) {
	bus := NewMemoryBus()
	var got1, gotAll []Type
	var mu sync.Mutex
	unsub1 := bus.Subscribe(1, func(e Event) { mu.Lock(); got1 = append(got1, e.Type); mu.Unlock() })
	unsubAll := bus.Subscribe(0, func(e Event) { mu.Lock(); gotAll = append(gotAll, e.Type); mu.Unlock() })
	defer unsubAll()

	require.NoError(t, bus.Publish(context.Background(), New(PageCreated, 1).WithPage("p1")))
	require.NoError(t, bus.Publish(context.Background(), New(PageDeleted, 2)))
	mu.Lock()
	require.Equal(t, []Type{PageCreated}, got1)
	require.Equal(t, []Type{PageCreated, PageDeleted}, gotAll)
	mu.Unlock()

	unsub1()
	require.NoError(t, bus.Publish(context.Background(), New(PageMoved, 1)))
	mu.Lock()
	require.Equal(t, []Type{PageCreated}, got1, "unsubscribed handler receives nothing")
	mu.Unlock()
	require.Equal(t, 1, bus.SubscriberCount())
}

func TestMemoryBusSurvivesPanickingSubscriber(t *testing.T) {
	bus := NewMemoryBus()
	bus.Subscribe(0, func(Event) { panic("boom") })
	delivered := false
	bus.Subscribe(0, func(Event) { delivered = true })
	require.NoError(t, bus.Publish(context.Background(), New(PageMeta, 1)))
	require.True(t, delivered, "a panicking subscriber must not stop delivery to the others")
}

func TestEventBuilder(t *testing.T) {
	e := New(CommentChanged, 7).WithSpace("s").WithPage("p").WithActor("u").With("k", 1)
	require.Equal(t, CommentChanged, e.Type)
	require.Equal(t, uint64(7), e.TenantID)
	require.Equal(t, "s", e.SpaceID)
	require.Equal(t, "p", e.PageID)
	require.Equal(t, "u", e.ActorID)
	require.Equal(t, 1, e.Payload["k"])
	require.False(t, e.At.IsZero())
}

func TestRedisBusFansOutAcrossInstances(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := NewRedisBus(rdb, "test.docs.events")
	b := NewRedisBus(rdb, "test.docs.events")
	a.Start(ctx)
	b.Start(ctx)
	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })

	gotA := make(chan Event, 4)
	gotB := make(chan Event, 4)
	a.Subscribe(1, func(e Event) { gotA <- e })
	b.Subscribe(1, func(e Event) { gotB <- e })

	// Give both subscriber loops a moment to attach.
	require.Eventually(t, func() bool {
		n, err := rdb.PubSubNumSub(ctx, "test.docs.events").Result()
		return err == nil && n["test.docs.events"] == 2
	}, 5*time.Second, 20*time.Millisecond)

	require.NoError(t, a.Publish(ctx, New(PageContent, 1).WithPage("p1")))

	select {
	case e := <-gotA:
		require.Equal(t, "p1", e.PageID)
	case <-time.After(time.Second):
		t.Fatal("publisher's local subscriber did not receive the event")
	}
	select {
	case e := <-gotB:
		require.Equal(t, "p1", e.PageID)
		require.Equal(t, PageContent, e.Type)
	case <-time.After(3 * time.Second):
		t.Fatal("peer instance did not receive the event via Redis")
	}
	// Exactly once on the publisher: its own mirrored message is skipped.
	select {
	case e := <-gotA:
		t.Fatalf("publisher received its own event twice: %+v", e)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestRedisBusCloseIsIdempotent(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	bus := NewRedisBus(rdb, "")
	bus.Start(context.Background())
	require.NoError(t, bus.Close())
	require.NoError(t, bus.Close())
}
