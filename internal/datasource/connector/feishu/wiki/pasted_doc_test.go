package wiki

import (
	"context"
	"testing"

	"github.com/magicyuan876/yuheng/internal/datasource/connector/feishu/core"
)

// Pasted-URL resource IDs ("node:<token>") reach docs in hidden personal
// document libraries that never appear in the space listing. ListResources
// resolves them to a single selectable resource, and the sync ops resolve the
// owning space via get_node before walking the subtree.
func TestPastedWikiDocResourceID(t *testing.T) {
	top := []core.WikiNode{
		{NodeToken: "docTok", ObjToken: "obj1", ObjType: "docx", Title: "Personal Doc", HasChild: true},
	}
	children := map[string][]core.WikiNode{
		"docTok": {{NodeToken: "childTok", ObjToken: "obj2", ObjType: "docx", Title: "Child Doc"}},
	}
	ts, cfg := fakeFeishuHierarchy(top, children, "")
	defer ts.Close()

	ctx := context.Background()
	c := NewConnector(core.RegionFeishu)
	dsCfg := makeConfig(cfg, nil)

	// Picker validation path: a pasted token comes in as parentID "node:<token>".
	resources, err := c.ListResources(ctx, dsCfg, "node:docTok")
	if err != nil {
		t.Fatalf("ListResources(node:docTok): %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("resources = %d, want 1", len(resources))
	}
	if resources[0].ExternalID != "node:docTok" {
		t.Errorf("ExternalID = %q, want %q", resources[0].ExternalID, "node:docTok")
	}
	if resources[0].Name != "Personal Doc" {
		t.Errorf("Name = %q, want %q", resources[0].Name, "Personal Doc")
	}
	if resources[0].HasChildren {
		t.Error("HasChildren = true, want false (pasted docs are not expandable in the picker)")
	}

	// Unknown token surfaces the API error instead of an empty result.
	if _, err := c.ListResources(ctx, dsCfg, "node:missing"); err == nil {
		t.Error("ListResources(node:missing) succeeded, want error")
	}

	// Sync path: ops.List resolves the hidden space and returns the doc subtree.
	feishuConfig, err := core.ParseFeishuConfig(dsCfg, core.RegionFeishu)
	if err != nil {
		t.Fatalf("ParseFeishuConfig: %v", err)
	}
	client := core.NewClient(feishuConfig)
	nodes, partial, err := wikiOps{region: core.RegionFeishu}.List(ctx, client, "node:docTok")
	if err != nil {
		t.Fatalf("wikiOps.List(node:docTok): %v", err)
	}
	if partial != nil {
		t.Fatalf("partial error: %v", partial)
	}
	tokens := make([]string, 0, len(nodes))
	for _, n := range nodes {
		tokens = append(tokens, n.NodeToken)
	}
	if len(nodes) != 2 || tokens[0] != "docTok" || tokens[1] != "childTok" {
		t.Errorf("node tokens = %v, want [docTok childTok]", tokens)
	}

	// Browse-form IDs still parse locally without extra API calls.
	spaceID, nodeToken, err := resolveWikiResourceID(ctx, client, "space1:docTok")
	if err != nil || spaceID != "space1" || nodeToken != "docTok" {
		t.Errorf("resolveWikiResourceID(space1:docTok) = %q,%q,%v", spaceID, nodeToken, err)
	}
}
