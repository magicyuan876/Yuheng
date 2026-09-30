package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/render"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/docs/schema"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Templates: a page body worth starting from again.
//
// Two scopes, and the difference is who the template is for rather than
// where it is stored: a SPACE template is one team's way of writing a thing,
// and a TENANT template is the organisation's. Listing for a space returns
// both, because that is the set somebody creating a page there can choose
// from; and the tenant-wide ones sort first, because they are the ones a new
// member is meant to find.
//
// ---- decision: a template carries structure and words, not ties
//
// Saving a page as a template runs its body through render.Portable, which
// removes the parts that cannot travel: page links and block references
// (they point at one specific page, and the reader of the new document may
// not be allowed to open it), mentions (a template that greets Alice by name
// every time it is used is wrong in a way nobody notices until it is
// embarrassing) and attachments (a file belongs to the space it was uploaded
// to — that is what makes its permissions and its cleanup tractable).
//
// The alternative was copying attachments into the target space on every
// use, which means either a cross-space reference that one space's tidying
// can break, or silently duplicating somebody's file into a space they never
// put it in. Neither is worth it for a feature whose job is to save typing.
// The client is told what will be dropped BEFORE it saves, so this is a
// choice somebody makes rather than a surprise they discover.

// MaxTemplatesPerScope bounds a template library. Past this it is not a
// library, it is a search problem nobody asked for.
const MaxTemplatesPerScope = 200

// MaxTemplateNameRunes bounds a template's name.
const MaxTemplateNameRunes = 120

// MaxTemplateCategoryRunes bounds the grouping label.
const MaxTemplateCategoryRunes = 64

// TemplateView is a template as a client sees it.
type TemplateView struct {
	ID          string  `json:"id"`
	SpaceID     *string `json:"space_id,omitempty"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Icon        string  `json:"icon,omitempty"`
	Category    string  `json:"category,omitempty"`
	// Shared is true for a tenant-wide template: the flag a client needs to
	// say where this came from without knowing how scopes are stored.
	Shared bool `json:"shared"`
	// Content is omitted from listings and present on a single read.
	Content   json.RawMessage `json:"content,omitempty"`
	Creator   UserView        `json:"creator"`
	CanEdit   bool            `json:"can_edit"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// CreateTemplateInput makes a template, either from a body or from a page.
type CreateTemplateInput struct {
	// SpaceID empty makes it tenant-wide, which needs a tenant administrator.
	SpaceID     string
	Name        string
	Description string
	Icon        string
	Category    string
	// Content is the body. Exactly one of Content and FromPageID.
	Content json.RawMessage
	// FromPageID saves an existing page's body, which the caller must be able
	// to read.
	FromPageID string
}

// UpdateTemplateInput is a partial change; nil leaves a field alone.
type UpdateTemplateInput struct {
	Name        *string
	Description *string
	Icon        *string
	Category    *string
	Content     json.RawMessage
}

// Templates lists what somebody may start a page from in a space.
func (s *PageService) Templates(ctx context.Context, actor *acl.Identity, spaceID string) (
	[]*TemplateView, error,
) {
	if s.d.Repos.Templates == nil {
		return []*TemplateView{}, nil
	}
	// A space id is checked rather than trusted: without this, naming any
	// space would list its templates, and a template's name and description
	// are written by people who assumed an audience.
	if spaceID != "" {
		if _, _, err := s.spaceFor(ctx, actor, spaceID, model.RoleReader); err != nil {
			return nil, err
		}
	}
	rows, err := s.d.Repos.Templates.ListFor(ctx, actor.TenantID, spaceID)
	if err != nil {
		return nil, err
	}
	return s.templateViews(ctx, actor, rows, false), nil
}

// Template reads one, with its body.
func (s *PageService) Template(ctx context.Context, actor *acl.Identity, id string) (
	*TemplateView, error,
) {
	row, err := s.loadTemplate(ctx, actor, id, model.RoleReader)
	if err != nil {
		return nil, err
	}
	return s.templateViews(ctx, actor, []*model.Template{row}, true)[0], nil
}

// CreateTemplate saves a body for reuse.
func (s *PageService) CreateTemplate(ctx context.Context, actor *acl.Identity,
	in CreateTemplateInput,
) (*TemplateView, error) {
	if s.d.Repos.Templates == nil {
		return nil, notFound("template")
	}
	if err := s.checkTemplateScope(ctx, actor, in.SpaceID); err != nil {
		return nil, err
	}
	name, err := cleanTemplateName(in.Name)
	if err != nil {
		return nil, err
	}

	body, err := s.templateBody(ctx, actor, in.Content, in.FromPageID)
	if err != nil {
		return nil, err
	}

	existing, err := s.d.Repos.Templates.ListFor(ctx, actor.TenantID, in.SpaceID)
	if err != nil {
		return nil, err
	}
	if len(existing) >= MaxTemplatesPerScope {
		return nil, invalid("there are already %d templates here", MaxTemplatesPerScope)
	}

	row := &model.Template{
		TenantID: actor.TenantID, Name: name,
		Description: strings.TrimSpace(in.Description),
		Category:    cleanTemplateCategory(in.Category),
		Content:     body.content, TextContent: body.text,
	}
	if in.SpaceID != "" {
		row.SpaceID = &in.SpaceID
	}
	if icon := strings.TrimSpace(in.Icon); icon != "" {
		row.Icon = &icon
	}
	if id := actorID(actor); id != "" {
		row.CreatorID, row.LastEditorID = &id, &id
	}
	if err := s.d.Repos.Templates.Create(ctx, row); err != nil {
		return nil, err
	}

	s.auditTemplate(ctx, actor, row, audit.TemplateSaved)
	return s.templateViews(ctx, actor, []*model.Template{row}, true)[0], nil
}

// UpdateTemplate changes one.
func (s *PageService) UpdateTemplate(ctx context.Context, actor *acl.Identity, id string,
	in UpdateTemplateInput,
) (*TemplateView, error) {
	row, err := s.loadTemplate(ctx, actor, id, model.RoleWriter)
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		if row.Name, err = cleanTemplateName(*in.Name); err != nil {
			return nil, err
		}
	}
	if in.Description != nil {
		row.Description = strings.TrimSpace(*in.Description)
	}
	if in.Category != nil {
		row.Category = cleanTemplateCategory(*in.Category)
	}
	if in.Icon != nil {
		if icon := strings.TrimSpace(*in.Icon); icon == "" {
			row.Icon = nil
		} else {
			row.Icon = &icon
		}
	}
	if len(in.Content) > 0 {
		body, err := s.templateBody(ctx, actor, in.Content, "")
		if err != nil {
			return nil, err
		}
		row.Content, row.TextContent = body.content, body.text
	}
	if editor := actorID(actor); editor != "" {
		row.LastEditorID = &editor
	}

	if err := s.d.Repos.Templates.Update(ctx, row); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("template")
		}
		return nil, err
	}
	s.auditTemplate(ctx, actor, row, audit.TemplateSaved)
	return s.templateViews(ctx, actor, []*model.Template{row}, true)[0], nil
}

// DeleteTemplate removes one from the library.
func (s *PageService) DeleteTemplate(ctx context.Context, actor *acl.Identity, id string) error {
	row, err := s.loadTemplate(ctx, actor, id, model.RoleAdmin)
	if err != nil {
		return err
	}
	if err := s.d.Repos.Templates.Delete(ctx, actor.TenantID, id, time.Now()); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound("template")
		}
		return err
	}
	s.auditTemplate(ctx, actor, row, audit.TemplateDelete)
	return nil
}

// ---- helpers -------------------------------------------------------------------

// templateBody validates a body and makes it portable.
type templateBody struct {
	content model.JSON
	text    string
}

func (s *PageService) templateBody(ctx context.Context, actor *acl.Identity,
	content json.RawMessage, fromPageID string,
) (*templateBody, error) {
	if len(content) > 0 && fromPageID != "" {
		return nil, invalid("give either a body or a page to save, not both")
	}

	raw := content
	if fromPageID != "" {
		// Read through the permission resolver rather than the repository:
		// saving a page as a template is a read of that page and must be
		// refused exactly when reading it would be.
		d, err := s.d.Resolver.Page(ctx, actor, fromPageID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, notFound("page")
			}
			return nil, err
		}
		if err := requireRole(d, model.RoleReader); err != nil {
			return nil, err
		}
		raw = json.RawMessage(d.Page.Content)
	}
	if len(raw) == 0 {
		raw = json.RawMessage(EmptyDocument)
	}

	doc, _, err := schema.Default().Validate(raw)
	if err != nil {
		return nil, invalid("the template body is not a valid document: %s", err.Error())
	}
	portable := render.Portable(doc)
	encoded, err := json.Marshal(portable)
	if err != nil {
		return nil, err
	}
	// Validated again after stripping, because storing something that cannot
	// be read back is the one failure a template must not have.
	if _, _, err := schema.Default().Validate(encoded); err != nil {
		return nil, invalid("the template body could not be made portable: %s", err.Error())
	}
	return &templateBody{content: model.JSON(encoded), text: render.Text(portable)}, nil
}

// checkTemplateScope decides whether this caller may add to a scope.
//
// A space template needs write access to that space — the same right as
// writing the pages it will produce. A tenant-wide one needs a tenant
// administrator, because it appears in every space including ones its author
// cannot see.
func (s *PageService) checkTemplateScope(ctx context.Context, actor *acl.Identity, spaceID string) error {
	if spaceID == "" {
		if !actor.IsTenantAdmin() {
			return forbidden("a shared template needs a workspace administrator")
		}
		return nil
	}
	_, _, err := s.spaceFor(ctx, actor, spaceID, model.RoleWriter)
	return err
}

// loadTemplate reads one and checks the caller holds min in its scope.
func (s *PageService) loadTemplate(ctx context.Context, actor *acl.Identity, id string,
	min model.SpaceRole,
) (*model.Template, error) {
	if s.d.Repos.Templates == nil || id == "" {
		return nil, notFound("template")
	}
	row, err := s.d.Repos.Templates.Get(ctx, actor.TenantID, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("template")
		}
		return nil, err
	}
	if row.SpaceID == nil {
		// Tenant-wide: every member may read and use it; only an
		// administrator may change or remove it.
		if min == model.RoleReader {
			return row, nil
		}
		if !actor.IsTenantAdmin() {
			return nil, forbidden("a shared template needs a workspace administrator")
		}
		return row, nil
	}
	if _, _, err := s.spaceFor(ctx, actor, *row.SpaceID, min); err != nil {
		return nil, err
	}
	return row, nil
}

// spaceFor loads a space and checks the caller's role in it, reporting a
// missing space and an invisible one the same way.
func (s *PageService) spaceFor(ctx context.Context, actor *acl.Identity, spaceID string,
	min model.SpaceRole,
) (*model.Space, model.SpaceRole, error) {
	space, err := s.d.Repos.Spaces.Get(ctx, actor.TenantID, spaceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, model.RoleNone, notFound("space")
		}
		return nil, model.RoleNone, err
	}
	role, err := s.d.Resolver.SpaceRole(ctx, actor, space)
	if err != nil {
		return nil, model.RoleNone, err
	}
	if err := requireSpaceRole(role, min); err != nil {
		return nil, role, err
	}
	return space, role, nil
}

func (s *PageService) templateViews(ctx context.Context, actor *acl.Identity,
	rows []*model.Template, withContent bool,
) []*TemplateView {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.CreatorID != nil {
			ids = append(ids, *row.CreatorID)
		}
	}
	users := s.users(ctx, ids)

	out := make([]*TemplateView, 0, len(rows))
	for _, row := range rows {
		v := &TemplateView{
			ID: row.ID, SpaceID: row.SpaceID, Name: row.Name,
			Description: row.Description, Icon: text(row.Icon),
			Category: row.Category, Shared: row.SpaceID == nil,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			CanEdit: s.canEditTemplate(ctx, actor, row),
		}
		if row.CreatorID != nil {
			v.Creator = userView(*row.CreatorID, users)
		}
		if withContent {
			v.Content = json.RawMessage(row.Content)
		}
		out = append(out, v)
	}
	return out
}

// canEditTemplate answers the flag a client draws a pencil from. A cheap
// approximation is not good enough here: it is re-checked on every write.
func (s *PageService) canEditTemplate(ctx context.Context, actor *acl.Identity,
	row *model.Template,
) bool {
	if row.SpaceID == nil {
		return actor.IsTenantAdmin()
	}
	_, _, err := s.spaceFor(ctx, actor, *row.SpaceID, model.RoleWriter)
	return err == nil
}

func (s *PageService) auditTemplate(ctx context.Context, actor *acl.Identity, row *model.Template,
	action types.AuditAction,
) {
	entry := audit.Entry{
		TenantID: row.TenantID, ActorUserID: actorID(actor), Action: action,
		TargetType: audit.TargetTemplate, TargetID: row.ID,
	}
	if row.SpaceID != nil {
		entry.SpaceID = *row.SpaceID
	}
	s.audit(ctx, entry)
	// A space template is news only to that space's readers; carrying the
	// space lets the event stream scope it. A workspace template has none.
	ev := events.New(events.TemplateChanged, row.TenantID).WithActor(actorID(actor)).With("template_id", row.ID)
	if row.SpaceID != nil {
		ev = ev.WithSpace(*row.SpaceID)
	}
	s.publish(ctx, ev)
}

func cleanTemplateName(raw string) (string, error) {
	name := strings.Join(strings.Fields(raw), " ")
	if name == "" {
		return "", invalid("a template needs a name")
	}
	if len([]rune(name)) > MaxTemplateNameRunes {
		return "", invalid("a template's name may not exceed %d characters", MaxTemplateNameRunes)
	}
	return name, nil
}

func cleanTemplateCategory(raw string) string {
	category := strings.Join(strings.Fields(raw), " ")
	if len([]rune(category)) > MaxTemplateCategoryRunes {
		category = string([]rune(category)[:MaxTemplateCategoryRunes])
	}
	return category
}

// templateFor resolves a template for use in one space.
//
// Separate from loadTemplate because using a template somewhere is a
// different question from reading it: a space's own template belongs to that
// space and must not be applied in another one, where its author never
// intended it and where the caller's right to read it came from elsewhere.
func (s *PageService) templateFor(ctx context.Context, actor *acl.Identity,
	templateID, spaceID string,
) (*model.Template, error) {
	row, err := s.loadTemplate(ctx, actor, templateID, model.RoleReader)
	if err != nil {
		return nil, err
	}
	if row.SpaceID != nil && *row.SpaceID != spaceID {
		return nil, notFound("template")
	}
	return row, nil
}
