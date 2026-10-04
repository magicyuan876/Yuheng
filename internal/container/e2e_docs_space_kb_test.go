package container

import (
	"context"
	"net/http"
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// TestASpaceGetsItsKnowledgeBaseThroughTheRealServices creates a docs space
// with {"knowledge_base":{"mode":"create"}} through the router and checks the
// knowledge base the real knowledge-base service made: that it is refused
// without an embedding model and leaves nothing behind, that it is named like
// the space, owned by the person who created the space, on the space's storage
// backend, with the workspace's embedding model — and that binding follows the
// knowledge base's ownership rule.
func TestASpaceGetsItsKnowledgeBaseThroughTheRealServices(t *testing.T) {
	s := bootServer(t, bootOptions{})
	owner, tenantID := createUserWithWorkspace(t, s, "kbowner", "kbowner@example.com", "owner password")

	// A Contributor of the same workspace: may create spaces and knowledge
	// bases, may not fill somebody else's knowledge base.
	var writer *types.User
	must1(t, s.DI.Invoke(func(users interfaces.UserService, members interfaces.TenantMemberService) {
		u, err := users.Register(context.Background(), &types.RegisterRequest{
			Username: "kbwriter", Email: "kbwriter@example.com", Password: "writer password",
		})
		must1(t, err)
		_, err = members.AddMember(context.Background(), u.ID, tenantID, types.TenantRoleContributor, nil)
		must1(t, err)
		writer = u
	}))

	login := func(email, password, ip string) string {
		t.Helper()
		got := s.do(request{
			method: http.MethodPost, path: "/api/v1/auth/login",
			body: map[string]string{"email": email, "password": password}, ip: ip,
		})
		if got.Code != http.StatusOK {
			t.Fatalf("login %s = %d: %s", email, got.Code, got.Body)
		}
		tok, _ := got.JSON(t)["token"].(string)
		return tok
	}
	ownerToken := login("kbowner@example.com", "owner password", "203.0.113.41")
	writerToken := login("kbwriter@example.com", "writer password", "203.0.113.42")

	knowledgeBases := func() []types.KnowledgeBase {
		t.Helper()
		var rows []types.KnowledgeBase
		must1(t, s.DB.Where("tenant_id = ?", tenantID).Find(&rows).Error)
		return rows
	}
	createSpace := func(token, name string) response {
		t.Helper()
		return s.do(request{
			method: http.MethodPost, path: "/api/v1/docs/spaces", bearer: token,
			body: map[string]any{"name": name, "knowledge_base": map[string]string{"mode": "create"}},
		})
	}
	errorCode := func(r response) float64 {
		t.Helper()
		errObj, _ := r.JSON(t)["error"].(map[string]any)
		code, _ := errObj["code"].(float64)
		return code
	}

	t.Run("without an embedding model nothing is made", func(t *testing.T) {
		got := createSpace(writerToken, "Handbook")
		if got.Code != http.StatusBadRequest || errorCode(got) != 2300 {
			t.Fatalf("create = %d %s, want 400 with code 2300", got.Code, got.Body)
		}
		if kbs := knowledgeBases(); len(kbs) != 0 {
			t.Errorf("%d knowledge base(s) left behind", len(kbs))
		}
		var spaces int64
		must1(t, s.DB.Table("docs_spaces").Where("tenant_id = ?", tenantID).Count(&spaces).Error)
		if spaces != 0 {
			t.Errorf("%d space(s) made", spaces)
		}
	})

	must1(t, s.DB.Create(&types.Model{
		ID: "model-embed-e2e", TenantID: tenantID, Name: "embedder", Type: types.ModelTypeEmbedding,
		Source: types.ModelSourceRemote, Status: types.ModelStatusActive,
	}).Error)

	var spaceID, madeKB string
	t.Run("the space's knowledge base is an ordinary one, made for its creator", func(t *testing.T) {
		got := createSpace(writerToken, "Handbook")
		if got.Code != http.StatusCreated {
			t.Fatalf("create = %d: %s", got.Code, got.Body)
		}
		data, _ := got.JSON(t)["data"].(map[string]any)
		spaceID, _ = data["id"].(string)
		madeKB, _ = data["knowledge_base_id"].(string)
		storage, _ := data["storage_backend_id"].(string)
		if madeKB == "" {
			t.Fatalf("the space is not bound: %s", got.Body)
		}
		kbs := knowledgeBases()
		if len(kbs) != 1 || kbs[0].ID != madeKB {
			t.Fatalf("knowledge bases %+v, want exactly the bound %s", kbs, madeKB)
		}
		kb := kbs[0]
		if kb.Name != "Handbook" || kb.Type != types.KnowledgeBaseTypeDocument {
			t.Errorf("name/type = %q/%q, want Handbook/document", kb.Name, kb.Type)
		}
		if kb.CreatorID != writer.ID {
			t.Errorf("creator = %q, want the space's creator %q", kb.CreatorID, writer.ID)
		}
		if kb.EmbeddingModelID != "model-embed-e2e" {
			t.Errorf("embedding model = %q", kb.EmbeddingModelID)
		}
		if kb.StorageBackendID == "" || kb.StorageBackendID != storage {
			t.Errorf("storage backend = %q, want the space's %q", kb.StorageBackendID, storage)
		}
	})

	put := func(token string, body any) response {
		t.Helper()
		return s.do(request{
			method: http.MethodPut, path: "/api/v1/docs/spaces/" + spaceID + "/knowledge-base", bearer: token,
			body: body,
		})
	}

	t.Run("a knowledge base is bound only by who may fill it", func(t *testing.T) {
		ownersKB := &types.KnowledgeBase{
			ID: "kb-owner-e2e", Name: "Owner's", TenantID: tenantID, Type: types.KnowledgeBaseTypeDocument,
			CreatorID: owner.ID, StorageBackendID: knowledgeBases()[0].StorageBackendID,
		}
		must1(t, s.DB.Create(ownersKB).Error)
		existing := map[string]any{"knowledge_base": map[string]string{"mode": "existing", "id": ownersKB.ID}}
		if got := put(writerToken, existing); got.Code != http.StatusForbidden {
			t.Errorf("a Contributor binding the owner's knowledge base = %d: %s", got.Code, got.Body)
		}
		if got := put(ownerToken, existing); got.Code != http.StatusOK {
			t.Errorf("the workspace owner binding it = %d: %s", got.Code, got.Body)
		}
	})

	t.Run("the replaced field is refused, not ignored", func(t *testing.T) {
		got := put(ownerToken, map[string]any{"knowledge_base_id": madeKB})
		if got.Code != http.StatusBadRequest {
			t.Errorf("knowledge_base_id = %d: %s", got.Code, got.Body)
		}
	})

	t.Run("unbinding leaves the space without a knowledge base", func(t *testing.T) {
		got := put(ownerToken, map[string]any{"knowledge_base": map[string]string{"mode": "none"}})
		if got.Code != http.StatusOK {
			t.Fatalf("unbind = %d: %s", got.Code, got.Body)
		}
		data, _ := got.JSON(t)["data"].(map[string]any)
		if data["knowledge_base_id"] != nil {
			t.Errorf("still bound: %v", data["knowledge_base_id"])
		}
	})
}
