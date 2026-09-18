package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// linkFixture builds a space with a writer and a reader, plus a target page
// and a source page that links to it.
func linkFixture(t *testing.T) (http.Handler, string, string) {
	t.Helper()
	r, target := leaseFixture(t)

	source := call(t, r, "alice", http.MethodPost, "/docs/pages", map[string]any{
		"space_id": spaceOf(t, r, target), "title": "Source",
	})
	require.Equal(t, http.StatusCreated, source.Code, source.Body)
	sid := data(source)["id"].(string)

	put := call(t, r, "alice", http.MethodPut, "/docs/pages/"+sid+"/content", map[string]any{
		"content": map[string]any{"type": "doc", "content": []any{
			map[string]any{"type": "paragraph", "content": []any{
				map[string]any{"type": "pageLink", "attrs": map[string]any{"pageId": target}},
			}},
		}},
	})
	require.Equal(t, http.StatusOK, put.Code, put.Body)
	return r, target, sid
}

func spaceOf(t *testing.T, r http.Handler, pageID string) string {
	t.Helper()
	res := call(t, r, "alice", http.MethodGet, "/docs/pages/"+pageID, nil)
	require.Equal(t, http.StatusOK, res.Code, res.Body)
	return data(res)["space_id"].(string)
}

func TestBacklinkRouteListsWhatPointsHere(t *testing.T) {
	r, target, source := linkFixture(t)

	res := call(t, r, "viewer", http.MethodGet, "/docs/pages/"+target+"/backlinks", nil)
	require.Equal(t, http.StatusOK, res.Code, res.Body)
	rows, _ := res.Body["data"].([]any)
	require.Len(t, rows, 1)
	require.Equal(t, source, rows[0].(map[string]any)["page_id"])

	// Somebody outside the space cannot see the page, let alone its backlinks.
	require.Equal(t, http.StatusNotFound,
		call(t, r, "carol", http.MethodGet, "/docs/pages/"+target+"/backlinks", nil).Code)
}

func TestTitleRouteResolvesWhatTheCallerMaySee(t *testing.T) {
	r, target, _ := linkFixture(t)

	mine := call(t, r, "viewer", http.MethodPost, "/docs/page-links/titles",
		map[string]any{"page_ids": []string{target, "no-such-page"}})
	require.Equal(t, http.StatusOK, mine.Code, mine.Body)
	rows, _ := mine.Body["data"].([]any)
	require.Len(t, rows, 2)
	require.Equal(t, true, rows[0].(map[string]any)["resolved"])
	require.Equal(t, false, rows[1].(map[string]any)["resolved"])

	// Outside the space, an existing page resolves exactly like a missing one.
	theirs := call(t, r, "carol", http.MethodPost, "/docs/page-links/titles",
		map[string]any{"page_ids": []string{target}})
	require.Equal(t, http.StatusOK, theirs.Code, theirs.Body)
	rows, _ = theirs.Body["data"].([]any)
	require.Equal(t, false, rows[0].(map[string]any)["resolved"])
	require.Empty(t, rows[0].(map[string]any)["title"])

	// The body is required rather than assumed empty.
	require.Equal(t, http.StatusBadRequest,
		call(t, r, "alice", http.MethodPost, "/docs/page-links/titles", map[string]any{}).Code)
}

func TestSuggestionRoutesOfferOnlyWhatTheCallerCanReach(t *testing.T) {
	r, target, _ := linkFixture(t)

	pages := call(t, r, "viewer", http.MethodGet, "/docs/page-links/suggest?q=page", nil)
	require.Equal(t, http.StatusOK, pages.Code, pages.Body)
	require.NotEmpty(t, pages.Body["data"])

	// Somebody in no space gets an empty list rather than an error.
	none := call(t, r, "carol", http.MethodGet, "/docs/page-links/suggest?q=page", nil)
	require.Equal(t, http.StatusOK, none.Code, none.Body)
	rows, _ := none.Body["data"].([]any)
	require.Empty(t, rows)

	people := call(t, r, "viewer", http.MethodGet, "/docs/pages/"+target+"/mention-candidates", nil)
	require.Equal(t, http.StatusOK, people.Code, people.Body)
	require.NotEmpty(t, people.Body["data"])

	require.Equal(t, http.StatusNotFound,
		call(t, r, "carol", http.MethodGet, "/docs/pages/"+target+"/mention-candidates", nil).Code)
}
