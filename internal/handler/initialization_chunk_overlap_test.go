package handler

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/magicyuan876/yuheng/internal/infrastructure/chunker"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/require"
)

// The KB settings endpoint takes camelCase chunking fields. An omitted
// chunkOverlap leaves the stored one alone, 0 turns overlap off, and a
// negative one is refused rather than ignored.
func TestKBModelConfigRequestChunkOverlap(t *testing.T) {
	decode := func(t *testing.T, body string) (*int, error) {
		t.Helper()
		var req KBModelConfigRequest
		err := binding.JSON.BindBody([]byte(body), &req)
		return req.DocumentSplitting.ChunkOverlap, err
	}

	got, err := decode(t, `{"llmModelId":"m","documentSplitting":{"chunkSize":512}}`)
	require.NoError(t, err)
	require.Nil(t, got)

	got, err = decode(t, `{"llmModelId":"m","documentSplitting":{"chunkSize":512,"chunkOverlap":0}}`)
	require.NoError(t, err)
	require.Equal(t, new(0), got)

	_, err = decode(t, `{"llmModelId":"m","documentSplitting":{"chunkSize":512,"chunkOverlap":-1}}`)
	require.Error(t, err)
}

// The configuration shown to the settings page reports the overlap the
// chunker will use, so a base that never chose one shows the default and one
// set to 0 shows 0.
func TestBuildConfigResponseShowsEffectiveChunkOverlap(t *testing.T) {
	h := &InitializationHandler{}
	ctx := context.WithValue(context.Background(), types.TenantRoleContextKey, types.TenantRoleAdmin)
	for _, tc := range []struct {
		name   string
		stored *int
		want   int
	}{
		{"unset", nil, chunker.DefaultChunkOverlap},
		{"zero", new(0), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kb := &types.KnowledgeBase{ChunkingConfig: types.ChunkingConfig{ChunkSize: 512, ChunkOverlap: tc.stored}}
			config := h.buildConfigResponse(ctx, nil, kb, false)
			ds, ok := config["documentSplitting"].(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, tc.want, ds["chunkOverlap"])
		})
	}
}
