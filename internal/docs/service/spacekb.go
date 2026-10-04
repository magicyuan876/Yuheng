package service

import (
	"context"
	"strings"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
)

// A space and the knowledge base its pages are mirrored into.
//
// ---- what a space can say about it
//
// Nothing (the pages stay out of every knowledge base), an existing knowledge
// base, or a new one made for the space and named like it. The third is what
// the form offers by default: the point of writing in Yuheng rather than in a
// wiki next to it is that what is written can be asked about, and a team that
// first has to go and make a knowledge base, then come back and bind it, mostly
// does not.
//
// ---- who may bind which knowledge base
//
// Binding is a space administrator's act (the route guards on it). The
// knowledge base is the second half of the question: binding writes every
// eligible page of the space into it, so it is the same act as uploading
// documents to that knowledge base, and is allowed to whoever may do that —
// its creator, or a workspace Owner or Admin. Letting anyone who can merely
// see a knowledge base fill it with their space's pages would bypass the
// ownership rule the knowledge base's own routes enforce. An API key binds
// only knowledge bases inside its allow-list, as everywhere else.
//
// Only document knowledge bases can hold pages. An FAQ knowledge base indexes
// question-and-answer entries, not documents, and refuses document uploads
// for the same reason; a temporary one is an internal artefact nobody sees.
//
// ---- who may have one made
//
// Whoever may create the space: workspace Contributor and up, the same floor
// as creating a knowledge base directly, with the caller recorded as its
// creator so it is theirs to configure afterwards. An API key also needs the
// capability that creates knowledge bases (manage_kbs, or full access), and a
// key limited to some knowledge bases cannot have one made at all: the new one
// would be outside its own allow-list, and so would the space bound to it.
//
// ---- no orphan knowledge base
//
// The knowledge base lives in another service's tables, so the two writes
// cannot share a transaction. Everything that can be checked is checked before
// the knowledge base is made; if the space still cannot be written after that,
// the knowledge base is deleted again. It is empty at that point, so deleting
// it loses nothing.
//
// ---- what happens to pages already mirrored
//
// Changing the binding makes every page of the space due for synchronisation
// at once. The synchronisation already does the right thing with an entry in
// the wrong place — unbound: the entry is deleted; rebound: it is deleted from
// the old knowledge base and made anew in the new one — but until this change
// nothing asked it to run, and a rebinding waited for the periodic sweep, which
// looks at a few hundred pages every five minutes. A large space stayed
// answerable from the knowledge base it had just been taken out of for hours.

// KnowledgeBaseMode says what a space does about a knowledge base.
type KnowledgeBaseMode string

const (
	// KnowledgeBaseNone keeps the space's pages out of every knowledge base.
	KnowledgeBaseNone KnowledgeBaseMode = "none"
	// KnowledgeBaseExisting binds a knowledge base that already exists.
	KnowledgeBaseExisting KnowledgeBaseMode = "existing"
	// KnowledgeBaseCreate makes a new knowledge base, named like the space,
	// and binds it.
	KnowledgeBaseCreate KnowledgeBaseMode = "create"
)

// KnowledgeBaseChoice is what a space syncs its pages into.
type KnowledgeBaseChoice struct {
	Mode KnowledgeBaseMode
	// ID names the knowledge base for KnowledgeBaseExisting, and must be
	// empty for the other modes.
	ID string
}

// KnowledgeBaseMaker makes the knowledge base a space asked for, and deletes
// it again when the space it was made for could not be written. The adapter
// over Yuheng's knowledge-base and model services satisfies it.
type KnowledgeBaseMaker interface {
	// CreateForSpace makes a document knowledge base in the tenant, on the
	// given storage backend ("" for the workspace default), with the
	// workspace's default models. It fails with
	// apperrors.ErrEmbeddingModelRequired when the workspace has no
	// embedding model, rather than make a knowledge base that could never
	// answer.
	CreateForSpace(ctx context.Context, tenantID uint64, name, description, storageBackendID string) (
		*types.KnowledgeBase, error)
	// DeleteKnowledgeBase removes a knowledge base made by CreateForSpace.
	DeleteKnowledgeBase(ctx context.Context, tenantID uint64, id string) error
}

// normaliseKnowledgeBaseChoice validates a choice; nil means none.
func normaliseKnowledgeBaseChoice(c *KnowledgeBaseChoice) (KnowledgeBaseChoice, error) {
	if c == nil {
		return KnowledgeBaseChoice{Mode: KnowledgeBaseNone}, nil
	}
	out := KnowledgeBaseChoice{
		Mode: KnowledgeBaseMode(strings.ToLower(strings.TrimSpace(string(c.Mode)))),
		ID:   strings.TrimSpace(c.ID),
	}
	switch out.Mode {
	case KnowledgeBaseNone, KnowledgeBaseCreate:
		if out.ID != "" {
			return out, invalid("knowledge_base.id is accepted only with mode %q", KnowledgeBaseExisting)
		}
	case KnowledgeBaseExisting:
		if out.ID == "" {
			return out, invalid("knowledge_base.id is required with mode %q", KnowledgeBaseExisting)
		}
	default:
		return out, invalid("knowledge_base.mode must be none, existing or create")
	}
	return out, nil
}

// bindableKnowledgeBase checks that the actor may bind an existing knowledge
// base to a space, and returns its id.
func (s *SpaceService) bindableKnowledgeBase(ctx context.Context, actor *acl.Identity, id string) (string, error) {
	if s.d.KnowledgeBases == nil {
		return "", invalid("knowledge base binding is not available in this deployment")
	}
	kb, err := s.d.KnowledgeBases.GetKnowledgeBaseByIDAndTenant(ctx, id, actor.TenantID)
	if err != nil || kb == nil {
		return "", invalid("knowledge base %q was not found in this workspace", id)
	}
	if kb.IsTemporary || (kb.Type != "" && kb.Type != types.KnowledgeBaseTypeDocument) {
		return "", invalid("knowledge base %q is not a document knowledge base; only those can hold pages", id)
	}
	if !mayFillKnowledgeBase(actor, kb) {
		return "", forbidden("binding adds the space's pages to the knowledge base, which only its creator " +
			"or a workspace administrator may do")
	}
	return kb.ID, nil
}

// mayFillKnowledgeBase is the knowledge base's own rule for adding documents
// to it (OwnedKBOrAdmin on its upload routes), applied to the binding that
// will add every page of a space.
func mayFillKnowledgeBase(actor *acl.Identity, kb *types.KnowledgeBase) bool {
	if actor.Machine {
		return actor.KnowledgeBases == nil || actor.KnowledgeBases[kb.ID]
	}
	if actor.IsTenantAdmin() {
		return true
	}
	return kb.CreatorID != "" && kb.CreatorID == actor.UserID
}

// mayCreateKnowledgeBase checks, before anything is made, that a knowledge
// base may be created for the actor.
func (s *SpaceService) mayCreateKnowledgeBase(ctx context.Context, actor *acl.Identity) error {
	if s.d.KnowledgeBaseMaker == nil {
		return invalid("creating a knowledge base is not available in this deployment")
	}
	if !actor.Machine {
		// The route admits workspace Contributors and up, which is also who
		// may create a knowledge base; this repeats it for callers that are
		// not routes.
		if !actor.Member || !actor.TenantRole.HasPermission(types.TenantRoleContributor) {
			return forbidden("creating a knowledge base needs the Contributor role in the workspace")
		}
		return nil
	}
	if actor.KnowledgeBases != nil {
		return forbidden("an API key limited to some knowledge bases cannot have one created")
	}
	scope, ok := types.TenantAPIKeyScopeFromContext(ctx)
	if !ok || (!scope.FullAccess && !scope.HasCapability(types.APIKeyCapabilityManageKnowledgeBases)) {
		return forbidden("creating a knowledge base needs the manage_kbs capability")
	}
	return nil
}

// discardKnowledgeBase deletes a knowledge base made for a space that was
// then not written. Detached from the request, because a client that gave up
// is the commonest reason the space was not written and the knowledge base
// must go all the same; a failure is logged, since there is nobody left to
// tell and the knowledge base is empty and visible in the list.
func (s *SpaceService) discardKnowledgeBase(ctx context.Context, tenantID uint64, id string) {
	if err := s.d.KnowledgeBaseMaker.DeleteKnowledgeBase(context.WithoutCancel(ctx), tenantID, id); err != nil {
		logger.Errorf(ctx, "[docs] knowledge base %s was made for a space that was not written, "+
			"and deleting it failed: %v", id, err)
	}
}

// requeueSpace makes every page of a space due for synchronisation now, after
// something that changes the answer for all of them: its knowledge base
// changed (see the top of this file), or the space was trashed or restored.
// Best effort: the change is committed, and if this fails the periodic sweep
// still finds the pages, because the space's updated_at is newer than their
// last look.
func (s *SpaceService) requeueSpace(ctx context.Context, tenantID uint64, spaceID string) {
	if s.d.Knowledge == nil {
		return
	}
	if err := s.d.Repos.IndexQueue.MarkSpace(ctx, tenantID, spaceID, 0); err != nil {
		logger.Warnf(ctx, "[docs] queuing the pages of space %s after its knowledge base changed failed: %v",
			spaceID, err)
	}
}

func sameKnowledgeBase(a, b *string) bool {
	if a == nil || *a == "" {
		return b == nil || *b == ""
	}
	return b != nil && *a == *b
}

// BindKnowledgeBase changes the knowledge base and storage backend a space
// uses. A nil choice or storage id leaves that binding alone; an empty storage
// id rebinds the space to the workspace default. The route has checked that
// the actor administers the space.
//
// With KnowledgeBaseCreate the new knowledge base is made on the storage
// backend the space ends up on, so a space and its knowledge base keep their
// files in one place unless somebody separates them on purpose.
func (s *SpaceService) BindKnowledgeBase(ctx context.Context, actor *acl.Identity, space *model.Space,
	choice *KnowledgeBaseChoice, storageBackendID *string,
) (view *SpaceView, err error) {
	// made is a knowledge base created by this call; it is deleted again if
	// the call fails after making it.
	made := ""
	defer func() {
		if err != nil && made != "" {
			s.discardKnowledgeBase(ctx, actor.TenantID, made)
		}
	}()

	fields := map[string]any{}
	storageID := space.StorageBackendID
	if storageBackendID != nil {
		// A space is never unbound: an empty id rebinds it to the workspace
		// default. Existing files stay where they are and keep resolving.
		storageID, err = s.checkStorageBackend(ctx, actor.TenantID, *storageBackendID)
		if err != nil {
			return nil, err
		}
		fields["storage_backend_id"] = storageID
	}

	var kbAction *audit.Entry
	kbChanged := false
	if choice != nil {
		c, cerr := normaliseKnowledgeBaseChoice(choice)
		if cerr != nil {
			return nil, cerr
		}
		var kbID *string
		switch c.Mode {
		case KnowledgeBaseExisting:
			id, err := s.bindableKnowledgeBase(ctx, actor, c.ID)
			if err != nil {
				return nil, err
			}
			kbID = &id
		case KnowledgeBaseCreate:
			if err := s.mayCreateKnowledgeBase(ctx, actor); err != nil {
				return nil, err
			}
			kb, err := s.d.KnowledgeBaseMaker.CreateForSpace(ctx, actor.TenantID, space.Name, space.Description,
				storageID)
			if err != nil {
				return nil, err
			}
			made = kb.ID
			kbID = &made
		}

		entry := audit.Entry{
			TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
			Action: audit.SpaceKBBound, SpaceID: space.ID, TargetType: "knowledge_base",
		}
		if kbID == nil {
			entry.Action = audit.SpaceKBUnbound
			if space.KnowledgeBaseID != nil {
				entry.TargetID = *space.KnowledgeBaseID
			}
		} else {
			entry.TargetID = *kbID
		}
		kbAction = &entry
		kbChanged = !sameKnowledgeBase(space.KnowledgeBaseID, kbID)
		fields["knowledge_base_id"] = kbID
		space.KnowledgeBaseID = kbID
	}
	if storageBackendID != nil {
		space.StorageBackendID = storageID
	}
	if len(fields) == 0 {
		return s.view(ctx, space, model.RoleAdmin)
	}
	if err := s.d.Repos.Spaces.Update(ctx, actor.TenantID, space.ID, fields); err != nil {
		return nil, err
	}
	if kbChanged {
		s.requeueSpace(ctx, actor.TenantID, space.ID)
	}
	s.publish(ctx, events.New(events.SpaceUpdated, actor.TenantID).WithSpace(space.ID).WithActor(actor.UserID).
		With("changed", []string{"bindings"}))
	if kbAction != nil {
		s.audit(ctx, *kbAction)
	} else {
		s.audit(ctx, audit.Entry{
			TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorRole: actorRole(actor),
			Action: audit.SpaceUpdated, SpaceID: space.ID, TargetType: audit.TargetSpace, TargetID: space.ID,
		})
	}
	fresh, err := s.d.Repos.Spaces.Get(ctx, actor.TenantID, space.ID)
	if err != nil {
		return nil, err
	}
	return s.view(ctx, fresh, model.RoleAdmin)
}
