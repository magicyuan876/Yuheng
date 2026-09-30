package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/infrastructure/docparser"
	"github.com/magicyuan876/yuheng/internal/models/asr"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

func seg(start, end float64, text string) asr.Segment {
	return asr.Segment{Start: start, End: end, Text: text}
}

func frameRef(tsMs int64) types.ImageRef {
	return types.ImageRef{
		Filename:    "vframe.jpg",
		OriginalRef: "video_frames/vframe.jpg",
		MimeType:    "image/jpeg",
		ImageData:   []byte{0xFF, 0xD8},
		TimestampMs: tsMs,
	}
}

func TestBuildVideoTimelineWindowsAggregatesByChars(t *testing.T) {
	long := strings.Repeat("字", 300)
	segments := []asr.Segment{
		seg(0, 10, long),
		seg(10, 20, long), // closes window 1 (>= 500 chars)
		seg(20, 30, long),
		seg(30, 40, long), // closes window 2
	}
	windows := buildVideoTimelineWindows(segments, nil, 40_000, 500, 90_000)
	require.Len(t, windows, 2)
	assert.Equal(t, int64(0), windows[0].StartMs)
	assert.Equal(t, int64(20_000), windows[0].EndMs)
	assert.Equal(t, int64(20_000), windows[1].StartMs)
	assert.Equal(t, int64(40_000), windows[1].EndMs)
}

func TestBuildVideoTimelineWindowsClosesByDuration(t *testing.T) {
	// Sparse transcript: little text but long stretches of timeline.
	segments := []asr.Segment{
		seg(0, 60, "第一句"),
		seg(60, 100, "第二句"), // window spans 100s > 90s cap → closes
		seg(100, 110, "第三句"),
		seg(110, 130, "第四句尾巴很短"),
	}
	windows := buildVideoTimelineWindows(segments, nil, 130_000, 10_000, 90_000)
	require.Len(t, windows, 2)
	assert.Equal(t, int64(0), windows[0].StartMs)
	assert.Equal(t, int64(100_000), windows[0].EndMs)
	assert.Equal(t, int64(100_000), windows[1].StartMs)
}

func TestBuildVideoTimelineWindowsMergesShortTail(t *testing.T) {
	long := strings.Repeat("a", 600)
	segments := []asr.Segment{
		seg(0, 10, long),   // closes window 1
		seg(10, 12, "短尾巴"), // would be a fragment window
	}
	windows := buildVideoTimelineWindows(segments, nil, 12_000, 500, 90_000)
	require.Len(t, windows, 1)
	assert.Contains(t, windows[0].Text, "短尾巴")
	assert.Equal(t, int64(12_000), windows[0].EndMs)
}

func TestBuildVideoTimelineWindowsAssignsFrames(t *testing.T) {
	long := strings.Repeat("a", 600)
	segments := []asr.Segment{
		seg(0, 30, long),
		seg(30, 60, long),
	}
	frames := []types.ImageRef{frameRef(5_000), frameRef(45_000), frameRef(120_000)}
	windows := buildVideoTimelineWindows(segments, frames, 60_000, 500, 90_000)
	require.Len(t, windows, 2)
	require.Len(t, windows[0].Frames, 1)
	assert.Equal(t, int64(5_000), windows[0].Frames[0].TimestampMs)
	// The 45s frame lands in window 2; the 120s frame is past the last
	// window's end and clamps to it.
	require.Len(t, windows[1].Frames, 2)
	assert.Equal(t, int64(45_000), windows[1].Frames[0].TimestampMs)
	assert.Equal(t, int64(120_000), windows[1].Frames[1].TimestampMs)
}

func TestBuildVideoTimelineWindowsFramesOnly(t *testing.T) {
	frames := []types.ImageRef{frameRef(0), frameRef(30_000)}
	windows := buildVideoTimelineWindows(nil, frames, 60_000, 500, 90_000)
	require.Len(t, windows, 2)
	assert.Equal(t, int64(0), windows[0].StartMs)
	assert.Equal(t, int64(30_000), windows[0].EndMs)
	assert.Equal(t, int64(30_000), windows[1].StartMs)
	assert.Equal(t, int64(60_000), windows[1].EndMs)
	assert.Empty(t, windows[0].Text)
}

func TestBuildVideoTimelineWindowsEmpty(t *testing.T) {
	assert.Nil(t, buildVideoTimelineWindows(nil, nil, 0, 500, 90_000))
	// Whitespace-only segments are unusable.
	assert.Nil(t, buildVideoTimelineWindows([]asr.Segment{seg(0, 1, "  ")}, nil, 1_000, 500, 90_000))
}

func TestFormatVideoTimestamp(t *testing.T) {
	assert.Equal(t, "00:00", formatVideoTimestamp(0))
	assert.Equal(t, "01:05", formatVideoTimestamp(65_000))
	assert.Equal(t, "01:00:01", formatVideoTimestamp(3_601_000))
	assert.Equal(t, "00:00", formatVideoTimestamp(-5))
}

func TestBuildVideoMarkdownAndParsedChunks(t *testing.T) {
	windows := []videoTimelineWindow{
		{StartMs: 0, EndMs: 30_000, Text: "第一段转写", Frames: []types.ImageRef{
			{Filename: "f1.jpg", OriginalRef: "video_frames/vframe_5000.jpg", TimestampMs: 5000, ImageData: []byte{1}},
		}},
		{StartMs: 30_000, EndMs: 61_000, Text: "第二段转写"},
	}

	md := buildVideoMarkdown(windows)
	assert.Contains(t, md, "**[00:00 - 00:30]**")
	assert.Contains(t, md, "**[00:30 - 01:01]**")
	assert.Contains(t, md, "![f1.jpg](video_frames/vframe_5000.jpg)")
	assert.Contains(t, md, "第一段转写")

	stored := []docparser.StoredImage{{
		OriginalRef: "video_frames/vframe_5000.jpg",
		ServingURL:  "s3://bucket/abc.jpg",
		TimestampMs: 5000,
	}}
	chunks := buildVideoParsedChunks(windows, stored)
	require.Len(t, chunks, 2)

	// Frame refs replaced with serving URLs so multimodal chunk matching works.
	assert.Contains(t, chunks[0].Content, "s3://bucket/abc.jpg")
	assert.NotContains(t, chunks[0].Content, "video_frames/vframe_5000.jpg")

	// Metadata carries the window's time range.
	var meta map[string]types.VideoSegmentMetadata
	require.NoError(t, json.Unmarshal(chunks[0].Metadata, &meta))
	assert.Equal(t, int64(0), meta[types.ChunkMetadataVideoSegmentKey].StartMs)
	assert.Equal(t, int64(30_000), meta[types.ChunkMetadataVideoSegmentKey].EndMs)

	// Rune offsets are consistent and ordered.
	assert.Equal(t, 0, chunks[0].Start)
	assert.Equal(t, len([]rune(chunks[0].Content)), chunks[0].End)
	assert.Equal(t, chunks[0].End+2, chunks[1].Start)
	assert.Equal(t, 0, chunks[0].Seq)
	assert.Equal(t, 1, chunks[1].Seq)
}

func TestAudioTrackFileName(t *testing.T) {
	assert.Equal(t, "lecture.mp3", audioTrackFileName("lecture.mp4", "audio/mpeg"))
	assert.Equal(t, "video_audio.mp3", audioTrackFileName("", ""))
	assert.Equal(t, "a.b.mp3", audioTrackFileName("a.b.mkv", "audio/mpeg"))
}

func TestVideoWindowTargetChars(t *testing.T) {
	assert.Equal(t, 1200, videoWindowTargetChars(types.ChunkingConfig{ChunkSize: 1200}))
	assert.Equal(t, defaultVideoWindowTargetChars, videoWindowTargetChars(types.ChunkingConfig{}))
}

// ---- prepareVideoTimeline service-level tests ----

// fakeVideoASR returns a canned transcription result.
type fakeVideoASR struct {
	result *asr.TranscriptionResult
	err    error
}

func (f *fakeVideoASR) Transcribe(_ context.Context, _ []byte, _ string) (*asr.TranscriptionResult, error) {
	return f.result, f.err
}
func (f *fakeVideoASR) GetModelName() string { return "fake-asr" }
func (f *fakeVideoASR) GetModelID() string   { return "fake-asr-id" }

// videoTestModelService stubs only GetASRModel; other calls panic via the
// embedded nil interface, which is what we want in these tests.
type videoTestModelService struct {
	interfaces.ModelService
	asrModel asr.ASR
	asrErr   error
}

func (s *videoTestModelService) GetASRModel(context.Context, string) (asr.ASR, error) {
	return s.asrModel, s.asrErr
}

// videoTestKnowledgeRepo records UpdateKnowledge calls.
type videoTestKnowledgeRepo struct {
	interfaces.KnowledgeRepository
	updated *types.Knowledge
}

func (r *videoTestKnowledgeRepo) UpdateKnowledge(_ context.Context, k *types.Knowledge) error {
	r.updated = k
	return nil
}

func videoTestService(asrModel asr.ASR, asrErr error) (*knowledgeService, *videoTestKnowledgeRepo) {
	repo := &videoTestKnowledgeRepo{}
	return &knowledgeService{
		modelService: &videoTestModelService{asrModel: asrModel, asrErr: asrErr},
		repo:         repo,
	}, repo
}

func asrEnabledConfig() types.EffectiveProcessConfig {
	return types.EffectiveProcessConfig{
		ASRConfig: types.ASRConfig{Enabled: true, ModelID: "asr-1"},
	}
}

func TestPrepareVideoTimelineWithSegments(t *testing.T) {
	long := strings.Repeat("字", 600)
	svc, _ := videoTestService(&fakeVideoASR{result: &asr.TranscriptionResult{
		Text:     long,
		Segments: []asr.Segment{seg(0, 30, long)},
	}}, nil)

	result := &types.ReadResult{
		IsVideo:         true,
		VideoDurationMs: 30_000,
		AudioSegments:   []types.AudioTrackSegment{{StartMs: 0, EndMs: 30_000, Data: []byte{1, 2, 3}}},
		AudioMimeType:   "audio/mpeg",
		ImageRefs:       []types.ImageRef{frameRef(5_000)},
	}
	knowledge := &types.Knowledge{ID: "k1", FileName: "talk.mp4"}

	windows, ok, err := svc.prepareVideoTimeline(
		context.Background(), knowledge, asrEnabledConfig(), result, true)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEmpty(t, windows)
	assert.Nil(t, result.AudioSegments, "audio bytes must be consumed")
	assert.Contains(t, result.MarkdownContent, "**[00:00 - 00:30]**")
	assert.Contains(t, result.MarkdownContent, "vframe")
}

func TestPrepareVideoTimelineNoProviderTimestampsUsesSegmentBounds(t *testing.T) {
	// Provider returns no timestamps (e.g. DashScope Qwen-ASR): the audio
	// track segments themselves provide a segment-level timeline.
	svc, _ := videoTestService(&fakeVideoASR{result: &asr.TranscriptionResult{
		Text: "纯文本转写，没有分段时间戳",
	}}, nil)

	result := &types.ReadResult{
		IsVideo:         true,
		VideoDurationMs: 600_000,
		AudioSegments: []types.AudioTrackSegment{
			{StartMs: 0, EndMs: 300_000, Data: []byte{1}},
			{StartMs: 300_000, EndMs: 600_000, Data: []byte{2}},
		},
	}
	knowledge := &types.Knowledge{ID: "k1", FileName: "talk.mp4"}

	windows, ok, err := svc.prepareVideoTimeline(
		context.Background(), knowledge, asrEnabledConfig(), result, true)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, windows, 2, "each audio segment becomes a timeline window")
	assert.Equal(t, int64(0), windows[0].StartMs)
	assert.Equal(t, int64(300_000), windows[0].EndMs)
	assert.Equal(t, int64(300_000), windows[1].StartMs)
	assert.Contains(t, result.MarkdownContent, "**[00:00 - 05:00]**")
	assert.Contains(t, result.MarkdownContent, "纯文本转写")
	assert.Nil(t, result.AudioSegments, "audio segments must be consumed")
}

func TestPrepareVideoTimelineShiftsProviderTimestampsBySegmentOffset(t *testing.T) {
	// Provider timestamps are relative to each audio segment; they must be
	// shifted by the segment's start offset in the full video. Text length
	// exceeds the default window target so each track closes its own window.
	long := strings.Repeat("字", 900)
	svc, _ := videoTestService(&fakeVideoASR{result: &asr.TranscriptionResult{
		Text:     long,
		Segments: []asr.Segment{seg(0, 30, long)},
	}}, nil)

	result := &types.ReadResult{
		IsVideo:         true,
		VideoDurationMs: 630_000,
		AudioSegments: []types.AudioTrackSegment{
			{StartMs: 0, EndMs: 300_000, Data: []byte{1}},
			{StartMs: 300_000, EndMs: 630_000, Data: []byte{2}},
		},
	}
	knowledge := &types.Knowledge{ID: "k1", FileName: "talk.mp4"}

	windows, ok, err := svc.prepareVideoTimeline(
		context.Background(), knowledge, asrEnabledConfig(), result, true)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, windows, 2)
	// Segment 2's provider timestamps (0-30s) shift to 300-330s.
	assert.Equal(t, int64(300_000), windows[1].StartMs)
	assert.Equal(t, int64(330_000), windows[1].EndMs)
}

func TestPrepareVideoTimelineFrameOnlyWithoutASR(t *testing.T) {
	svc, _ := videoTestService(nil, nil)

	result := &types.ReadResult{
		IsVideo:         true,
		VideoDurationMs: 60_000,
		AudioSegments:   []types.AudioTrackSegment{{EndMs: 60_000, Data: []byte{1}}}, // audio present, ASR unavailable
		ImageRefs:       []types.ImageRef{frameRef(0), frameRef(30_000)},
	}
	knowledge := &types.Knowledge{ID: "k1", FileName: "silent.mp4"}
	eff := types.EffectiveProcessConfig{EnableMultimodel: true}

	windows, ok, err := svc.prepareVideoTimeline(context.Background(), knowledge, eff, result, true)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, windows, 2)
	assert.Empty(t, windows[0].Text)
}

func TestPrepareVideoTimelineFailsWhenNothingUsable(t *testing.T) {
	svc, repo := videoTestService(nil, nil)

	result := &types.ReadResult{IsVideo: true} // no audio, no frames
	knowledge := &types.Knowledge{ID: "k1", FileName: "broken.mp4"}
	eff := types.EffectiveProcessConfig{EnableMultimodel: true}

	windows, ok, err := svc.prepareVideoTimeline(context.Background(), knowledge, eff, result, true)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, windows)
	require.NotNil(t, repo.updated)
	assert.Equal(t, types.ParseStatusFailed, repo.updated.ParseStatus)
}

func TestPrepareVideoTimelineTranscribeErrorRetries(t *testing.T) {
	svc, repo := videoTestService(&fakeVideoASR{err: assert.AnError}, nil)

	result := &types.ReadResult{IsVideo: true, AudioSegments: []types.AudioTrackSegment{{Data: []byte{1}}}}
	knowledge := &types.Knowledge{ID: "k1", FileName: "talk.mp4"}

	// Not the last retry → error returned for asynq retry, status untouched.
	_, ok, err := svc.prepareVideoTimeline(context.Background(), knowledge, asrEnabledConfig(), result, false)
	require.Error(t, err)
	assert.False(t, ok)
	assert.Nil(t, repo.updated)

	// Last retry → knowledge marked failed.
	result2 := &types.ReadResult{IsVideo: true, AudioSegments: []types.AudioTrackSegment{{Data: []byte{1}}}}
	_, ok, err = svc.prepareVideoTimeline(context.Background(), knowledge, asrEnabledConfig(), result2, true)
	require.Error(t, err)
	assert.False(t, ok)
	require.NotNil(t, repo.updated)
	assert.Equal(t, types.ParseStatusFailed, repo.updated.ParseStatus)
}

// videoTestResourceCatalog stubs only Resolve for shared-path tests.
type videoTestResourceCatalog struct {
	interfaces.ResourceCatalog
	resource *types.StoredResource
	err      error
}

func (c *videoTestResourceCatalog) Resolve(context.Context, string) (*types.StoredResource, error) {
	return c.resource, c.err
}

func TestDocreaderSharedFilePath(t *testing.T) {
	ctx := context.Background()
	svc := &knowledgeService{}

	t.Setenv("DOCREADER_SHARED_DATA_DIR", "/data/files")
	assert.Equal(t, "/data/files/videos/a.mp4",
		svc.docreaderSharedFilePath(ctx, "local://videos/a.mp4"))
	// Multi-backend prefix unwraps to the local relative path.
	assert.Equal(t, "/data/files/10000/k1/v.mp4",
		svc.docreaderSharedFilePath(ctx, "storage://backend-1/local://10000/k1/v.mp4"))
	// Traversal attempts are neutralised, non-local schemes are refused.
	assert.Equal(t, "/data/files/etc/passwd",
		svc.docreaderSharedFilePath(ctx, "local://../../etc/passwd"))
	assert.Equal(t, "", svc.docreaderSharedFilePath(ctx, "s3://bucket/a.mp4"))
	assert.Equal(t, "", svc.docreaderSharedFilePath(ctx, "local://"))

	// Stable resource references resolve through the catalog (the shape every
	// new upload uses); non-local providers and lookup failures refuse handoff.
	svc.resourceCatalog = &videoTestResourceCatalog{resource: &types.StoredResource{
		Provider:     "local",
		PhysicalPath: "storage://backend-1/local://10000/k1/1787.mp4",
	}}
	assert.Equal(t, "/data/files/10000/k1/1787.mp4",
		svc.docreaderSharedFilePath(ctx, "resource://worYS6H5gNcTguFDE6XOlw"))

	svc.resourceCatalog = &videoTestResourceCatalog{resource: &types.StoredResource{
		Provider:     "s3",
		PhysicalPath: "storage://backend-2/s3://bucket/k1/v.mp4",
	}}
	assert.Equal(t, "", svc.docreaderSharedFilePath(ctx, "resource://worYS6H5gNcTguFDE6XOlw"))

	svc.resourceCatalog = &videoTestResourceCatalog{err: assert.AnError}
	assert.Equal(t, "", svc.docreaderSharedFilePath(ctx, "resource://worYS6H5gNcTguFDE6XOlw"))

	t.Setenv("DOCREADER_SHARED_DATA_DIR", "")
	assert.Equal(t, "", svc.docreaderSharedFilePath(ctx, "local://videos/a.mp4"),
		"path handoff disabled when shared dir is unset")
}

func TestNewVideoChunkMetadataRoundTrip(t *testing.T) {
	segMeta := types.NewVideoSegmentChunkMetadata(1000, 2000)
	var seg map[string]types.VideoSegmentMetadata
	require.NoError(t, json.Unmarshal(segMeta, &seg))
	assert.Equal(t, int64(1000), seg[types.ChunkMetadataVideoSegmentKey].StartMs)

	frameMeta := types.NewVideoFrameChunkMetadata(1500)
	var frame map[string]types.VideoFrameMetadata
	require.NoError(t, json.Unmarshal(frameMeta, &frame))
	assert.Equal(t, int64(1500), frame[types.ChunkMetadataVideoFrameKey].TimestampMs)
}
