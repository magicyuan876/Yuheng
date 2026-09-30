package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/handler"
	"github.com/magicyuan876/yuheng/internal/middleware"
	"github.com/magicyuan876/yuheng/internal/types"
)

// stubFindingService answers the finding API with fixed data and remembers
// the tenant each call was made for.
type stubFindingService struct {
	tenants []uint64
	scans   int
	status  string
}

func (s *stubFindingService) List(_ context.Context, tenantID uint64, kbID string,
	filter types.KnowledgeFindingFilter,
) (*types.KnowledgeFindingPage, error) {
	s.tenants = append(s.tenants, tenantID)
	related := &types.KnowledgeRef{KnowledgeID: "doc-b", Title: "B"}
	return &types.KnowledgeFindingPage{Items: []*types.KnowledgeFindingView{{
		ID: "f1", KnowledgeBaseID: kbID, Type: types.FindingTypeDuplicate, Detector: "duplicate",
		Severity: types.FindingSeverityWarning, Status: types.FindingStatusOpen, Score: 0.97, OverlapRatio: 0.8,
		Subject: types.KnowledgeRef{KnowledgeID: "doc-a", Title: "A"}, Related: related,
		Evidence: []types.FindingEvidence{}, CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
	}}, Total: 1, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func (s *stubFindingService) Summary(_ context.Context, tenantID uint64, _ string,
) (*types.KnowledgeFindingSummary, error) {
	s.tenants = append(s.tenants, tenantID)
	return &types.KnowledgeFindingSummary{
		OpenTotal: 1, OpenByType: map[string]int64{"duplicate": 1}, Enabled: true,
	}, nil
}

func (s *stubFindingService) UpdateStatus(_ context.Context, tenantID uint64, kbID, id, status string,
) (*types.KnowledgeFindingView, error) {
	s.tenants = append(s.tenants, tenantID)
	s.status = status
	return &types.KnowledgeFindingView{
		ID: id, KnowledgeBaseID: kbID, Status: status, Evidence: []types.FindingEvidence{},
	}, nil
}

func (s *stubFindingService) Scan(_ context.Context, tenantID uint64, _ string) (int, error) {
	s.tenants = append(s.tenants, tenantID)
	s.scans++
	return 3, nil
}

func (s *stubFindingService) OpenForKnowledge(context.Context, uint64, string) ([]*types.KnowledgeFindingView, error) {
	return nil, nil
}

type findingCaller struct {
	role   types.TenantRole
	userID string
	scope  *types.TenantAPIKeyScope
}

// newFindingRouteEngine mounts the finding routes behind the real guards and
// the real API-key gate. kb-own belongs to tenant 1 and was created by
// "creator"; kb-foreign belongs to tenant 999.
func newFindingRouteEngine(t *testing.T, svc *stubFindingService, caller findingCaller) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	enabled := true
	kbs := &stubWikiKBLookup{kbs: map[string]*types.KnowledgeBase{
		"kb-own":     {ID: "kb-own", TenantID: 1, CreatorID: "creator", Type: types.KnowledgeBaseTypeDocument},
		"kb-foreign": {ID: "kb-foreign", TenantID: 999, Type: types.KnowledgeBaseTypeDocument},
	}}
	guards := &rbacGuards{
		cfg:       &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}},
		kbService: kbs,
		kbCreator: func(c *gin.Context) (string, error) {
			kb, err := kbs.GetKnowledgeBaseByID(c.Request.Context(), c.Param("id"))
			if err != nil {
				return "", middleware.ErrResourceNotFound
			}
			return kb.CreatorID, nil
		},
	}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, caller.role)
		ctx = context.WithValue(ctx, types.UserIDContextKey, caller.userID)
		if caller.scope != nil {
			ctx = types.WithTenantAPIKeyScope(ctx, *caller.scope)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Next()
	})
	v1 := r.Group("/api/v1")
	v1.Use(guards.ensureAPIKeyAuthorizer().Middleware())
	RegisterKnowledgeFindingRoutes(v1, handler.NewKnowledgeFindingHandler(svc), guards)
	guards.assertAPIKeyPoliciesMatchRoutes(r)
	return r
}

type findingRequest struct{ method, path, body string }

var (
	listFindings   = findingRequest{http.MethodGet, "/api/v1/knowledge-bases/kb-own/findings?page=1&page_size=5", ""}
	findingSummary = findingRequest{http.MethodGet, "/api/v1/knowledge-bases/kb-own/findings/summary", ""}
	dismissFinding = findingRequest{
		http.MethodPatch, "/api/v1/knowledge-bases/kb-own/findings/f1", `{"status":"dismissed"}`,
	}
	scanFindings = findingRequest{http.MethodPost, "/api/v1/knowledge-bases/kb-own/findings/scan", ""}
)

func serveFinding(t *testing.T, r *gin.Engine, req findingRequest) (int, map[string]any) {
	t.Helper()
	httpReq := httptest.NewRequest(req.method, req.path, strings.NewReader(req.body))
	httpReq.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httpReq)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

// The response shapes the frontend is written against.
func TestFindingRoutesAnswerInTheEnvelope(t *testing.T) {
	svc := &stubFindingService{}
	r := newFindingRouteEngine(t, svc, findingCaller{role: types.TenantRoleAdmin, userID: "admin"})

	code, body := serveFinding(t, r, listFindings)
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, true, body["success"])
	page := body["data"].(map[string]any)
	assert.EqualValues(t, 1, page["total"])
	assert.EqualValues(t, 1, page["page"])
	assert.EqualValues(t, 5, page["page_size"])
	item := page["items"].([]any)[0].(map[string]any)
	for _, key := range []string{
		"id", "knowledge_base_id", "type", "detector", "severity", "status", "score", "overlap_ratio",
		"subject", "related", "evidence", "created_at", "updated_at", "resolved_at", "resolved_by",
	} {
		assert.Contains(t, item, key)
	}
	assert.Nil(t, item["resolved_at"], "an open finding has resolved_at: null")
	assert.Equal(t, map[string]any{"knowledge_id": "doc-a", "title": "A"}, item["subject"])

	code, body = serveFinding(t, r, findingSummary)
	require.Equal(t, http.StatusOK, code, body)
	summary := body["data"].(map[string]any)
	for _, key := range []string{"open_total", "open_by_type", "last_scan_at", "enabled", "supported"} {
		assert.Contains(t, summary, key)
	}

	code, body = serveFinding(t, r, dismissFinding)
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, "dismissed", svc.status)
	assert.Equal(t, "dismissed", body["data"].(map[string]any)["status"])

	code, body = serveFinding(t, r, scanFindings)
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, map[string]any{"queued": float64(3)}, body["data"])

	code, _ = serveFinding(t, r, findingRequest{
		http.MethodGet, "/api/v1/knowledge-bases/kb-own/findings?page_size=101", "",
	})
	assert.Equal(t, http.StatusBadRequest, code, "page_size is capped at 100")
	code, _ = serveFinding(t, r, findingRequest{
		http.MethodPatch, "/api/v1/knowledge-bases/kb-own/findings/f1", `{}`,
	})
	assert.Equal(t, http.StatusBadRequest, code, "status is required")

	for _, tenant := range svc.tenants {
		assert.Equal(t, uint64(1), tenant, "calls are made in the base owner's tenant")
	}
}

// Roles: a viewer reads; changing a finding or re-checking is the base
// creator's or an admin's; nobody reaches another tenant's base.
func TestFindingRoutesFollowTheKBRoleMatrix(t *testing.T) {
	cases := []struct {
		name   string
		caller findingCaller
		req    findingRequest
		want   int
	}{
		{"viewer reads the list", findingCaller{role: types.TenantRoleViewer, userID: "v"}, listFindings, 200},
		{"viewer reads the summary", findingCaller{role: types.TenantRoleViewer, userID: "v"}, findingSummary, 200},
		{"viewer cannot dismiss", findingCaller{role: types.TenantRoleViewer, userID: "v"}, dismissFinding, 403},
		{"viewer cannot re-check", findingCaller{role: types.TenantRoleViewer, userID: "v"}, scanFindings, 403},
		{
			"another contributor cannot dismiss",
			findingCaller{role: types.TenantRoleContributor, userID: "someone"},
			dismissFinding, 403,
		},
		{
			"the base's creator dismisses",
			findingCaller{role: types.TenantRoleContributor, userID: "creator"},
			dismissFinding, 200,
		},
		{
			"the base's creator re-checks",
			findingCaller{role: types.TenantRoleContributor, userID: "creator"},
			scanFindings, 200,
		},
		{"an admin dismisses", findingCaller{role: types.TenantRoleAdmin, userID: "a"}, dismissFinding, 200},
		{
			"no cross-tenant read",
			findingCaller{role: types.TenantRoleOwner, userID: "o"},
			findingRequest{http.MethodGet, "/api/v1/knowledge-bases/kb-foreign/findings", ""},
			403,
		},
		{
			"no cross-tenant dismiss",
			findingCaller{role: types.TenantRoleOwner, userID: "o"},
			findingRequest{
				http.MethodPatch, "/api/v1/knowledge-bases/kb-foreign/findings/f1", `{"status":"dismissed"}`,
			},
			403,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newFindingRouteEngine(t, &stubFindingService{}, tc.caller)
			code, body := serveFinding(t, r, tc.req)
			assert.Equal(t, tc.want, code, body)
		})
	}
}

// API keys: reads need retrieve, a status change needs ingest (the content
// edit capability), a full re-check needs manage_kbs; a key limited to other
// bases reaches none of this one.
func TestFindingRoutesAPIKeyPolicy(t *testing.T) {
	key := func(caps ...types.APIKeyCapability) findingCaller {
		scope := &types.TenantAPIKeyScope{}
		for _, c := range caps {
			scope.Capabilities = append(scope.Capabilities, string(c))
		}
		return findingCaller{role: types.TenantRoleViewer, userID: "api-key", scope: scope}
	}
	retrieve := key(types.APIKeyCapabilityRetrieve)
	ingest := key(types.APIKeyCapabilityIngest)
	manage := key(types.APIKeyCapabilityManageKnowledgeBases)
	full := findingCaller{
		role: types.TenantRoleOwner, userID: "api-key", scope: &types.TenantAPIKeyScope{FullAccess: true},
	}
	elsewhere := key(types.APIKeyCapabilityRetrieve, types.APIKeyCapabilityIngest,
		types.APIKeyCapabilityManageKnowledgeBases)
	elsewhere.scope.KnowledgeBaseIDs = types.StringArray{"kb-other"}

	cases := []struct {
		name   string
		caller findingCaller
		req    findingRequest
		want   int
	}{
		{"retrieve reads", retrieve, listFindings, 200},
		{"retrieve reads the summary", retrieve, findingSummary, 200},
		{"retrieve cannot dismiss", retrieve, dismissFinding, 403},
		{"ingest dismisses", ingest, dismissFinding, 200},
		{"ingest cannot read", ingest, listFindings, 403},
		{"ingest cannot re-check", ingest, scanFindings, 403},
		{"manage_kbs re-checks", manage, scanFindings, 200},
		{"manage_kbs cannot dismiss", manage, dismissFinding, 403},
		{"full access does everything", full, scanFindings, 200},
		{"a key for other bases reads nothing here", elsewhere, listFindings, 403},
		{"a key for other bases changes nothing here", elsewhere, dismissFinding, 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newFindingRouteEngine(t, &stubFindingService{}, tc.caller)
			code, body := serveFinding(t, r, tc.req)
			assert.Equal(t, tc.want, code, body)
		})
	}

	g := &rbacGuards{}
	RegisterKnowledgeFindingRoutes(gin.New().Group("/api/v1"), &handler.KnowledgeFindingHandler{}, g)
	read := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/knowledge-bases/:id/findings")
	assert.True(t, policyHasCapability(read, types.APIKeyCapabilityRetrieve))
	patch := mustLookupAPIKeyPolicy(t, g, http.MethodPatch, "/api/v1/knowledge-bases/:id/findings/:finding_id")
	assert.True(t, policyHasCapability(patch, types.APIKeyCapabilityIngest))
	assert.False(t, policyHasCapability(patch, types.APIKeyCapabilityRetrieve))
	scan := mustLookupAPIKeyPolicy(t, g, http.MethodPost, "/api/v1/knowledge-bases/:id/findings/scan")
	assert.True(t, policyHasCapability(scan, types.APIKeyCapabilityManageKnowledgeBases))
	assert.False(t, policyHasCapability(scan, types.APIKeyCapabilityIngest))
}

// An organisation member with editor access to a shared base may dismiss its
// findings but not start a re-check of the owner's whole base.
func TestFindingScanIsTheOwningWorkspaces(t *testing.T) {
	svc := &stubFindingService{}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		// What the KB-access guard leaves behind for a sharee: the request
		// context carries the owner's tenant, the gin keys the caller's.
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(999))
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.TenantIDContextKey.String(), uint64(1))
	})
	h := handler.NewKnowledgeFindingHandler(svc)
	r.POST("/scan/:id", h.ScanFindings)
	r.PATCH("/findings/:id/:finding_id", h.UpdateFinding)

	code, _ := serveFinding(t, r, findingRequest{http.MethodPost, "/scan/kb-shared", ""})
	assert.Equal(t, http.StatusForbidden, code)
	assert.Zero(t, svc.scans)

	code, _ = serveFinding(t, r, findingRequest{http.MethodPatch, "/findings/kb-shared/f1", `{"status":"dismissed"}`})
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, []uint64{999}, svc.tenants, "the change is made in the owner's tenant")
}
