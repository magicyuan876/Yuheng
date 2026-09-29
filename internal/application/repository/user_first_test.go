package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
)

func newFirstUserCandidate(n int) *types.User {
	return &types.User{
		ID:           fmt.Sprintf("user-%d", n),
		Username:     fmt.Sprintf("user%d", n),
		Email:        fmt.Sprintf("user%d@example.com", n),
		PasswordHash: "x",
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func TestCreateFirstUserOnlyWhenEmpty(t *testing.T) {
	ctx := context.Background()
	repo := NewUserRepository(pgtest.New(t))

	has, err := repo.HasAnyUser(ctx)
	if err != nil || has {
		t.Fatalf("empty database: HasAnyUser = %v, %v", has, err)
	}

	first := newFirstUserCandidate(1)
	if err := repo.CreateFirstUser(ctx, first); err != nil {
		t.Fatalf("first user: %v", err)
	}
	stored, err := repo.GetUserByID(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.IsSystemAdmin {
		t.Fatal("the first user must be the system administrator")
	}
	if has, _ := repo.HasAnyUser(ctx); !has {
		t.Fatal("HasAnyUser must be true once a user exists")
	}

	err = repo.CreateFirstUser(ctx, newFirstUserCandidate(2))
	if !errors.Is(err, types.ErrRegistrationClosed) {
		t.Fatalf("second CreateFirstUser = %v, want ErrRegistrationClosed", err)
	}
}

// TestCreateFirstUserRace fires many simultaneous first registrations; the
// advisory lock must let exactly one through, so there is exactly one
// administrator.
func TestCreateFirstUserRace(t *testing.T) {
	ctx := context.Background()
	db := pgtest.New(t)
	repo := NewUserRepository(db)

	const n = 12
	var wg sync.WaitGroup
	results := make([]error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = repo.CreateFirstUser(ctx, newFirstUserCandidate(i))
		}(i)
	}
	close(start)
	wg.Wait()

	winners := 0
	for i, err := range results {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, types.ErrRegistrationClosed):
		default:
			t.Fatalf("attempt %d: unexpected error %v", i, err)
		}
	}
	if winners != 1 {
		t.Fatalf("%d registrations won, want exactly 1", winners)
	}
	var admins, total int64
	db.Model(&types.User{}).Count(&total)
	db.Model(&types.User{}).Where("is_system_admin = ?", true).Count(&admins)
	if total != 1 || admins != 1 {
		t.Fatalf("users=%d admins=%d, want 1 and 1", total, admins)
	}
}
