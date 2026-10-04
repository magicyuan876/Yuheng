package docs

import (
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/types"
)

// The knowledge base made for a space gets the model the knowledge-base editor
// would have preselected, and the same one every time.
func TestDefaultModelIDPrefersTheDefaultThenTheOldestActiveModel(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	embed := func(id string, age int, isDefault bool, status types.ModelStatus) *types.Model {
		return &types.Model{
			ID: id, Type: types.ModelTypeEmbedding, IsDefault: isDefault, Status: status,
			CreatedAt: t0.Add(time.Duration(age) * time.Hour),
		}
	}
	chat := &types.Model{ID: "chat", Type: types.ModelTypeKnowledgeQA, IsDefault: true}

	cases := []struct {
		name   string
		models []*types.Model
		want   string
	}{
		{"none at all", nil, ""},
		{"only another type", []*types.Model{chat}, ""},
		{"the oldest", []*types.Model{embed("newer", 2, false, ""), embed("older", 1, false, "")}, "older"},
		{
			"the default beats the oldest",
			[]*types.Model{embed("older", 1, false, ""), embed("default", 3, true, types.ModelStatusActive)},
			"default",
		},
		{
			"a model still downloading is not active",
			[]*types.Model{embed("downloading", 1, true, types.ModelStatusDownloading), embed("ready", 2, false, "")},
			"ready",
		},
	}
	for _, c := range cases {
		if got := defaultModelID(c.models, types.ModelTypeEmbedding); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
