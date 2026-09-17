package core

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/magicyuan876/yuheng/internal/types"
)

func TestFetchOptionsFromConfigDefaults(t *testing.T) {
	opts := FetchOptionsFromConfig(nil)
	assert.True(t, opts.SyncVideos, "video sync defaults on")
	assert.Equal(t, int64(DefaultVideoMaxMB)*1024*1024, opts.VideoMaxBytes)
	assert.False(t, opts.Multimodal)

	opts = FetchOptionsFromConfig(&types.DataSourceConfig{MultimodalEnabled: true})
	assert.True(t, opts.Multimodal)
	assert.True(t, opts.SyncVideos)
}

func TestFetchOptionsFromConfigSettings(t *testing.T) {
	cfg := &types.DataSourceConfig{Settings: map[string]interface{}{
		"sync_video_attachments": false,
		// JSON numbers decode to float64.
		"video_max_mb": float64(512),
	}}
	opts := FetchOptionsFromConfig(cfg)
	assert.False(t, opts.SyncVideos)
	assert.Equal(t, int64(512)*1024*1024, opts.VideoMaxBytes)

	// Garbage / non-positive values keep defaults.
	cfg = &types.DataSourceConfig{Settings: map[string]interface{}{
		"sync_video_attachments": "yes",
		"video_max_mb":           float64(-3),
	}}
	opts = FetchOptionsFromConfig(cfg)
	assert.True(t, opts.SyncVideos)
	assert.Equal(t, int64(DefaultVideoMaxMB)*1024*1024, opts.VideoMaxBytes)
}

func TestIsVideoAttachmentExt(t *testing.T) {
	assert.True(t, IsVideoAttachmentExt(".mp4"))
	assert.True(t, IsVideoAttachmentExt(".MOV"))
	assert.False(t, IsVideoAttachmentExt(".pdf"))
	assert.False(t, IsVideoAttachmentExt(""))
}

func TestCollectVideoAttachments(t *testing.T) {
	blocks := []DocxBlock{
		{BlockType: BlockTypeFile, File: &BlockFileRef{Token: "tok-video", Name: "讲座.mp4"}},
		{BlockType: BlockTypeFile, File: &BlockFileRef{Token: "tok-pdf", Name: "资料.pdf"}},
		{BlockType: BlockTypeFile, File: &BlockFileRef{Token: "", Name: "空token.mp4"}},
		{BlockType: BlockTypeImage},
	}
	vids := collectVideoAttachments(blocks)
	assert.Len(t, vids, 1)
	assert.Equal(t, "tok-video", vids[0].FileToken)
	assert.Equal(t, "讲座.mp4", vids[0].Name)
}

func TestEmitVideoItemRouting(t *testing.T) {
	vi := &types.FetchedItem{ExternalID: "child:video"}

	// Without EmitEarly the item accumulates in the returned slice.
	items, err := emitVideoItem(DocxFetchInput{}, nil, vi)
	assert.NoError(t, err)
	assert.Len(t, items, 1)

	// With EmitEarly the item goes to the callback and never accumulates.
	var emitted []string
	in := DocxFetchInput{EmitEarly: func(it *types.FetchedItem) error {
		emitted = append(emitted, it.ExternalID)
		return nil
	}}
	items, err = emitVideoItem(in, nil, vi)
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, []string{"child:video"}, emitted)

	// An Emit failure aborts the fetch.
	in.EmitEarly = func(*types.FetchedItem) error { return assert.AnError }
	_, err = emitVideoItem(in, nil, vi)
	assert.Error(t, err)
}
