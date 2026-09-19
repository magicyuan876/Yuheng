package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// hold puts bytes into the fixture's space without going through an upload,
// which needs storage this fixture does not wire.
func (p *pageEnv) hold(t *testing.T, bytes int64) *model.Attachment {
	t.Helper()
	row := &model.Attachment{
		TenantID: 1, SpaceID: p.space.ID, FileName: "big.bin",
		Mime: "application/octet-stream", SizeBytes: bytes,
		FilePath: "docs/1/" + p.space.ID + "/" + randomWord(t),
	}
	require.NoError(t, p.repos.Files.Create(ctx(), row))
	return row
}

func TestAnUnlimitedSpaceReportsNoCeiling(t *testing.T) {
	p := newPageEnv(t)
	p.hold(t, 4096)

	usage, err := p.svc.Spaces.SpaceUsage(ctx(), p.alice, p.space, model.RoleAdmin)
	require.NoError(t, err)
	assert.EqualValues(t, 4096, usage.UsedBytes)
	assert.EqualValues(t, 0, usage.QuotaBytes, "0 is unlimited, as everywhere else")
	assert.EqualValues(t, -1, usage.Remaining())
}

func TestUsageCountsWhatTheSpaceHolds(t *testing.T) {
	p := newPageEnv(t)
	p.hold(t, 1000)
	p.hold(t, 2000)

	usage, err := p.svc.Spaces.SpaceUsage(ctx(), p.alice, p.space, model.RoleAdmin)
	require.NoError(t, err)
	assert.EqualValues(t, 3000, usage.UsedBytes)
}

// A member who cannot see why their upload was refused will try again and
// then ask somebody, which costs more than telling them.
func TestAnyReaderMaySeeTheQuota(t *testing.T) {
	p := newPageEnv(t)
	usage, err := p.svc.Spaces.SpaceUsage(ctx(), p.carol, p.space, model.RoleReader)
	require.NoError(t, err)
	assert.False(t, usage.CanManage, "but she is told she cannot change it")
}

// The whole point of the column: a space administrator cannot raise their
// own ceiling.
func TestOnlyAWorkspaceAdministratorSetsAQuota(t *testing.T) {
	p := newPageEnv(t)

	// alice administers the space but is only a contributor in the tenant.
	_, err := p.svc.Spaces.SetSpaceQuota(ctx(), p.alice, p.space, 1<<30)
	require.Error(t, err)

	owner := p.identity("owner")
	usage, err := p.svc.Spaces.SetSpaceQuota(ctx(), owner, p.space, 1<<20)
	require.NoError(t, err)
	assert.EqualValues(t, 1<<20, usage.QuotaBytes)
	assert.False(t, usage.FromDefault)
}

func TestANegativeQuotaIsRefused(t *testing.T) {
	p := newPageEnv(t)
	owner := p.identity("owner")
	_, err := p.svc.Spaces.SetSpaceQuota(ctx(), owner, p.space, -1)
	require.Error(t, err)
}

// Setting a quota below what the space already holds stops it growing and
// deletes nothing: destroying data is never a side effect of changing a
// number.
func TestAQuotaBelowCurrentUsageDeletesNothing(t *testing.T) {
	p := newPageEnv(t)
	row := p.hold(t, 5000)
	owner := p.identity("owner")

	usage, err := p.svc.Spaces.SetSpaceQuota(ctx(), owner, p.space, 1000)
	require.NoError(t, err)
	assert.EqualValues(t, 5000, usage.UsedBytes)
	assert.EqualValues(t, 1000, usage.QuotaBytes)
	assert.EqualValues(t, 0, usage.Remaining(), "no room, but nothing removed")

	_, err = p.repos.Files.Get(ctx(), 1, row.ID)
	require.NoError(t, err)
}

// A careful deployment can be careful without anybody visiting every space.
func TestTheDeploymentDefaultFillsInForSpacesWithoutAQuota(t *testing.T) {
	p := newPageEnvWith(t, func(d *Deps) { d.DefaultSpaceQuotaBytes = 4096 })

	usage, err := p.svc.Spaces.SpaceUsage(ctx(), p.alice, p.space, model.RoleAdmin)
	require.NoError(t, err)
	assert.EqualValues(t, 4096, usage.QuotaBytes)
	assert.True(t, usage.FromDefault, "and says where the number came from")
}

// A space's own number wins over the deployment default, in both directions.
func TestASpacesOwnQuotaOverridesTheDefault(t *testing.T) {
	p := newPageEnvWith(t, func(d *Deps) { d.DefaultSpaceQuotaBytes = 4096 })
	owner := p.identity("owner")

	raised, err := p.svc.Spaces.SetSpaceQuota(ctx(), owner, p.space, 1<<20)
	require.NoError(t, err)
	assert.EqualValues(t, 1<<20, raised.QuotaBytes)
	assert.False(t, raised.FromDefault)
}

// ---- the check the upload path runs ---------------------------------------------

func TestTheSpaceQuotaCheckLetsAnUploadThroughWhenThereIsRoom(t *testing.T) {
	p := newPageEnvWith(t, func(d *Deps) { d.DefaultSpaceQuotaBytes = 10000 })
	p.hold(t, 1000)

	require.NoError(t, p.svc.Pages.checkSpaceQuota(ctx(), p.space, 500))
}

func TestTheSpaceQuotaCheckRefusesAnUploadThatWouldNotFit(t *testing.T) {
	p := newPageEnvWith(t, func(d *Deps) { d.DefaultSpaceQuotaBytes = 10000 })
	p.hold(t, 9800)

	err := p.svc.Pages.checkSpaceQuota(ctx(), p.space, 500)
	require.Error(t, err)
	assert.Equal(t, 409, httpCode(t, err))
	assert.Contains(t, err.Error(), "quota")
}

// Exactly filling the quota is allowed; one byte past it is not.
func TestTheBoundaryIsInclusive(t *testing.T) {
	p := newPageEnvWith(t, func(d *Deps) { d.DefaultSpaceQuotaBytes = 1000 })
	p.hold(t, 900)

	require.NoError(t, p.svc.Pages.checkSpaceQuota(ctx(), p.space, 100))
	require.Error(t, p.svc.Pages.checkSpaceQuota(ctx(), p.space, 101))
}

func TestNoQuotaMeansNoCheck(t *testing.T) {
	p := newPageEnv(t)
	p.hold(t, 1<<40)
	require.NoError(t, p.svc.Pages.checkSpaceQuota(ctx(), p.space, 1<<40))
}

func TestEffectiveQuotaPrefersTheSpaceThenTheDeployment(t *testing.T) {
	space := &model.Space{QuotaBytes: 100}

	limit, fromDefault := effectiveQuota(space, 500)
	assert.EqualValues(t, 100, limit)
	assert.False(t, fromDefault)

	limit, fromDefault = effectiveQuota(&model.Space{}, 500)
	assert.EqualValues(t, 500, limit)
	assert.True(t, fromDefault)

	limit, fromDefault = effectiveQuota(&model.Space{}, 0)
	assert.EqualValues(t, 0, limit)
	assert.False(t, fromDefault)

	limit, _ = effectiveQuota(nil, 500)
	assert.EqualValues(t, 500, limit)
}

func TestRemainingReportsWhatIsLeft(t *testing.T) {
	assert.EqualValues(t, -1, SpaceUsage{}.Remaining())
	assert.EqualValues(t, 400, SpaceUsage{UsedBytes: 600, QuotaBytes: 1000}.Remaining())
	assert.EqualValues(t, 0, SpaceUsage{UsedBytes: 1000, QuotaBytes: 1000}.Remaining())
	assert.EqualValues(t, 0, SpaceUsage{UsedBytes: 2000, QuotaBytes: 1000}.Remaining(),
		"over the line reports nothing left rather than a negative number")
}
