package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	apprepo "github.com/magicyuan876/yuheng/internal/application/repository"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/handler"
	"github.com/magicyuan876/yuheng/internal/middleware"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/require"
)

type downloadKnowledgeLookup struct {
	knowledge *types.Knowledge
}

func (s *downloadKnowledgeLookup) GetKnowledgeByIDOnly(_ context.Context, id string) (*types.Knowledge, error) {
	if s.knowledge != nil && s.knowledge.ID == id {
		return s.knowledge, nil
	}
	return nil, apprepo.ErrKnowledgeNotFound
}

// newKnowledgeDownloadRouteTestEngine wires the knowledge routes with the
// given caller role (tenant 1) and a single document in a single base, so
// the download route's guard chain can be exercised end to end.
func newKnowledgeDownloadRouteTestEngine(
	t *testing.T,
	role types.TenantRole,
	knowledge *types.Knowledge,
	kb *types.KnowledgeBase,
) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	enabled := true
	guards := &rbacGuards{
		cfg:              &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}},
		knowledgeService: &downloadKnowledgeLookup{knowledge: knowledge},
		kbService:        &stubWikiKBLookup{kbs: map[string]*types.KnowledgeBase{kb.ID: kb}},
	}

	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Next()
	})
	RegisterKnowledgeRoutes(r.Group("/api/v1"), &handler.KnowledgeHandler{}, guards)
	return r
}

func TestKnowledgeDownloadRejectsTenantViewer(t *testing.T) {
	engine := newKnowledgeDownloadRouteTestEngine(
		t,
		types.TenantRoleViewer,
		&types.Knowledge{ID: "knowledge-own", KnowledgeBaseID: "kb-own", TenantID: 1},
		&types.KnowledgeBase{ID: "kb-own", TenantID: 1},
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/knowledge-own/download", nil)
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code, "body=%s", rec.Body.String())
}

// A document in another workspace's base does not exist from here: even a
// Contributor gets the same 404 a missing document would produce.
func TestKnowledgeDownloadForeignKBIsNotFound(t *testing.T) {
	engine := newKnowledgeDownloadRouteTestEngine(
		t,
		types.TenantRoleContributor,
		&types.Knowledge{ID: "knowledge-foreign", KnowledgeBaseID: "kb-foreign", TenantID: 2},
		&types.KnowledgeBase{ID: "kb-foreign", TenantID: 2},
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/knowledge-foreign/download", nil)
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code, "body=%s", rec.Body.String())
}
