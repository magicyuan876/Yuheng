package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// Acceptance (T1.2): the page routes enforce roles, a deleted page answers
// 410 with restore details for those who could read it, and moves take
// after_id in its three forms.
func TestPageRoutesTreeTrashAndGone(t *testing.T) {
	r, _ := newSpaceRouter(t)

	sp := call(t, r, "alice", http.MethodPost, "/docs/spaces", map[string]any{"name": "Handbook"})
	require.Equal(t, http.StatusCreated, sp.Code, sp.Body)
	sid := data(sp)["id"].(string)
	set := call(t, r, "alice", http.MethodPut, "/docs/spaces/"+sid+"/members", map[string]any{
		"members": []map[string]any{
			{"principal_type": "user", "principal_id": "bob", "role": "writer"},
			{"principal_type": "user", "principal_id": "viewer", "role": "reader"},
		},
	})
	require.Equal(t, http.StatusOK, set.Code, set.Body)

	mk := func(user string, parent any, title string) map[string]any {
		res := call(t, r, user, http.MethodPost, "/docs/pages",
			map[string]any{"space_id": sid, "parent_id": parent, "title": title})
		require.Equal(t, http.StatusCreated, res.Code, res.Body)
		return data(res)
	}
	a := mk("alice", nil, "A")
	b := mk("bob", nil, "B")
	c := mk("bob", nil, "C")
	a1 := mk("bob", a["id"], "A.1")
	aid, bid, cid := a["id"].(string), b["id"].(string), c["id"].(string)

	// Readers cannot create; strangers get 404 for the space.
	require.Equal(t, http.StatusForbidden,
		call(t, r, "viewer", http.MethodPost, "/docs/pages", map[string]any{"space_id": sid, "title": "x"}).Code)
	require.Equal(t, http.StatusNotFound,
		call(t, r, "carol", http.MethodPost, "/docs/pages", map[string]any{"space_id": sid, "title": "x"}).Code)
	require.Equal(t, http.StatusBadRequest,
		call(t, r, "bob", http.MethodPost, "/docs/pages", map[string]any{"title": "no space"}).Code)

	// Reads by id and by short id; the tree and children listings.
	got := call(t, r, "viewer", http.MethodGet, "/docs/pages/"+aid, nil)
	require.Equal(t, http.StatusOK, got.Code)
	require.Equal(t, "reader", data(got)["role"])
	require.Equal(t, true, data(got)["has_children"])
	require.Equal(t, false, data(got)["can_edit"])
	short := call(t, r, "viewer", http.MethodGet, "/docs/pages/by-short-id/"+a["short_id"].(string), nil)
	require.Equal(t, http.StatusOK, short.Code)
	require.Equal(t, aid, data(short)["id"])
	tree := call(t, r, "viewer", http.MethodGet, "/docs/spaces/"+sid+"/tree", nil)
	require.Equal(t, http.StatusOK, tree.Code)
	require.Len(t, data(tree)["items"], 3)
	kids := call(t, r, "viewer", http.MethodGet, "/docs/pages/"+aid+"/children", nil)
	require.Equal(t, http.StatusOK, kids.Code)
	require.Len(t, data(kids)["items"], 1)
	anc := call(t, r, "viewer", http.MethodGet, "/docs/pages/"+a1["id"].(string)+"/ancestors", nil)
	require.Equal(t, http.StatusOK, anc.Code)
	require.Len(t, anc.Body["data"], 1)
	content := call(t, r, "viewer", http.MethodGet, "/docs/pages/"+aid+"/content?format=html", nil)
	require.Equal(t, http.StatusOK, content.Code)
	require.NotEmpty(t, data(content)["content"])
	require.Equal(t, http.StatusBadRequest,
		call(t, r, "viewer", http.MethodGet, "/docs/pages/"+aid+"/content?format=pdf", nil).Code)

	// Metadata updates need the writer role.
	require.Equal(t, http.StatusForbidden,
		call(t, r, "viewer", http.MethodPatch, "/docs/pages/"+aid, map[string]any{"title": "x"}).Code)
	upd := call(t, r, "bob", http.MethodPatch, "/docs/pages/"+aid, map[string]any{"title": "Alpha", "icon": "📗"})
	require.Equal(t, http.StatusOK, upd.Code, upd.Body)
	require.Equal(t, "Alpha", data(upd)["title"])

	// Moves: after_id absent (end), null (first), value (after that sibling).
	titles := func(user string) []string {
		res := call(t, r, user, http.MethodGet, "/docs/spaces/"+sid+"/tree", nil)
		require.Equal(t, http.StatusOK, res.Code)
		items := data(res)["items"].([]any)
		out := make([]string, len(items))
		for i, it := range items {
			out[i] = it.(map[string]any)["title"].(string)
		}
		return out
	}
	require.Equal(t, []string{"Alpha", "B", "C"}, titles("viewer"))
	mv := call(t, r, "bob", http.MethodPost, "/docs/pages/"+aid+"/move", map[string]any{"parent_id": nil})
	require.Equal(t, http.StatusOK, mv.Code, mv.Body)
	require.Equal(t, []string{"B", "C", "Alpha"}, titles("viewer"))
	mv = call(t, r, "bob", http.MethodPost, "/docs/pages/"+cid+"/move", map[string]any{"after_id": nil})
	require.Equal(t, http.StatusOK, mv.Code, mv.Body)
	require.Equal(t, []string{"C", "B", "Alpha"}, titles("viewer"))
	mv = call(t, r, "bob", http.MethodPost, "/docs/pages/"+aid+"/move", map[string]any{"after_id": cid})
	require.Equal(t, http.StatusOK, mv.Code, mv.Body)
	require.Equal(t, []string{"C", "Alpha", "B"}, titles("viewer"))
	mv = call(t, r, "bob", http.MethodPost, "/docs/pages/"+bid+"/move", map[string]any{"parent_id": aid})
	require.Equal(t, http.StatusOK, mv.Code, mv.Body)
	require.Equal(t, []string{"C", "Alpha"}, titles("viewer"))
	require.Equal(t, http.StatusBadRequest,
		call(t, r, "bob", http.MethodPost, "/docs/pages/"+aid+"/move", map[string]any{"parent_id": bid}).Code,
		"cannot move under own descendant")
	require.Equal(t, http.StatusForbidden,
		call(t, r, "viewer", http.MethodPost, "/docs/pages/"+aid+"/move", map[string]any{"after_id": nil}).Code)

	// Duplicate next to the original.
	dup := call(t, r, "bob", http.MethodPost, "/docs/pages/"+aid+"/duplicate", nil)
	require.Equal(t, http.StatusCreated, dup.Code, dup.Body)
	require.Equal(t, float64(3), data(dup)["count"], "Alpha, A.1 and B")
	require.Equal(t, []string{"C", "Alpha", "Alpha (copy)"}, titles("viewer"))
	dupID := data(dup)["page"].(map[string]any)["id"].(string)

	// Delete -> 410 with restore details for readers/writers, 404 for others.
	require.Equal(t, http.StatusForbidden, call(t, r, "viewer", http.MethodDelete, "/docs/pages/"+dupID, nil).Code)
	del := call(t, r, "bob", http.MethodDelete, "/docs/pages/"+dupID, nil)
	require.Equal(t, http.StatusOK, del.Code, del.Body)
	require.Equal(t, float64(3), data(del)["deleted"])
	gone := call(t, r, "viewer", http.MethodGet, "/docs/pages/"+dupID, nil)
	require.Equal(t, http.StatusGone, gone.Code)
	require.Equal(t, false, data(gone)["restorable"])
	gone = call(t, r, "bob", http.MethodGet, "/docs/pages/"+dupID, nil)
	require.Equal(t, http.StatusGone, gone.Code)
	require.Equal(t, true, data(gone)["restorable"])
	require.Equal(t, http.StatusNotFound, call(t, r, "carol", http.MethodGet, "/docs/pages/"+dupID, nil).Code)
	require.Equal(t, http.StatusNotFound, call(t, r, "viewer", http.MethodGet, "/docs/pages/nope", nil).Code)
	require.Equal(t, http.StatusGone,
		call(t, r, "bob", http.MethodGet, "/docs/pages/"+dupID+"/children", nil).Code)

	trash := call(t, r, "viewer", http.MethodGet, "/docs/spaces/"+sid+"/trash", nil)
	require.Equal(t, http.StatusOK, trash.Code)
	require.Len(t, trash.Body["data"], 1)

	// Restore, then permanently delete as space admin.
	require.Equal(t, http.StatusForbidden,
		call(t, r, "viewer", http.MethodPost, "/docs/pages/"+dupID+"/restore", nil).Code)
	restored := call(t, r, "bob", http.MethodPost, "/docs/pages/"+dupID+"/restore", nil)
	require.Equal(t, http.StatusOK, restored.Code, restored.Body)
	require.Equal(t, []string{"C", "Alpha", "Alpha (copy)"}, titles("viewer"))
	require.Equal(t, http.StatusConflict,
		call(t, r, "bob", http.MethodPost, "/docs/pages/"+dupID+"/restore", nil).Code)
	require.Equal(t, http.StatusOK, call(t, r, "bob", http.MethodDelete, "/docs/pages/"+dupID, nil).Code)
	require.Equal(t, http.StatusForbidden,
		call(t, r, "bob", http.MethodDelete, "/docs/spaces/"+sid+"/trash/"+dupID, nil).Code)
	purged := call(t, r, "alice", http.MethodDelete, "/docs/spaces/"+sid+"/trash/"+dupID, nil)
	require.Equal(t, http.StatusOK, purged.Code, purged.Body)
	require.Len(t, data(purged)["purged"], 3)
	require.Equal(t, http.StatusNotFound, call(t, r, "bob", http.MethodGet, "/docs/pages/"+dupID, nil).Code)
	empty := call(t, r, "alice", http.MethodDelete, "/docs/spaces/"+sid+"/trash", nil)
	require.Equal(t, http.StatusOK, empty.Code)
	require.Equal(t, float64(0), data(empty)["purged"])
}
