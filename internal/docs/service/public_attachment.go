package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
)

// Attachments for anonymous visitors.
//
// A published page embeds its images, files and diagram previews by URL, and
// the visitor's browser fetches them with no session. The member route
// (/docs/attachments/:aid) cannot serve that request, so a share link and a
// public space each get an attachment route of their own, and the page is
// rendered to point at it.
//
// What such a route may serve is exactly what the page view may show: an
// attachment belongs to a page, and it is served only when that page is one
// the link or the space publishes, checked on every request against the same
// rules (the link's state, restrictions, the trash). Revoking a link or
// restricting a page therefore stops its attachments at the same moment as
// its text.
//
// A password-protected link adds one problem: the unlock token travels in a
// header, deliberately never in a URL, and an <img> cannot send a header. The
// rendered page instead carries a signature per attachment, issued only to a
// visitor who presented a valid unlock token. It is keyed like the unlock
// token (by the link's password hash, so changing or removing the password
// voids every one of them), bound to one attachment and one expiry, and it
// opens nothing but that file: a signature leaked from an access log does not
// unlock the page.

// PublicReach is what an anonymous visitor may read: the pages one share link
// or one public space publishes, in one tenant.
type PublicReach struct {
	TenantID uint64
	covers   func(ctx context.Context, page *model.Page) bool
}

// Covers reports whether a live page is published to this visitor.
func (r *PublicReach) Covers(ctx context.Context, page *model.Page) bool {
	return r != nil && page != nil && page.TenantID == r.TenantID && r.covers(ctx, page)
}

// attachmentSigParam is the query parameter a protected link's attachment
// signature travels in.
const attachmentSigParam = "sig"

// ShareAttachmentReach authorises an attachment request through a share link.
// sig is the request's attachment signature, needed only when the link has a
// password. Anything but a visible link answers not found, like the page.
func (s *PageService) ShareAttachmentReach(ctx context.Context, key, attachmentID, sig string) (
	*PublicReach, error,
) {
	link, err := s.openShare(ctx, key)
	if err != nil {
		return nil, err
	}
	if !link.state(s.attachmentSigValid(link.row, attachmentID, sig)).Visible() {
		return nil, notFound("attachment")
	}
	row, root := link.row, link.root
	return &PublicReach{TenantID: row.TenantID, covers: func(ctx context.Context, page *model.Page) bool {
		if page.SpaceID != row.SpaceID {
			return false
		}
		if page.ID == root.ID {
			return true // the link's state already proved the root unrestricted
		}
		if !row.IncludeChildren || !s.descendsFrom(ctx, row.TenantID, page, root.ID) {
			return false
		}
		return !s.anyRestricted(ctx, row.TenantID, page)
	}}, nil
}

// PublicSpaceAttachmentReach authorises an attachment request through a
// public space: any live, unrestricted page of that space.
func (s *PageService) PublicSpaceAttachmentReach(ctx context.Context, spaceID string) (*PublicReach, error) {
	space, err := s.publicSpace(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	return &PublicReach{TenantID: space.TenantID, covers: func(ctx context.Context, page *model.Page) bool {
		return page.SpaceID == space.ID && !s.anyRestricted(ctx, space.TenantID, page)
	}}, nil
}

// descendsFrom reports whether rootID is an ancestor of page. An error counts
// as "no": not being able to prove a page is inside a link is not permission
// to serve it.
func (s *PageService) descendsFrom(ctx context.Context, tenantID uint64, page *model.Page, rootID string) bool {
	ancestors, err := s.d.Repos.Pages.ListAncestors(ctx, tenantID, page.ID)
	if err != nil {
		return false
	}
	for _, a := range ancestors {
		if a.ID == rootID {
			return true
		}
	}
	return false
}

// shareAttachmentURL addresses attachments of a page rendered through a link.
// A protected link's URLs carry a signature valid as long as an unlock.
func (s *PageService) shareAttachmentURL(row *model.Share) func(attachmentID string) string {
	base := "/api/v1/docs/public/" + url.PathEscape(row.Key) + "/attachments/"
	until := time.Now().Add(unlockLifetime)
	return func(attachmentID string) string {
		u := base + url.PathEscape(attachmentID)
		if sig := s.attachmentSig(row, attachmentID, until); sig != "" {
			u += "?" + attachmentSigParam + "=" + url.QueryEscape(sig)
		}
		return u
	}
}

// publicSpaceAttachmentURL addresses attachments of a public space's pages.
func publicSpaceAttachmentURL(spaceID string) func(attachmentID string) string {
	base := "/api/v1/docs/public-spaces/" + url.PathEscape(spaceID) + "/attachments/"
	return func(attachmentID string) string { return base + url.PathEscape(attachmentID) }
}

// attachmentSig signs one attachment of a protected link until a moment; ""
// for a link without a password, which needs none.
func (s *PageService) attachmentSig(row *model.Share, attachmentID string, until time.Time) string {
	if row.PasswordHash == nil {
		return ""
	}
	exp := strconv.FormatInt(until.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(*row.PasswordHash))
	// Prefixed so a signature can never be replayed as an unlock token, which
	// is keyed the same way over "<share>|<exp>".
	mac.Write([]byte("attachment|" + row.ID + "|" + attachmentID + "|" + exp))
	return exp + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *PageService) attachmentSigValid(row *model.Share, attachmentID, sig string) bool {
	if row.PasswordHash == nil {
		return true
	}
	exp, _, found := strings.Cut(sig, ".")
	if !found {
		return false
	}
	at, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || time.Now().After(time.Unix(at, 0)) {
		return false
	}
	want := s.attachmentSig(row, attachmentID, time.Unix(at, 0))
	return want != "" && hmac.Equal([]byte(want), []byte(sig))
}

// FetchPublished serves an attachment to an anonymous visitor: only one bound
// to a live page the reach covers. Everything else — a space-level upload not
// yet placed on a page, a page in the trash, a page the link does not publish
// — is not found, indistinguishable from an id that does not exist.
func (s *AttachmentService) FetchPublished(ctx context.Context, reach *PublicReach, id string, width int) (
	*ServeResult, error,
) {
	row, err := s.d.Repos.Files.Get(ctx, reach.TenantID, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("attachment")
		}
		return nil, err
	}
	if row.PageID == nil || *row.PageID == "" {
		return nil, notFound("attachment")
	}
	page, err := s.d.Repos.Pages.Get(ctx, reach.TenantID, *row.PageID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("attachment")
		}
		return nil, err
	}
	if !reach.Covers(ctx, page) {
		return nil, notFound("attachment")
	}
	return s.serve(ctx, row, width)
}
