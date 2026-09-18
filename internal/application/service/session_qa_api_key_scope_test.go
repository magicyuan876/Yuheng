package service

import (
	"context"
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
)

func TestResolveKnowledgeBasesAllowsInScopeRequestKBsForRestrictedAPIKey(t *testing.T) {
	ctx := types.WithTenantAPIKeyScope(context.Background(), types.TenantAPIKeyScope{
		KnowledgeBaseIDs: types.StringArray{"kb-allowed"},
	})
	svc := &sessionService{}

	kbIDs, knowledgeIDs, err := svc.resolveKnowledgeBases(ctx, &types.QARequest{
		Session:          &types.Session{TenantID: 10000},
		KnowledgeBaseIDs: []string{"kb-allowed"},
	})
	if err != nil {
		t.Fatalf("resolveKnowledgeBases returned error: %v", err)
	}
	if len(kbIDs) != 1 || kbIDs[0] != "kb-allowed" {
		t.Fatalf("kbIDs = %#v, want only kb-allowed", kbIDs)
	}
	if len(knowledgeIDs) != 0 {
		t.Fatalf("knowledgeIDs = %#v, want empty", knowledgeIDs)
	}
}

func TestResolveKnowledgeBasesRejectsExplicitOutOfScopeKBForRestrictedAPIKey(t *testing.T) {
	ctx := types.WithTenantAPIKeyScope(context.Background(), types.TenantAPIKeyScope{
		KnowledgeBaseIDs: types.StringArray{"kb-allowed"},
	})
	svc := &sessionService{}

	// Explicit @mention of a KB outside the key scope must be rejected, not
	// silently filtered — the same holds for a mixed list (kb-blocked is
	// out of scope even though kb-allowed is fine).
	for _, requested := range [][]string{{"kb-blocked"}, {"kb-allowed", "kb-blocked"}} {
		_, _, err := svc.resolveKnowledgeBases(ctx, &types.QARequest{
			Session:          &types.Session{TenantID: 10000},
			KnowledgeBaseIDs: requested,
		})
		if err == nil {
			t.Fatalf("expected forbidden for explicit out-of-scope knowledge_base_ids %v", requested)
		}
	}
}
