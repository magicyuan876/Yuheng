package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/notify"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/share"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Public links.
//
// This is the only place in the module where content leaves the permission
// system entirely: a share link is a capability, and whoever holds the URL is
// the audience. Three rules follow, and each is enforced at read time rather
// than at creation time, so that no code path can forget them by failing to
// clean up after itself:
//
//  1. A restricted page is never shared. Restricting a page is a statement
//     about who may see it and a public URL contradicts it, so restricting
//     silently stops every link to that page — and to anything below it —
//     without anybody having to remember to revoke them.
//
//  2. A page in the trash is not shared, and neither is a subtree page the
//     link's own settings do not cover.
//
//  3. Nothing that is not the page itself travels with it. Comments,
//     history, the tree around it, other people's names: a visitor gets a
//     rendered document and the title, and that is all.
//
// The whole feature is additionally behind a deployment switch that is off by
// default, because a private-cloud installation that never wanted anything on
// the public internet should not acquire the ability by upgrading.

// ShareView is a link as its owner sees it. It carries the key, because the
// owner is the one person entitled to it.
type ShareView struct {
	ID               string     `json:"id"`
	PageID           string     `json:"page_id"`
	SpaceID          string     `json:"space_id"`
	Key              string     `json:"key"`
	IncludeChildren  bool       `json:"include_children"`
	AllowSearchIndex bool       `json:"allow_search_index"`
	HasPassword      bool       `json:"has_password"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	ViewCount        int64      `json:"view_count"`
	Creator          UserView   `json:"creator"`
	CreatedAt        time.Time  `json:"created_at"`
	// Live is false when the link exists but currently resolves to nothing —
	// the page was restricted or trashed after the link was made. The owner
	// should be told that rather than handing out a URL that does not work.
	Live bool `json:"live"`
}

// CreateShareInput is a new link.
type CreateShareInput struct {
	IncludeChildren  bool
	AllowSearchIndex bool
	// Password empty means no password.
	Password  string
	ExpiresAt *time.Time
}

// UpdateShareInput changes a link. Nil fields are left as they are; Password
// is three-valued, hence the pointer.
type UpdateShareInput struct {
	IncludeChildren  *bool
	AllowSearchIndex *bool
	// Password: nil leaves it, empty string removes it, a value sets it.
	Password  *string
	ExpiresAt *time.Time
	// ClearExpiry makes the link permanent, since ExpiresAt nil means "leave".
	ClearExpiry bool
}

// SharedPage is what an anonymous visitor gets.
//
// Deliberately not a PageView: that type carries a role, a space id, edit
// flags and the caller's own state, none of which a visitor should receive.
// This is a separate type so that adding a field to the internal view cannot
// accidentally publish it.
type SharedPage struct {
	Title string `json:"title"`
	Icon  string `json:"icon,omitempty"`
	HTML  string `json:"html"`
	// ShortID identifies the page within the link, for subtree navigation.
	ShortID string `json:"short_id"`
	// UpdatedAt is the body's last change, shown so a reader knows how old
	// what they are reading is.
	UpdatedAt time.Time `json:"updated_at"`
	// Children is the subtree navigation, empty unless the link includes it.
	Children []SharedRef `json:"children"`
	// Breadcrumb is the path from the shared root to this page, root first
	// and excluding this page. Empty for the root itself.
	Breadcrumb []SharedRef `json:"breadcrumb"`
	// AllowSearchIndex tells the handler which robots header to send.
	AllowSearchIndex bool `json:"allow_search_index"`
	// SpaceName is shown as the source. No slug, no id: a visitor has no use
	// for an identifier they cannot open.
	SpaceName string `json:"space_name"`
}

// SharedRef is one page in a shared subtree.
type SharedRef struct {
	ShortID string `json:"short_id"`
	Title   string `json:"title"`
	Icon    string `json:"icon,omitempty"`
}

// ShareResult pairs a visit's outcome with the page, if there is one.
type ShareResult struct {
	State share.State `json:"state"`
	Page  *SharedPage `json:"page,omitempty"`
	// UnlockToken is issued when a password was just accepted.
	UnlockToken string `json:"unlock_token,omitempty"`
}

// MaxSharesPerPage bounds how many live links one page may have. Several is
// reasonable (different audiences, different expiries); a hundred is somebody
// scripting something.
const MaxSharesPerPage = 20

// unlockLifetime is how long a proved password lasts.
const unlockLifetime = 12 * time.Hour

// ---- the owner's side ----------------------------------------------------------

// Shares lists a page's live links.
//
// Read access is enough: "this page is published on the internet" is
// something every reader of it is entitled to know, and hiding it from
// readers would mean the people most likely to notice a mistake are the ones
// who cannot see it.
func (s *PageService) Shares(ctx context.Context, actor *acl.Identity, d acl.Decision) (
	[]*ShareView, error,
) {
	if err := requireRole(d, model.RoleReader); err != nil {
		return nil, err
	}
	if s.d.Repos.Shares == nil {
		return []*ShareView{}, nil
	}
	rows, err := s.d.Repos.Shares.ListForPage(ctx, d.Page.TenantID, d.Page.ID)
	if err != nil {
		return nil, err
	}
	restricted := len(d.RestrictedAt) > 0
	out := make([]*ShareView, 0, len(rows))
	var creators []string
	for _, row := range rows {
		if row.CreatorID != nil {
			creators = append(creators, *row.CreatorID)
		}
	}
	users := s.users(ctx, creators)
	for _, row := range rows {
		out = append(out, s.shareView(row, users, !restricted))
	}
	return out, nil
}

// CreateShare publishes a page to a public URL.
func (s *PageService) CreateShare(ctx context.Context, actor *acl.Identity, d acl.Decision,
	in CreateShareInput,
) (*ShareView, error) {
	// Publishing to the internet is a bigger act than editing, but it is the
	// people who write a page who need to publish it; requiring an
	// administrator would route every external document through one person.
	if err := requireRole(d, model.RoleWriter); err != nil {
		return nil, err
	}
	if err := s.checkShareable(d); err != nil {
		return nil, err
	}

	existing, err := s.d.Repos.Shares.ListForPage(ctx, d.Page.TenantID, d.Page.ID)
	if err != nil {
		return nil, err
	}
	if len(existing) >= MaxSharesPerPage {
		return nil, invalid("this page already has %d share links", MaxSharesPerPage)
	}

	password, err := share.CleanPassword(in.Password)
	if err != nil {
		return nil, invalid("%s", err.Error())
	}
	expiry, err := share.CleanExpiry(in.ExpiresAt, time.Now())
	if err != nil {
		return nil, invalid("%s", err.Error())
	}

	row := &model.Share{
		TenantID: d.Page.TenantID, SpaceID: d.Page.SpaceID, PageID: d.Page.ID,
		IncludeChildren: in.IncludeChildren, AllowSearchIndex: in.AllowSearchIndex,
		ExpiresAt: expiry,
	}
	if id := actorID(actor); id != "" {
		row.CreatorID = &id
	}
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		h := string(hash)
		row.PasswordHash = &h
	}
	if err := s.issueKey(ctx, row); err != nil {
		return nil, err
	}

	s.audit(ctx, audit.Entry{
		TenantID: d.Page.TenantID, ActorUserID: actorID(actor), Action: audit.ShareCreated,
		SpaceID: d.Page.SpaceID, TargetType: audit.TargetPage, TargetID: d.Page.ID,
	})
	s.publish(ctx, events.New(events.ShareChanged, d.Page.TenantID).
		WithSpace(d.Page.SpaceID).WithPage(d.Page.ID).WithActor(actorID(actor)).
		With("action", "created"))
	return s.shareView(row, s.users(ctx, []string{actorID(actor)}), true), nil
}

// issueKey stores the link, retrying on the vanishingly unlikely collision
// rather than assuming one cannot happen.
func (s *PageService) issueKey(ctx context.Context, row *model.Share) error {
	for attempt := 0; attempt < 5; attempt++ {
		key, err := share.NewKey()
		if err != nil {
			return err
		}
		row.Key = key
		err = s.d.Repos.Shares.Create(ctx, row)
		if err == nil {
			return nil
		}
		if !errors.Is(err, repository.ErrDuplicate) {
			return err
		}
		row.ID = ""
	}
	return fmt.Errorf("docs: could not allocate a share key")
}

// UpdateShare changes a link's settings.
func (s *PageService) UpdateShare(ctx context.Context, actor *acl.Identity, d acl.Decision,
	shareID string, in UpdateShareInput,
) (*ShareView, error) {
	if err := requireRole(d, model.RoleWriter); err != nil {
		return nil, err
	}
	row, err := s.loadShare(ctx, d, shareID)
	if err != nil {
		return nil, err
	}

	if in.IncludeChildren != nil {
		row.IncludeChildren = *in.IncludeChildren
	}
	if in.AllowSearchIndex != nil {
		row.AllowSearchIndex = *in.AllowSearchIndex
	}
	if in.Password != nil {
		password, err := share.CleanPassword(*in.Password)
		if err != nil {
			return nil, invalid("%s", err.Error())
		}
		if password == "" {
			// Removing the password also invalidates every unlock token, for
			// free: the tokens are signed with the hash. See unlockToken.
			row.PasswordHash = nil
		} else {
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return nil, err
			}
			h := string(hash)
			row.PasswordHash = &h
		}
	}
	switch {
	case in.ClearExpiry:
		row.ExpiresAt = nil
	case in.ExpiresAt != nil:
		expiry, err := share.CleanExpiry(in.ExpiresAt, time.Now())
		if err != nil {
			return nil, invalid("%s", err.Error())
		}
		row.ExpiresAt = expiry
	}

	if err := s.d.Repos.Shares.Update(ctx, row); err != nil {
		return nil, err
	}
	s.audit(ctx, audit.Entry{
		TenantID: d.Page.TenantID, ActorUserID: actorID(actor), Action: audit.ShareUpdated,
		SpaceID: d.Page.SpaceID, TargetType: audit.TargetPage, TargetID: d.Page.ID,
	})
	s.publish(ctx, events.New(events.ShareChanged, d.Page.TenantID).
		WithSpace(d.Page.SpaceID).WithPage(d.Page.ID).WithActor(actorID(actor)).
		With("action", "updated"))
	return s.shareView(row, s.users(ctx, []string{actorID(actor)}), len(d.RestrictedAt) == 0), nil
}

// RevokeShare turns a link off for good.
func (s *PageService) RevokeShare(ctx context.Context, actor *acl.Identity, d acl.Decision,
	shareID string,
) error {
	if err := requireRole(d, model.RoleWriter); err != nil {
		return err
	}
	if _, err := s.loadShare(ctx, d, shareID); err != nil {
		return err
	}
	if err := s.d.Repos.Shares.Revoke(ctx, d.Page.TenantID, shareID, time.Now()); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound("share")
		}
		return err
	}
	s.audit(ctx, audit.Entry{
		TenantID: d.Page.TenantID, ActorUserID: actorID(actor), Action: audit.ShareRevoked,
		SpaceID: d.Page.SpaceID, TargetType: audit.TargetPage, TargetID: d.Page.ID,
	})
	s.publish(ctx, events.New(events.ShareChanged, d.Page.TenantID).
		WithSpace(d.Page.SpaceID).WithPage(d.Page.ID).WithActor(actorID(actor)).
		With("action", "revoked"))
	return nil
}

// RevokeSharesForPages turns off links to pages that are leaving.
//
// Called when pages go to the trash. A trashed page already fails rule 2 at
// read time, so this is not what makes it safe — it is what makes the link
// list honest, and what stops a restore from silently republishing something.
func (s *PageService) RevokeSharesForPages(ctx context.Context, tenantID uint64, pageIDs []string) {
	if s.d.Repos.Shares == nil || len(pageIDs) == 0 {
		return
	}
	if _, err := s.d.Repos.Shares.RevokeForPages(ctx, tenantID, pageIDs, time.Now()); err != nil {
		// The read-time checks already refuse these pages, so failing here
		// costs tidiness rather than safety, and the delete has committed.
		logger.Warnf(ctx, "docs: revoking share links for removed pages failed: %v", err)
	}
}

// ---- the visitor's side --------------------------------------------------------

// ResolveShare is an anonymous visit to a link.
//
// unlockToken is whatever the visitor's browser kept from a previous password
// prompt; an absent or stale one simply means they are asked again.
func (s *PageService) ResolveShare(ctx context.Context, key, unlockToken, wantShortID string) (
	*ShareResult, error,
) {
	link, err := s.openShare(ctx, key)
	if err != nil {
		return nil, err
	}
	state := link.state(s.unlockValid(link.row, unlockToken))
	if !state.Visible() {
		return &ShareResult{State: state}, nil
	}

	view, err := s.renderShared(ctx, link, wantShortID)
	if err != nil {
		return nil, err
	}
	if view == nil {
		// Asked for a page the link does not cover. Not an error with a
		// different shape: from outside, a page outside the link and a page
		// that does not exist are the same thing.
		return &ShareResult{State: share.StateGone}, nil
	}
	s.countVisit(ctx, link.row)
	return &ShareResult{State: state, Page: view}, nil
}

// openedShare is a link looked up by its key, with the facts its state is
// computed from. Every anonymous entry point — the page, the password prompt,
// an attachment — starts here, so none can skip a rule the others apply.
type openedShare struct {
	row        *model.Share
	root       *model.Page // nil when the page is no longer live
	restricted bool
}

func (s *PageService) openShare(ctx context.Context, key string) (*openedShare, error) {
	if !s.d.PublicSharing {
		return nil, notFound("share")
	}
	clean, err := share.NormaliseKey(key)
	if err != nil {
		return nil, notFound("share")
	}
	row, err := s.d.Repos.Shares.GetByKey(ctx, clean)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("share")
		}
		return nil, err
	}
	root, _ := s.sharedRootPage(ctx, row)
	return &openedShare{row: row, root: root, restricted: root != nil && s.anyRestricted(ctx, row.TenantID, root)}, nil
}

// state evaluates the link now; unlocked says whether the visitor proved the
// password (ignored for a link without one).
func (l *openedShare) state(unlocked bool) share.State {
	return share.Evaluate(share.Link{
		ExpiresAt: l.row.ExpiresAt, RevokedAt: l.row.RevokedAt,
		HasPassword: l.row.PasswordHash != nil,
		PageLive:    l.root != nil, PageRestricted: l.restricted,
	}, time.Now(), unlocked)
}

// UnlockShare checks a password and issues a token.
func (s *PageService) UnlockShare(ctx context.Context, key, password string) (*ShareResult, error) {
	link, err := s.openShare(ctx, key)
	if err != nil {
		return nil, err
	}
	// The state is checked BEFORE the password, so a revoked or expired link
	// cannot be used as an oracle for whether a guess was right.
	if state := link.state(false); state != share.StatePassword {
		return &ShareResult{State: state}, nil
	}
	if bcrypt.CompareHashAndPassword([]byte(*link.row.PasswordHash), []byte(password)) != nil {
		return nil, forbidden("that password does not open this link")
	}
	return &ShareResult{
		State: share.StateOK, UnlockToken: s.unlockToken(link.row, time.Now().Add(unlockLifetime)),
	}, nil
}

// ---- helpers -------------------------------------------------------------------

// unlockToken proves a password was given, without any server-side session.
//
// It is signed with the link's own password hash, which has two useful
// consequences and no key management: changing the password invalidates every
// outstanding token, and removing the password does too. The hash never
// leaves the server, and the token is scoped to one link and one expiry.
func (s *PageService) unlockToken(row *model.Share, until time.Time) string {
	if row.PasswordHash == nil {
		return ""
	}
	exp := strconv.FormatInt(until.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(*row.PasswordHash))
	mac.Write([]byte(row.ID + "|" + exp))
	return exp + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *PageService) unlockValid(row *model.Share, token string) bool {
	if row.PasswordHash == nil {
		return true
	}
	exp, _, found := strings.Cut(token, ".")
	if !found {
		return false
	}
	at, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || time.Now().After(time.Unix(at, 0)) {
		return false
	}
	want := s.unlockToken(row, time.Unix(at, 0))
	return want != "" && hmac.Equal([]byte(want), []byte(token))
}

// sharedRootPage loads the page a link points at, reporting whether it is
// still live.
func (s *PageService) sharedRootPage(ctx context.Context, row *model.Share) (*model.Page, bool) {
	page, err := s.d.Repos.Pages.Get(ctx, row.TenantID, row.PageID)
	if err != nil || page == nil {
		return nil, false
	}
	return page, true
}

// checkShareable refuses to publish something that is not publishable.
func (s *PageService) checkShareable(d acl.Decision) error {
	if !s.d.PublicSharing {
		return forbidden("public sharing is switched off in this deployment")
	}
	if s.d.Repos.Shares == nil {
		return notFound("page")
	}
	if len(d.RestrictedAt) > 0 {
		return invalid("a page with restricted permissions cannot be shared publicly")
	}
	return nil
}

func (s *PageService) loadShare(ctx context.Context, d acl.Decision, shareID string) (
	*model.Share, error,
) {
	row, err := s.d.Repos.Shares.Get(ctx, d.Page.TenantID, shareID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("share")
		}
		return nil, err
	}
	// Naming another page's link must not reach it, the rule labels,
	// revisions and comments all follow.
	if row.PageID != d.Page.ID {
		return nil, notFound("share")
	}
	return row, nil
}

func (s *PageService) shareView(row *model.Share, users map[string]*types.User, live bool) *ShareView {
	v := &ShareView{
		ID: row.ID, PageID: row.PageID, SpaceID: row.SpaceID, Key: row.Key,
		IncludeChildren: row.IncludeChildren, AllowSearchIndex: row.AllowSearchIndex,
		HasPassword: row.PasswordHash != nil, ExpiresAt: row.ExpiresAt,
		ViewCount: row.ViewCount, CreatedAt: row.CreatedAt, Live: live,
	}
	if row.CreatorID != nil {
		v.Creator = userView(*row.CreatorID, users)
	}
	return v
}

// renderShared builds what the visitor sees, or nil when the requested page
// is outside the link.
func (s *PageService) renderShared(ctx context.Context, link *openedShare, wantShortID string) (*SharedPage, error) {
	row, root := link.row, link.root
	page := root
	var breadcrumb []SharedRef

	if wantShortID != "" && wantShortID != root.ShortID {
		if !row.IncludeChildren {
			return nil, nil
		}
		found, trail, err := s.findInSharedSubtree(ctx, row, root, wantShortID)
		if err != nil || found == nil {
			return nil, err
		}
		page, breadcrumb = found, trail
	}

	inLink, err := s.sharedTitles(ctx, row, root)
	if err != nil {
		return nil, err
	}
	html, err := s.renderForVisitor(page, inLink, s.shareAttachmentURL(row))
	if err != nil {
		return nil, err
	}
	out := &SharedPage{
		Title: page.Title, Icon: text(page.Icon), HTML: html, ShortID: page.ShortID,
		UpdatedAt: when(page.ContentUpdatedAt, page.UpdatedAt), Breadcrumb: breadcrumb,
		AllowSearchIndex: row.AllowSearchIndex, Children: []SharedRef{},
	}
	if space, err := s.d.Repos.Spaces.Get(ctx, row.TenantID, row.SpaceID); err == nil && space != nil {
		out.SpaceName = space.Name
	}
	if row.IncludeChildren {
		kids, err := s.sharedChildren(ctx, row, page.ID)
		if err != nil {
			return nil, err
		}
		out.Children = kids
	}
	return out, nil
}

// sharedChildren lists a shared page's children, leaving out anything
// restricted. Rule 1 applies all the way down.
func (s *PageService) sharedChildren(ctx context.Context, row *model.Share, parentID string) (
	[]SharedRef, error,
) {
	kids, err := s.d.Repos.Pages.ListChildren(ctx, row.TenantID, row.SpaceID, &parentID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(kids))
	for _, k := range kids {
		ids = append(ids, k.ID)
	}
	restricted, err := s.d.Repos.Access.Restricted(ctx, row.TenantID, ids)
	if err != nil {
		return nil, err
	}
	out := make([]SharedRef, 0, len(kids))
	for _, k := range kids {
		if restricted[k.ID] {
			continue
		}
		out = append(out, SharedRef{ShortID: k.ShortID, Title: k.Title, Icon: text(k.Icon)})
	}
	return out, nil
}

// findInSharedSubtree walks down from the shared root looking for a page,
// refusing to pass through a restricted level.
//
// Walking down from the root rather than looking the page up and checking its
// ancestors is the point: it can only ever find pages the link actually
// covers, so a page id from somewhere else cannot be made to resolve.
func (s *PageService) findInSharedSubtree(ctx context.Context, row *model.Share, root *model.Page,
	wantShortID string,
) (*model.Page, []SharedRef, error) {
	type step struct {
		page  *model.Page
		trail []SharedRef
	}
	queue := []step{{page: root, trail: nil}}
	// Bounded so a pathological tree cannot turn one request into a scan of
	// the whole space.
	for visited := 0; len(queue) > 0 && visited < MaxSharedSubtreePages; visited++ {
		current := queue[0]
		queue = queue[1:]

		parentID := current.page.ID
		kids, err := s.d.Repos.Pages.ListChildren(ctx, row.TenantID, row.SpaceID, &parentID)
		if err != nil {
			return nil, nil, err
		}
		ids := make([]string, 0, len(kids))
		for _, k := range kids {
			ids = append(ids, k.ID)
		}
		restricted, err := s.d.Repos.Access.Restricted(ctx, row.TenantID, ids)
		if err != nil {
			return nil, nil, err
		}
		trail := append(append([]SharedRef{}, current.trail...),
			SharedRef{ShortID: current.page.ShortID, Title: current.page.Title, Icon: text(current.page.Icon)})
		for _, k := range kids {
			if restricted[k.ID] {
				continue
			}
			if k.ShortID == wantShortID {
				return k, trail, nil
			}
			queue = append(queue, step{page: k, trail: trail})
		}
	}
	return nil, nil, nil
}

// MaxSharedSubtreePages bounds a public subtree walk.
const MaxSharedSubtreePages = 500

// countVisit records the visit and tells the link's owner when it reaches a
// milestone. §8.3's fifth row.
func (s *PageService) countVisit(ctx context.Context, row *model.Share) {
	views, err := s.d.Repos.Shares.CountViewed(ctx, row.ID)
	if err != nil {
		logger.Warnf(ctx, "docs: counting a share visit failed: %v", err)
		return
	}
	// Audited on the first visit and at each milestone, not on every one.
	//
	// "Content of ours was read from the public internet" is a fact a
	// compliance reviewer needs, and the first visit is when it becomes true.
	// A row per visit would make the audit log unreadable and would put the
	// module's highest write rate on its slowest table; the milestones keep
	// the trail proportionate to how far the content actually travelled.
	milestone := share.Milestone(views)
	if views == 1 || milestone > 0 {
		s.audit(ctx, audit.Entry{
			TenantID: row.TenantID, Action: audit.ShareAccessed,
			SpaceID: row.SpaceID, TargetType: audit.TargetShare, TargetID: row.ID,
		})
	}
	if milestone == 0 || row.CreatorID == nil {
		return
	}
	s.notifyShareMilestone(ctx, row, milestone)
}

// embedURL is the render hook that decides what an embed node frames.
//
// Checked again here rather than trusted from when the page was saved, so
// that narrowing the deployment's embed policy takes effect on the next
// public view rather than on the next save.
func (s *PageService) embedURL(provider, rawURL string) string {
	if s.d.Embeds == nil {
		return ""
	}
	return s.d.Embeds.EmbedURL(provider, rawURL)
}

// sharedTitles is the set of page ids this link covers, with their titles,
// so a page link inside the document renders only when its destination is
// also shared.
func (s *PageService) sharedTitles(ctx context.Context, row *model.Share, root *model.Page) (
	map[string]string, error,
) {
	out := map[string]string{root.ID: root.Title}
	if !row.IncludeChildren {
		return out, nil
	}
	frontier := []string{root.ID}
	for visited := 0; len(frontier) > 0 && visited < MaxSharedSubtreePages; {
		parentID := frontier[0]
		frontier = frontier[1:]
		visited++

		kids, err := s.d.Repos.Pages.ListChildren(ctx, row.TenantID, row.SpaceID, &parentID)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(kids))
		for _, k := range kids {
			ids = append(ids, k.ID)
		}
		restricted, err := s.d.Repos.Access.Restricted(ctx, row.TenantID, ids)
		if err != nil {
			return nil, err
		}
		for _, k := range kids {
			if restricted[k.ID] {
				continue
			}
			out[k.ID] = k.Title
			frontier = append(frontier, k.ID)
		}
	}
	return out, nil
}

// notifyShareMilestone tells the link's owner their page is being read.
//
// This is §8.3's fifth notification row. It is a threshold rather than an
// event because a notification per visit would be useless on a link that
// works and silent on one that does not.
func (s *PageService) notifyShareMilestone(ctx context.Context, row *model.Share, milestone int64) {
	if s.d.Repos.Notices == nil || row.CreatorID == nil {
		return
	}
	page, err := s.d.Repos.Pages.Get(ctx, row.TenantID, row.PageID)
	if err != nil {
		return
	}
	// Only the person who made the link: it is their link, and everybody who
	// watches the page has not asked to hear about its traffic.
	s.deliver(ctx, notification{
		kind: notify.ShareViewed, page: page,
		payload: map[string]any{
			"title": page.Title, "views": milestone, "share_id": row.ID,
		},
	}, []string{*row.CreatorID})
}

// text dereferences an optional string column.
func text(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// when prefers the first timestamp and falls back to the second.
func when(first *time.Time, fallback time.Time) time.Time {
	if first != nil {
		return *first
	}
	return fallback
}
