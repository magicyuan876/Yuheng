package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/stretchr/testify/require"
)

// leaseFor builds the row a caller would ask for.
func leaseFor(pageID, user, session string, expires time.Time) model.EditLease {
	return model.EditLease{TenantID: 1, PageID: pageID, UserID: user, SessionID: session, ExpiresAt: expires}
}

func TestLeaseAcquireTakesAnUnleasedPage(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "Page", nil, "a0")
	at := time.Now().UTC().Truncate(time.Microsecond)

	held, acquired, err := f.repos.Leases.Acquire(ctx(), leaseFor(page.ID, "u1", "s1", at.Add(5*time.Minute)), at)
	require.NoError(t, err)
	require.True(t, acquired)
	require.Equal(t, "u1", held.UserID)

	stored, err := f.repos.Leases.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Equal(t, "s1", stored.SessionID)
}

func TestLeaseAcquireExtendsTheSameSession(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "Page", nil, "a0")
	at := time.Now().UTC().Truncate(time.Microsecond)
	first := at.Add(5 * time.Minute)
	_, acquired, err := f.repos.Leases.Acquire(ctx(), leaseFor(page.ID, "u1", "s1", first), at)
	require.NoError(t, err)
	require.True(t, acquired)

	// One minute later the same tab renews: still well inside its lease.
	renewAt := at.Add(time.Minute)
	extended := renewAt.Add(5 * time.Minute)
	_, acquired, err = f.repos.Leases.Acquire(ctx(), leaseFor(page.ID, "u1", "s1", extended), renewAt)
	require.NoError(t, err)
	require.True(t, acquired, "the holder must be able to renew its own lease")

	stored, err := f.repos.Leases.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.WithinDuration(t, extended, stored.ExpiresAt, time.Second)
}

func TestLeaseAcquireRefusesWhileSomebodyElseHoldsIt(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "Page", nil, "a0")
	at := time.Now().UTC().Truncate(time.Microsecond)
	_, acquired, err := f.repos.Leases.Acquire(ctx(), leaseFor(page.ID, "u1", "s1", at.Add(5*time.Minute)), at)
	require.NoError(t, err)
	require.True(t, acquired)

	holder, acquired, err := f.repos.Leases.Acquire(ctx(),
		leaseFor(page.ID, "u2", "s2", at.Add(10*time.Minute)), at.Add(time.Minute))
	require.NoError(t, err)
	require.False(t, acquired)
	require.Equal(t, "u1", holder.UserID, "the caller must learn who is editing")
	require.Equal(t, "s1", holder.SessionID)
}

func TestLeaseAcquireRefusesASecondTabOfTheSameUser(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "Page", nil, "a0")
	at := time.Now().UTC().Truncate(time.Microsecond)
	_, acquired, err := f.repos.Leases.Acquire(ctx(), leaseFor(page.ID, "u1", "s1", at.Add(5*time.Minute)), at)
	require.NoError(t, err)
	require.True(t, acquired)

	// Two tabs of one person are still two editors: without a merging
	// collaboration service the second one must not write over the first.
	holder, acquired, err := f.repos.Leases.Acquire(ctx(),
		leaseFor(page.ID, "u1", "s2", at.Add(10*time.Minute)), at)
	require.NoError(t, err)
	require.False(t, acquired)
	require.Equal(t, "s1", holder.SessionID)
}

func TestLeaseAcquireTakesOverOnceItExpires(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "Page", nil, "a0")
	at := time.Now().UTC().Truncate(time.Microsecond)
	_, acquired, err := f.repos.Leases.Acquire(ctx(), leaseFor(page.ID, "u1", "s1", at.Add(5*time.Minute)), at)
	require.NoError(t, err)
	require.True(t, acquired)

	// The first editor closed their laptop; six minutes on, the lease is dead.
	later := at.Add(6 * time.Minute)
	held, acquired, err := f.repos.Leases.Acquire(ctx(),
		leaseFor(page.ID, "u2", "s2", later.Add(5*time.Minute)), later)
	require.NoError(t, err)
	require.True(t, acquired, "an expired lease must be takeable")
	require.Equal(t, "u2", held.UserID)

	stored, err := f.repos.Leases.Get(ctx(), 1, page.ID)
	require.NoError(t, err)
	require.Equal(t, "u2", stored.UserID)
	require.Equal(t, "s2", stored.SessionID)
}

func TestLeaseReleaseOnlyWorksForTheHolder(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "Page", nil, "a0")
	at := time.Now().UTC().Truncate(time.Microsecond)
	_, _, err := f.repos.Leases.Acquire(ctx(), leaseFor(page.ID, "u1", "s1", at.Add(5*time.Minute)), at)
	require.NoError(t, err)

	released, err := f.repos.Leases.Release(ctx(), 1, page.ID, "u2", "s2")
	require.NoError(t, err)
	require.False(t, released, "a stranger must not be able to unlock the page")

	released, err = f.repos.Leases.Release(ctx(), 1, page.ID, "u1", "s9")
	require.NoError(t, err)
	require.False(t, released, "a superseded session of the same user must not release it either")

	released, err = f.repos.Leases.Release(ctx(), 1, page.ID, "u1", "s1")
	require.NoError(t, err)
	require.True(t, released)

	_, err = f.repos.Leases.Get(ctx(), 1, page.ID)
	require.True(t, errors.Is(err, ErrNotFound))
}

func TestLeaseIsScopedToItsTenant(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "Page", nil, "a0")
	at := time.Now().UTC().Truncate(time.Microsecond)
	_, _, err := f.repos.Leases.Acquire(ctx(), leaseFor(page.ID, "u1", "s1", at.Add(5*time.Minute)), at)
	require.NoError(t, err)

	_, err = f.repos.Leases.Get(ctx(), 2, page.ID)
	require.True(t, errors.Is(err, ErrNotFound), "another tenant must not see the lease")

	released, err := f.repos.Leases.Release(ctx(), 2, page.ID, "u1", "s1")
	require.NoError(t, err)
	require.False(t, released, "another tenant must not be able to release it")
}

func TestLeaseDisappearsWithItsPage(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "Page", nil, "a0")
	at := time.Now().UTC().Truncate(time.Microsecond)
	_, _, err := f.repos.Leases.Acquire(ctx(), leaseFor(page.ID, "u1", "s1", at.Add(5*time.Minute)), at)
	require.NoError(t, err)

	_, err = f.repos.Pages.SoftDeleteSubtree(ctx(), 1, page.ID, "u1")
	require.NoError(t, err)
	_, err = f.repos.Pages.PurgeOne(ctx(), 1, page.ID)
	require.NoError(t, err)

	_, err = f.repos.Leases.Get(ctx(), 1, page.ID)
	require.True(t, errors.Is(err, ErrNotFound), "the foreign key must cascade the lease away")
}

func TestLeasePurgeExpiredRemovesOnlyDeadRows(t *testing.T) {
	f := newFixture(t)
	live := f.page(t, "Live", nil, "a0")
	dead := f.page(t, "Dead", nil, "a1")
	at := time.Now().UTC().Truncate(time.Microsecond)
	_, _, err := f.repos.Leases.Acquire(ctx(), leaseFor(live.ID, "u1", "s1", at.Add(5*time.Minute)), at)
	require.NoError(t, err)
	_, _, err = f.repos.Leases.Acquire(ctx(), leaseFor(dead.ID, "u2", "s2", at.Add(-time.Minute)), at)
	require.NoError(t, err)

	removed, err := f.repos.Leases.PurgeExpired(ctx(), at)
	require.NoError(t, err)
	require.Equal(t, int64(1), removed)

	_, err = f.repos.Leases.Get(ctx(), 1, live.ID)
	require.NoError(t, err)
	_, err = f.repos.Leases.Get(ctx(), 1, dead.ID)
	require.True(t, errors.Is(err, ErrNotFound))
}
