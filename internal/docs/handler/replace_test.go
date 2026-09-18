package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// Acceptance (T1.7): writing a body over REST goes through the one replace
// path, and the page reads back as what was written.
func TestReplaceContentRouteWritesTheWholeBody(t *testing.T) {
	r, pid := leaseFixture(t)

	put := call(t, r, "bob", http.MethodPut, "/docs/pages/"+pid+"/content", map[string]any{
		"content": pmDoc("written over REST"),
	})
	require.Equal(t, http.StatusOK, put.Code, put.Body)
	require.Equal(t, float64(1), data(put)["ydoc_version"])
	require.Equal(t, "direct", data(put)["applied"])

	got := call(t, r, "viewer", http.MethodGet, "/docs/pages/"+pid+"/content", nil)
	require.Equal(t, http.StatusOK, got.Code, got.Body)
	body, _ := data(got)["content"].(map[string]any)
	require.NotNil(t, body)
	require.Equal(t, float64(1), data(got)["ydoc_version"])

	// Markdown is accepted in place of a document.
	md := call(t, r, "bob", http.MethodPut, "/docs/pages/"+pid+"/content", map[string]any{
		"markdown": "## Heading\n\nText.\n",
	})
	require.Equal(t, http.StatusOK, md.Code, md.Body)
	require.Equal(t, float64(2), data(md)["ydoc_version"])
}

func TestReplaceContentRouteEnforcesTheWriterRole(t *testing.T) {
	r, pid := leaseFixture(t)

	require.Equal(t, http.StatusForbidden, call(t, r, "viewer", http.MethodPut,
		"/docs/pages/"+pid+"/content", map[string]any{"content": pmDoc("no")}).Code)
	require.Equal(t, http.StatusNotFound, call(t, r, "carol", http.MethodPut,
		"/docs/pages/"+pid+"/content", map[string]any{"content": pmDoc("no")}).Code)
}

func TestReplaceContentRouteRejectsABodyTheSchemaDoesNotAllow(t *testing.T) {
	r, pid := leaseFixture(t)
	res := call(t, r, "bob", http.MethodPut, "/docs/pages/"+pid+"/content", map[string]any{
		"content": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "nope"}}},
	})
	require.Equal(t, http.StatusBadRequest, res.Code, res.Body)
}
