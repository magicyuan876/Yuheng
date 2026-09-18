package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/stretchr/testify/require"
)

func (f *fixture) attach(t *testing.T, spaceID, path, digest string, size int64,
	pageID *string,
) *model.Attachment {
	t.Helper()
	row := &model.Attachment{
		TenantID: 1, SpaceID: spaceID, PageID: pageID, FilePath: path,
		FileName: "file.bin", Mime: "application/octet-stream", SizeBytes: size,
		Kind: model.AttachmentFile,
	}
	if digest != "" {
		row.SHA256 = &digest
	}
	require.NoError(t, f.repos.Files.Create(ctx(), row))
	return row
}

func TestAttachmentDigestLookupStaysInsideOneSpace(t *testing.T) {
	f := newFixture(t)
	mine := f.attach(t, f.space.ID, "mem://1", "deadbeef", 100, nil)

	found, err := f.repos.Files.FindByDigest(ctx(), 1, f.space.ID, "deadbeef")
	require.NoError(t, err)
	require.Equal(t, mine.ID, found.ID)

	// Another space must not be handed a path into this one: its cleanup
	// would then be able to break this space's pages.
	_, err = f.repos.Files.FindByDigest(ctx(), 1, f.other.ID, "deadbeef")
	require.True(t, errors.Is(err, ErrNotFound))

	// Nor may another tenant see it at all.
	_, err = f.repos.Files.FindByDigest(ctx(), 2, f.space.ID, "deadbeef")
	require.True(t, errors.Is(err, ErrNotFound))
	_, err = f.repos.Files.Get(ctx(), 2, mine.ID)
	require.True(t, errors.Is(err, ErrNotFound))
}

func TestAttachmentDigestLookupIgnoresDeletedRows(t *testing.T) {
	f := newFixture(t)
	row := f.attach(t, f.space.ID, "mem://1", "cafe", 10, nil)
	require.NoError(t, f.repos.Files.SoftDelete(ctx(), 1, row.ID))

	_, err := f.repos.Files.FindByDigest(ctx(), 1, f.space.ID, "cafe")
	require.True(t, errors.Is(err, ErrNotFound), "a deleted row must not be deduplicated against")
	_, err = f.repos.Files.Get(ctx(), 1, row.ID)
	require.True(t, errors.Is(err, ErrNotFound))
}

func TestAttachmentCountByPathDrivesObjectLifetime(t *testing.T) {
	f := newFixture(t)
	a := f.attach(t, f.space.ID, "mem://shared", "aaa", 10, nil)
	b := f.attach(t, f.space.ID, "mem://shared", "aaa", 10, nil)

	n, err := f.repos.Files.CountByPath(ctx(), 1, "mem://shared")
	require.NoError(t, err)
	require.Equal(t, int64(2), n)

	require.NoError(t, f.repos.Files.SoftDelete(ctx(), 1, a.ID))
	n, err = f.repos.Files.CountByPath(ctx(), 1, "mem://shared")
	require.NoError(t, err)
	require.Equal(t, int64(1), n, "the object is still referenced")

	require.NoError(t, f.repos.Files.SoftDelete(ctx(), 1, b.ID))
	n, err = f.repos.Files.CountByPath(ctx(), 1, "mem://shared")
	require.NoError(t, err)
	require.Zero(t, n, "now it may be released")
}

func TestAttachmentBindingNeverStealsOneAlreadyClaimed(t *testing.T) {
	f := newFixture(t)
	first := f.page(t, "First", nil, "a0")
	second := f.page(t, "Second", nil, "a1")

	loose := f.attach(t, f.space.ID, "mem://1", "d1", 10, nil)
	claimed := f.attach(t, f.space.ID, "mem://2", "d2", 20, &first.ID)

	n, err := f.repos.Files.BindToPage(ctx(), 1, f.space.ID, second.ID,
		[]string{loose.ID, claimed.ID})
	require.NoError(t, err)
	require.Equal(t, int64(1), n, "only the unbound one moves")

	rows, err := f.repos.Files.ListByPage(ctx(), 1, second.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, loose.ID, rows[0].ID)

	rows, err = f.repos.Files.ListByPage(ctx(), 1, first.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1, "copying a reference must not take the original away")
}

func TestAttachmentBindingStaysInsideTheSpace(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "Page", nil, "a0")
	elsewhere := f.attach(t, f.other.ID, "mem://1", "d1", 10, nil)

	n, err := f.repos.Files.BindToPage(ctx(), 1, f.space.ID, page.ID, []string{elsewhere.ID})
	require.NoError(t, err)
	require.Zero(t, n, "a page cannot adopt another space's file")
}

func TestAttachmentTotalsPerPage(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "Page", nil, "a0")
	f.attach(t, f.space.ID, "mem://1", "d1", 100, &page.ID)
	f.attach(t, f.space.ID, "mem://2", "d2", 250, &page.ID)
	deleted := f.attach(t, f.space.ID, "mem://3", "d3", 999, &page.ID)
	require.NoError(t, f.repos.Files.SoftDelete(ctx(), 1, deleted.ID))

	totals, err := f.repos.Files.SumBytesByPage(ctx(), 1, []string{page.ID})
	require.NoError(t, err)
	require.Equal(t, int64(350), totals[page.ID], "deleted attachments must not be counted")
}

func TestAttachmentListForPagesIncludesDeletedOnesSoAPurgeCanReleaseThem(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "Page", nil, "a0")
	live := f.attach(t, f.space.ID, "mem://1", "d1", 10, &page.ID)
	gone := f.attach(t, f.space.ID, "mem://2", "d2", 20, &page.ID)
	require.NoError(t, f.repos.Files.SoftDelete(ctx(), 1, gone.ID))

	rows, err := f.repos.Files.ListForPages(ctx(), 1, []string{page.ID})
	require.NoError(t, err)
	require.Len(t, rows, 2)

	require.NoError(t, f.repos.Files.DeleteRows(ctx(), 1, []string{live.ID, gone.ID}))
	rows, err = f.repos.Files.ListForPages(ctx(), 1, []string{page.ID})
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestAttachmentOrphanSweepSeesOnlyUnclaimedRows(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "Page", nil, "a0")
	f.attach(t, f.space.ID, "mem://1", "d1", 10, &page.ID)
	loose := f.attach(t, f.space.ID, "mem://2", "d2", 20, nil)
	deleted := f.attach(t, f.space.ID, "mem://3", "d3", 30, nil)
	require.NoError(t, f.repos.Files.SoftDelete(ctx(), 1, deleted.ID))

	rows, err := f.repos.Files.ListOrphans(ctx(), time.Now().UTC().Add(time.Hour), 100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, loose.ID, rows[0].ID)
}

func TestAttachmentsVanishWithTheirPage(t *testing.T) {
	f := newFixture(t)
	page := f.page(t, "Page", nil, "a0")
	row := f.attach(t, f.space.ID, "mem://1", "d1", 10, &page.ID)

	_, err := f.repos.Pages.SoftDeleteSubtree(ctx(), 1, page.ID, "u1")
	require.NoError(t, err)
	_, err = f.repos.Pages.PurgeOne(ctx(), 1, page.ID)
	require.NoError(t, err)

	// The foreign key is ON DELETE SET NULL, so the row outlives the page and
	// becomes an orphan. That is why the service collects a page's
	// attachments before purging it rather than afterwards.
	stored, err := f.repos.Files.Get(ctx(), 1, row.ID)
	require.NoError(t, err)
	require.Nil(t, stored.PageID)
}
