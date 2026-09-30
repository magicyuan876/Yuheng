package service

import (
	"context"
	"strings"
	"testing"

	"github.com/magicyuan876/yuheng/internal/models/chat"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// indexIntroWikiService serves one index page and records what is written back.
// Methods rebuildIndexPage does not call fall through to the nil embedded
// interface and would panic, which keeps the fake honest about its scope.
type indexIntroWikiService struct {
	interfaces.WikiPageService
	index   *types.WikiPage
	updated *types.WikiPage
}

func (f *indexIntroWikiService) GetIndex(context.Context, string) (*types.WikiPage, error) {
	return f.index, nil
}

func (f *indexIntroWikiService) ListByTypeRecent(context.Context, string, string, int) ([]types.WikiIndexEntry, error) {
	return []types.WikiIndexEntry{{Title: "Doc A", Summary: "About A"}}, nil
}

func (f *indexIntroWikiService) CountByType(context.Context, string) (map[string]int64, error) {
	return map[string]int64{types.WikiPageTypeSummary: 1}, nil
}

func (f *indexIntroWikiService) UpdatePage(_ context.Context, page *types.WikiPage) (*types.WikiPage, error) {
	f.updated = page
	return page, nil
}

// promptRecordingChat answers every call with a fixed intro and keeps the
// prompts it was sent.
type promptRecordingChat struct {
	prompts []string
}

func (c *promptRecordingChat) Chat(
	_ context.Context, messages []chat.Message, _ *chat.ChatOptions,
) (*types.ChatResponse, error) {
	c.prompts = append(c.prompts, messages[len(messages)-1].Content)
	return &types.ChatResponse{Content: "Generated intro"}, nil
}

func (c *promptRecordingChat) ChatStream(
	context.Context, []chat.Message, *chat.ChatOptions,
) (<-chan types.StreamResponse, error) {
	return nil, nil
}
func (c *promptRecordingChat) GetModelName() string { return "fake" }
func (c *promptRecordingChat) GetModelID() string   { return "fake" }

// A freshly created index page carries defaultWikiIndexContent. It must count
// as "no intro yet", so the first batch writes an intro from the document
// summaries instead of incrementally "updating" the placeholder text. The
// check once compared against a placeholder string no code wrote any more,
// so no index page ever got a first-time intro.
func TestRebuildIndexPageTreatsDefaultPlaceholderAsNoIntro(t *testing.T) {
	wiki := &indexIntroWikiService{index: &types.WikiPage{
		Slug: "index", PageType: types.WikiPageTypeIndex, Content: defaultWikiIndexContent,
	}}
	model := &promptRecordingChat{}
	svc := &wikiIngestService{wikiService: wiki}

	err := svc.rebuildIndexPage(context.Background(), model, WikiIngestPayload{KnowledgeBaseID: "kb-1"},
		"added Doc A", "English", "")
	require.NoError(t, err)

	require.Len(t, model.prompts, 1)
	require.Contains(t, model.prompts[0], "About A", "first-time intro must be built from document summaries")
	require.False(t, strings.Contains(model.prompts[0], "This is the index page"),
		"the placeholder must not be fed back as an existing intro")
	require.NotNil(t, wiki.updated)
	require.Equal(t, "Generated intro", wiki.updated.Content)
	require.Equal(t, "Generated intro", wiki.updated.Summary)
}
