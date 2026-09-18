package repository

import (
	"testing"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/stretchr/testify/require"
)

func TestSpaceCRUDAndTenantIsolation(t *testing.T) {
	repos := New(openTestDB(t))
	s := &model.Space{TenantID: 1, Slug: "eng", Name: "Engineering", CreatorID: ptr("u1")}
	require.NoError(t, repos.Spaces.Create(ctx(), s))
	require.NotEmpty(t, s.ID)
	require.Equal(t, model.VisibilityPrivate, s.Visibility)
	require.Equal(t, model.RoleNone, s.DefaultRole)

	got, err := repos.Spaces.Get(ctx(), 1, s.ID)
	require.NoError(t, err)
	require.Equal(t, "Engineering", got.Name)
	require.JSONEq(t, "{}", string(got.Settings))

	_, err = repos.Spaces.Get(ctx(), 2, s.ID)
	require.ErrorIs(t, err, ErrNotFound, "another tenant must not see the space")

	bySlug, err := repos.Spaces.GetBySlug(ctx(), 1, "eng")
	require.NoError(t, err)
	require.Equal(t, s.ID, bySlug.ID)

	// Same slug in another tenant is fine; in the same tenant it is not.
	require.NoError(t, repos.Spaces.Create(ctx(), &model.Space{TenantID: 2, Slug: "eng", Name: "Other"}))
	err = repos.Spaces.Create(ctx(), &model.Space{TenantID: 1, Slug: "eng", Name: "Dup"})
	require.ErrorIs(t, err, ErrDuplicate)

	require.NoError(t, repos.Spaces.Update(ctx(), 1, s.ID, map[string]any{
		"name": "Platform", "visibility": model.VisibilityOpen, "default_role": model.RoleReader,
		"settings": model.JSON(`{"theme":"dark"}`),
	}))
	got, err = repos.Spaces.Get(ctx(), 1, s.ID)
	require.NoError(t, err)
	require.Equal(t, "Platform", got.Name)
	require.Equal(t, model.VisibilityOpen, got.Visibility)
	require.JSONEq(t, `{"theme":"dark"}`, string(got.Settings))

	require.Error(t, repos.Spaces.Update(ctx(), 1, s.ID, map[string]any{"tenant_id": 9}),
		"tenant_id is not an updatable column")

	// Soft delete frees the slug for reuse and hides the space.
	require.NoError(t, repos.Spaces.SoftDelete(ctx(), 1, s.ID))
	_, err = repos.Spaces.Get(ctx(), 1, s.ID)
	require.ErrorIs(t, err, ErrNotFound)
	list, err := repos.Spaces.List(ctx(), 1)
	require.NoError(t, err)
	require.Empty(t, list)
	require.NoError(t, repos.Spaces.Create(ctx(), &model.Space{TenantID: 1, Slug: "eng", Name: "Reborn"}))

	// Restoring the old one now collides on the slug.
	require.ErrorIs(t, repos.Spaces.Restore(ctx(), 1, s.ID), ErrDuplicate)
}

func TestSpaceMembersUpsertAndRoleQueries(t *testing.T) {
	repos := New(openTestDB(t))
	a := &model.Space{TenantID: 1, Slug: "a", Name: "A"}
	b := &model.Space{TenantID: 1, Slug: "b", Name: "B"}
	require.NoError(t, repos.Spaces.Create(ctx(), a))
	require.NoError(t, repos.Spaces.Create(ctx(), b))

	require.NoError(t, repos.Members.Upsert(ctx(), &model.SpaceMember{
		SpaceID: a.ID, TenantID: 1, PrincipalType: model.PrincipalUser, PrincipalID: "u1", Role: model.RoleReader,
	}))
	// Upsert of the same principal changes the role instead of failing.
	require.NoError(t, repos.Members.Upsert(ctx(), &model.SpaceMember{
		SpaceID: a.ID, TenantID: 1, PrincipalType: model.PrincipalUser, PrincipalID: "u1", Role: model.RoleWriter,
	}))
	require.NoError(t, repos.Members.Upsert(ctx(), &model.SpaceMember{
		SpaceID: a.ID, TenantID: 1, PrincipalType: model.PrincipalGroup, PrincipalID: "g1", Role: model.RoleAdmin,
	}))
	require.NoError(t, repos.Members.Upsert(ctx(), &model.SpaceMember{
		SpaceID: b.ID, TenantID: 1, PrincipalType: model.PrincipalGroup, PrincipalID: "g1", Role: model.RoleReader,
	}))
	require.Error(t, repos.Members.Upsert(ctx(), &model.SpaceMember{
		SpaceID: b.ID, TenantID: 1, PrincipalType: model.PrincipalUser, PrincipalID: "u9", Role: model.RoleNone,
	}), "none is not grantable")

	members, err := repos.Members.ListBySpace(ctx(), 1, a.ID)
	require.NoError(t, err)
	require.Len(t, members, 2)

	roles, err := repos.Members.RolesFor(ctx(), 1, a.ID, []model.Principal{
		model.UserPrincipal("u1"), model.GroupPrincipal("g1"),
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []model.SpaceRole{model.RoleWriter, model.RoleAdmin}, roles)

	bySpace, err := repos.Members.SpaceRolesFor(ctx(), 1, []model.Principal{
		model.UserPrincipal("u1"), model.GroupPrincipal("g1"),
	})
	require.NoError(t, err)
	require.Equal(t, map[string]model.SpaceRole{a.ID: model.RoleAdmin, b.ID: model.RoleReader}, bySpace)

	// Tenant pinning: the same principal IDs in tenant 2 see nothing.
	none, err := repos.Members.SpaceRolesFor(ctx(), 2, []model.Principal{model.UserPrincipal("u1")})
	require.NoError(t, err)
	require.Empty(t, none)

	require.NoError(t, repos.Members.Remove(ctx(), 1, a.ID, model.UserPrincipal("u1")))
	require.ErrorIs(t, repos.Members.Remove(ctx(), 1, a.ID, model.UserPrincipal("u1")), ErrNotFound)
	require.NoError(t, repos.Members.RemoveAllForPrincipal(ctx(), 1, model.GroupPrincipal("g1")))
	bySpace, err = repos.Members.SpaceRolesFor(ctx(), 1, []model.Principal{model.GroupPrincipal("g1")})
	require.NoError(t, err)
	require.Empty(t, bySpace)
}

func TestRoleOrdering(t *testing.T) {
	require.Equal(t, model.RoleAdmin, model.MaxRole(model.RoleReader, model.RoleAdmin))
	require.Equal(t, model.RoleReader, model.MinRole(model.RoleWriter, model.RoleReader))
	require.Equal(t, model.RoleNone, model.MinRole(model.RoleWriter, model.RoleNone))
	require.True(t, model.RoleWriter.AtLeast(model.RoleReader))
	require.False(t, model.RoleReader.AtLeast(model.RoleWriter))
	require.Equal(t, 0, model.SpaceRole("bogus").Level())
}
