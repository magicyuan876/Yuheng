package service

import (
	"context"
	"errors"
	"strings"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
)

// Labels.
//
// A label belongs to a space, not to the tenant. Two teams both wanting a
// "draft" label get two labels rather than an argument about one — and a
// label list cannot report the existence of pages in a space somebody cannot
// see, because it is read through the space's own permissions.
//
// Putting a label on a page is a write to the page; managing the labels
// themselves is a write to the space. Those are different permissions and
// they are checked separately.

// MaxLabelsPerSpace bounds a space's vocabulary. A tagging scheme past this
// is a filing system nobody can hold in their head, and the bound is what
// stops a list becoming unusable rather than a technical limit.
const MaxLabelsPerSpace = 200

// MaxLabelsPerPage bounds how many a page may carry.
const MaxLabelsPerPage = 20

// MaxLabelNameRunes bounds a label's name.
const MaxLabelNameRunes = 32

// LabelColors is the closed set a label may be coloured with.
//
// Closed rather than free-form, because a colour reaches every list the label
// appears in and an arbitrary value would be somebody else's contrast problem
// — and because a fixed set can be themed for dark mode in one place.
var LabelColors = []string{"gray", "red", "orange", "yellow", "green", "teal", "blue", "purple", "pink"}

// LabelView is a label as a client sees it.
type LabelView struct {
	ID        string `json:"id"`
	SpaceID   string `json:"space_id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	PageCount int64  `json:"page_count"`
}

// CreateLabelInput is a new label.
type CreateLabelInput struct {
	Name  string
	Color string
}

// Labels lists a space's labels with how many pages carry each.
func (s *PageService) Labels(ctx context.Context, actor *acl.Identity, space *model.Space,
	role model.SpaceRole,
) ([]*LabelView, error) {
	if err := requireSpaceRole(role, model.RoleReader); err != nil {
		return nil, err
	}
	if s.d.Repos.Labels == nil {
		return []*LabelView{}, nil
	}
	rows, err := s.d.Repos.Labels.ListForSpace(ctx, space.TenantID, space.ID)
	if err != nil {
		return nil, err
	}
	out := make([]*LabelView, 0, len(rows))
	for _, row := range rows {
		out = append(out, &LabelView{
			ID: row.ID, SpaceID: row.SpaceID, Name: row.Name,
			Color: row.Color, PageCount: row.PageCount,
		})
	}
	return out, nil
}

// CreateLabel adds one to a space.
func (s *PageService) CreateLabel(ctx context.Context, actor *acl.Identity, space *model.Space,
	role model.SpaceRole, in CreateLabelInput,
) (*LabelView, error) {
	// A writer may label pages and may make the labels to do it with;
	// requiring an admin would mean asking permission to file your own work.
	if err := requireSpaceRole(role, model.RoleWriter); err != nil {
		return nil, err
	}
	if s.d.Repos.Labels == nil {
		return nil, notFound("space")
	}

	name, err := cleanLabelName(in.Name)
	if err != nil {
		return nil, err
	}
	color, err := cleanLabelColor(in.Color)
	if err != nil {
		return nil, err
	}

	existing, err := s.d.Repos.Labels.ListForSpace(ctx, space.TenantID, space.ID)
	if err != nil {
		return nil, err
	}
	if len(existing) >= MaxLabelsPerSpace {
		return nil, invalid("a space may have at most %d labels", MaxLabelsPerSpace)
	}

	label := &model.Label{TenantID: space.TenantID, SpaceID: space.ID, Name: name, Color: color}
	if err := s.d.Repos.Labels.Create(ctx, label); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, conflict("a label called %q already exists here", name)
		}
		return nil, err
	}
	s.publish(ctx, events.New(events.LabelChanged, space.TenantID).
		WithSpace(space.ID).WithActor(actorID(actor)).
		With("label_id", label.ID).With("action", "created"))
	return &LabelView{ID: label.ID, SpaceID: label.SpaceID, Name: label.Name, Color: label.Color}, nil
}

// UpdateLabel renames a label or recolours it.
func (s *PageService) UpdateLabel(ctx context.Context, actor *acl.Identity, space *model.Space,
	role model.SpaceRole, labelID string, in CreateLabelInput,
) (*LabelView, error) {
	if err := requireSpaceRole(role, model.RoleWriter); err != nil {
		return nil, err
	}
	label, err := s.loadLabel(ctx, space, labelID)
	if err != nil {
		return nil, err
	}

	name := label.Name
	if strings.TrimSpace(in.Name) != "" {
		if name, err = cleanLabelName(in.Name); err != nil {
			return nil, err
		}
	}
	color := label.Color
	if in.Color != "" {
		if color, err = cleanLabelColor(in.Color); err != nil {
			return nil, err
		}
	}

	if err := s.d.Repos.Labels.Update(ctx, space.TenantID, labelID, name, color); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, conflict("a label called %q already exists here", name)
		}
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("label")
		}
		return nil, err
	}
	s.publish(ctx, events.New(events.LabelChanged, space.TenantID).
		WithSpace(space.ID).WithActor(actorID(actor)).
		With("label_id", labelID).With("action", "updated"))
	return &LabelView{ID: labelID, SpaceID: space.ID, Name: name, Color: color}, nil
}

// DeleteLabel removes a label from the space and from every page carrying it.
func (s *PageService) DeleteLabel(ctx context.Context, actor *acl.Identity, space *model.Space,
	role model.SpaceRole, labelID string,
) error {
	// Deleting takes the label off everybody's pages, which is a different
	// kind of act from making one: an admin's.
	if err := requireSpaceRole(role, model.RoleAdmin); err != nil {
		return err
	}
	if _, err := s.loadLabel(ctx, space, labelID); err != nil {
		return err
	}
	if err := s.d.Repos.Labels.Delete(ctx, space.TenantID, labelID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return notFound("label")
		}
		return err
	}
	s.publish(ctx, events.New(events.LabelChanged, space.TenantID).
		WithSpace(space.ID).WithActor(actorID(actor)).
		With("label_id", labelID).With("action", "deleted"))
	return nil
}

// SetPageLabels makes a page's labels exactly the given set.
func (s *PageService) SetPageLabels(ctx context.Context, actor *acl.Identity, d acl.Decision,
	labelIDs []string,
) ([]*LabelView, error) {
	if err := requireRole(d, model.RoleWriter); err != nil {
		return nil, err
	}
	if s.d.Repos.Labels == nil {
		return []*LabelView{}, nil
	}
	if len(labelIDs) > MaxLabelsPerPage {
		return nil, invalid("a page may carry at most %d labels", MaxLabelsPerPage)
	}
	// Only this space's labels: a page in one space must not be filed under
	// another's vocabulary, which would also make that space's label counts
	// mean something the space cannot see.
	if err := s.checkLabelsBelongHere(ctx, d.Page, labelIDs); err != nil {
		return nil, err
	}

	if err := s.d.Repos.Labels.SetForPage(ctx, d.Page.TenantID, d.Page.ID, labelIDs); err != nil {
		return nil, err
	}
	s.publish(ctx, events.New(events.PageMeta, d.Page.TenantID).
		WithSpace(d.Page.SpaceID).WithPage(d.Page.ID).WithActor(actorID(actor)).
		With("labels_changed", true))

	byPage, err := s.d.Repos.Labels.ForPages(ctx, d.Page.TenantID, []string{d.Page.ID})
	if err != nil {
		return nil, err
	}
	return labelViews(byPage[d.Page.ID]), nil
}

// PageLabels returns the labels on one page.
func (s *PageService) PageLabels(ctx context.Context, actor *acl.Identity, d acl.Decision) (
	[]*LabelView, error,
) {
	if s.d.Repos.Labels == nil {
		return []*LabelView{}, nil
	}
	byPage, err := s.d.Repos.Labels.ForPages(ctx, d.Page.TenantID, []string{d.Page.ID})
	if err != nil {
		return nil, err
	}
	return labelViews(byPage[d.Page.ID]), nil
}

// ---- helpers -------------------------------------------------------------------

func (s *PageService) loadLabel(ctx context.Context, space *model.Space, labelID string) (
	*model.Label, error,
) {
	if s.d.Repos.Labels == nil || labelID == "" {
		return nil, notFound("label")
	}
	label, err := s.d.Repos.Labels.Get(ctx, space.TenantID, labelID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, notFound("label")
		}
		return nil, err
	}
	// Naming another space's label must not reach it, the same rule revisions
	// and comments follow.
	if label.SpaceID != space.ID {
		return nil, notFound("label")
	}
	return label, nil
}

func (s *PageService) checkLabelsBelongHere(ctx context.Context, page *model.Page, ids []string) error {
	for _, id := range dedupe(ids) {
		label, err := s.d.Repos.Labels.Get(ctx, page.TenantID, id)
		if err != nil || label.SpaceID != page.SpaceID {
			return invalid("one of the labels does not belong to this space")
		}
	}
	return nil
}

func cleanLabelName(raw string) (string, error) {
	name := strings.Join(strings.Fields(raw), " ")
	if name == "" {
		return "", invalid("a label needs a name")
	}
	if len([]rune(name)) > MaxLabelNameRunes {
		return "", invalid("a label's name may not exceed %d characters", MaxLabelNameRunes)
	}
	return name, nil
}

func cleanLabelColor(raw string) (string, error) {
	if raw == "" {
		return LabelColors[0], nil
	}
	for _, allowed := range LabelColors {
		if raw == allowed {
			return raw, nil
		}
	}
	return "", invalid("%q is not one of this product's label colours", raw)
}

func labelViews(labels []*model.Label) []*LabelView {
	out := make([]*LabelView, 0, len(labels))
	for _, label := range labels {
		out = append(out, &LabelView{
			ID: label.ID, SpaceID: label.SpaceID, Name: label.Name, Color: label.Color,
		})
	}
	return out
}
