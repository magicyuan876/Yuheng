package events

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/redis/go-redis/v9"
)

// ---- in-process ------------------------------------------------------------

type subscription struct {
	id       uint64
	tenantID uint64
	h        Handler
}

// MemoryBus delivers events to subscribers in the same process.
type MemoryBus struct {
	mu   sync.RWMutex
	subs map[uint64]*subscription
	seq  atomic.Uint64
}

// NewMemoryBus creates an empty bus.
func NewMemoryBus() *MemoryBus { return &MemoryBus{subs: map[uint64]*subscription{}} }

// Publish implements Bus.
func (b *MemoryBus) Publish(_ context.Context, e Event) error {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	b.dispatch(e)
	return nil
}

func (b *MemoryBus) dispatch(e Event) {
	b.mu.RLock()
	targets := make([]Handler, 0, len(b.subs))
	for _, s := range b.subs {
		if s.tenantID == 0 || s.tenantID == e.TenantID {
			targets = append(targets, s.h)
		}
	}
	b.mu.RUnlock()
	for _, h := range targets {
		safeCall(h, e)
	}
}

func safeCall(h Handler, e Event) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf(context.Background(), "[docs.events] subscriber panicked on %s: %v", e.Type, r)
		}
	}()
	h(e)
}

// Subscribe implements Bus.
func (b *MemoryBus) Subscribe(tenantID uint64, h Handler) func() {
	id := b.seq.Add(1)
	b.mu.Lock()
	b.subs[id] = &subscription{id: id, tenantID: tenantID, h: h}
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		delete(b.subs, id)
		b.mu.Unlock()
	}
}

// Close implements Bus.
func (b *MemoryBus) Close() error {
	b.mu.Lock()
	b.subs = map[uint64]*subscription{}
	b.mu.Unlock()
	return nil
}

// SubscriberCount is for tests and metrics.
func (b *MemoryBus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

// ---- Redis -----------------------------------------------------------------

// DefaultChannel is the pub/sub channel RedisBus uses.
const DefaultChannel = "docs.events"

// RedisBus delivers locally like MemoryBus and mirrors every event to a Redis
// channel so other instances deliver it to their own subscribers. Events
// carry the publishing instance id; the subscriber loop drops its own.
type RedisBus struct {
	local    *MemoryBus
	rdb      redis.UniversalClient
	channel  string
	instance string
	cancel   context.CancelFunc
	done     chan struct{}
	started  sync.Once
	closed   sync.Once
}

// NewRedisBus creates the bus; call Start to begin receiving from peers.
func NewRedisBus(rdb redis.UniversalClient, channel string) *RedisBus {
	if channel == "" {
		channel = DefaultChannel
	}
	return &RedisBus{local: NewMemoryBus(), rdb: rdb, channel: channel, instance: uuid.NewString()}
}

// Start launches the subscriber loop. It reconnects with exponential
// backoff (capped at 30s) and exits when ctx is done or Close is called.
func (b *RedisBus) Start(ctx context.Context) {
	b.started.Do(func() {
		ctx, b.cancel = context.WithCancel(ctx)
		b.done = make(chan struct{})
		go b.loop(ctx)
	})
}

func (b *RedisBus) loop(ctx context.Context) {
	defer close(b.done)
	const maxBackoff = 30 * time.Second
	backoff := time.Second
	for ctx.Err() == nil {
		sub := b.rdb.Subscribe(ctx, b.channel)
		if _, err := sub.Receive(ctx); err != nil {
			_ = sub.Close()
			if ctx.Err() != nil {
				return
			}
			logger.Warnf(ctx, "[docs.events] subscribe %s: %v (retry in %s)", b.channel, err, backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			if backoff < maxBackoff {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		b.consume(ctx, sub.Channel())
		_ = sub.Close()
	}
}

func (b *RedisBus) consume(ctx context.Context, ch <-chan *redis.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var e Event
			if err := json.Unmarshal([]byte(msg.Payload), &e); err != nil {
				logger.Warnf(ctx, "[docs.events] bad payload on %s: %v", b.channel, err)
				continue
			}
			if e.Origin == b.instance {
				continue // already delivered locally by Publish
			}
			b.local.dispatch(e)
		}
	}
}

// Publish implements Bus: local delivery first (so the publisher's own SSE
// clients never wait on Redis), then the mirror to peers.
func (b *RedisBus) Publish(ctx context.Context, e Event) error {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	e.Origin = b.instance
	b.local.dispatch(e)
	payload, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return b.rdb.Publish(ctx, b.channel, payload).Err()
}

// Subscribe implements Bus.
func (b *RedisBus) Subscribe(tenantID uint64, h Handler) func() {
	return b.local.Subscribe(tenantID, h)
}

// Close implements Bus.
func (b *RedisBus) Close() error {
	b.closed.Do(func() {
		if b.cancel != nil {
			b.cancel()
			<-b.done
		}
		_ = b.local.Close()
	})
	return nil
}
