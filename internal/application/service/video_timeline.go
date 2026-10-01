package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/magicyuan876/yuheng/internal/infrastructure/docparser"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/models/asr"
	"github.com/magicyuan876/yuheng/internal/types"
)

// Video timeline construction: ASR transcript segments are aggregated into
// retrieval-sized windows, keyframes are assigned to their windows by
// timestamp, and each window becomes one text chunk whose Metadata carries
// the window's start/end milliseconds. The chunks keep ChunkTypeText so the
// whole retrieval/summary/QA pipeline works on them unchanged.

const (
	// defaultVideoWindowTargetChars closes a transcript window once it has
	// accumulated roughly this much text (mirrors typical chunk sizes).
	defaultVideoWindowTargetChars = 800
	// defaultVideoWindowMaxMs closes a window after this much timeline time
	// even when the transcript is sparse, so citations stay precise.
	defaultVideoWindowMaxMs = int64(90_000)
	// videoWindowMergeTailFraction: a trailing window shorter than
	// targetChars/4 is merged into its predecessor to avoid fragment chunks.
	videoWindowMergeTailDivisor = 4
)

// videoTimelineWindow is one transcript window with its assigned keyframes.
type videoTimelineWindow struct {
	StartMs int64
	EndMs   int64
	Text    string
	Frames  []types.ImageRef
}

// buildVideoTimelineWindows aggregates ASR segments into timeline windows and
// assigns each keyframe to the window covering its timestamp. Returns nil
// when there are no usable segments AND no frames (caller falls back to plain
// text chunking of the full transcript).
func buildVideoTimelineWindows(
	segments []asr.Segment,
	frames []types.ImageRef,
	durationMs int64,
	targetChars int,
	maxWindowMs int64,
) []videoTimelineWindow {
	if targetChars <= 0 {
		targetChars = defaultVideoWindowTargetChars
	}
	if maxWindowMs <= 0 {
		maxWindowMs = defaultVideoWindowMaxMs
	}

	var windows []videoTimelineWindow

	// Segments with usable timestamps drive the windows.
	usable := make([]asr.Segment, 0, len(segments))
	for _, seg := range segments {
		if strings.TrimSpace(seg.Text) == "" {
			continue
		}
		usable = append(usable, seg)
	}

	if len(usable) > 0 {
		var cur *videoTimelineWindow
		var curChars int
		for _, seg := range usable {
			segStart := int64(seg.Start * 1000)
			segEnd := int64(seg.End * 1000)
			if segEnd < segStart {
				segEnd = segStart
			}
			text := strings.TrimSpace(seg.Text)

			if cur == nil {
				windows = append(windows, videoTimelineWindow{StartMs: segStart, EndMs: segEnd, Text: text})
				cur = &windows[len(windows)-1]
				curChars = len([]rune(text))
			} else {
				cur.Text += " " + text
				if segEnd > cur.EndMs {
					cur.EndMs = segEnd
				}
				curChars += len([]rune(text)) + 1
			}

			if curChars >= targetChars || cur.EndMs-cur.StartMs >= maxWindowMs {
				cur = nil
				curChars = 0
			}
		}

		// Merge an undersized trailing window into its predecessor — but never
		// past the duration cap, or a citation would cover far too much video.
		if n := len(windows); n >= 2 {
			tail := windows[n-1]
			prev := &windows[n-2]
			mergedEnd := prev.EndMs
			if tail.EndMs > mergedEnd {
				mergedEnd = tail.EndMs
			}
			if len([]rune(tail.Text)) < targetChars/videoWindowMergeTailDivisor &&
				mergedEnd-prev.StartMs <= maxWindowMs {
				prev.Text += " " + tail.Text
				prev.EndMs = mergedEnd
				prev.Frames = append(prev.Frames, tail.Frames...)
				windows = windows[:n-1]
			}
		}
	}

	sortedFrames := make([]types.ImageRef, 0, len(frames))
	for _, f := range frames {
		if len(f.ImageData) > 0 {
			sortedFrames = append(sortedFrames, f)
		}
	}
	sort.SliceStable(sortedFrames, func(i, j int) bool {
		return sortedFrames[i].TimestampMs < sortedFrames[j].TimestampMs
	})

	if len(windows) == 0 {
		if len(sortedFrames) == 0 {
			return nil
		}
		// No transcript: build one window per frame so the visual timeline is
		// still chunked and citable by timestamp.
		windows = make([]videoTimelineWindow, 0, len(sortedFrames))
		for i, f := range sortedFrames {
			end := durationMs
			if i+1 < len(sortedFrames) {
				end = sortedFrames[i+1].TimestampMs
			}
			if end < f.TimestampMs {
				end = f.TimestampMs
			}
			windows = append(windows, videoTimelineWindow{
				StartMs: f.TimestampMs,
				EndMs:   end,
				Frames:  []types.ImageRef{f},
			})
		}
		return windows
	}

	// Assign frames to the window covering their timestamp; frames before the
	// first window go to the first, frames after the last go to the last.
	for _, f := range sortedFrames {
		idx := sort.Search(len(windows), func(i int) bool {
			return windows[i].EndMs > f.TimestampMs
		})
		if idx >= len(windows) {
			idx = len(windows) - 1
		}
		windows[idx].Frames = append(windows[idx].Frames, f)
	}

	return windows
}

// formatVideoTimestamp renders milliseconds as MM:SS, or HH:MM:SS beyond an
// hour, for markdown headings and UI badges.
func formatVideoTimestamp(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	totalSec := ms / 1000
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// videoWindowMarkdown renders one timeline window as a markdown section:
// a bold time-range heading, the window's keyframes as image references,
// then the transcript text.
func videoWindowMarkdown(w videoTimelineWindow) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**[%s - %s]**\n\n",
		formatVideoTimestamp(w.StartMs), formatVideoTimestamp(w.EndMs)))
	for _, f := range w.Frames {
		sb.WriteString(fmt.Sprintf("![%s](%s)\n\n", f.Filename, f.OriginalRef))
	}
	if text := strings.TrimSpace(w.Text); text != "" {
		sb.WriteString(text)
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildVideoMarkdown renders the full timeline document (window sections
// separated by blank lines). Frame references use their OriginalRef paths;
// the image resolver later replaces them with provider:// serving URLs.
func buildVideoMarkdown(windows []videoTimelineWindow) string {
	sections := make([]string, 0, len(windows))
	for _, w := range windows {
		sections = append(sections, strings.TrimRight(videoWindowMarkdown(w), "\n"))
	}
	return strings.Join(sections, "\n\n") + "\n"
}

// prepareVideoTimeline transcribes a parsed video's audio track (preserving
// ASR segment timestamps) and rebuilds convertResult.MarkdownContent as a
// timeline document of transcript windows and keyframe references.
//
// Returns (windows, true, nil) on success — windows may be nil when the ASR
// provider returned no segment timestamps, in which case the markdown is a
// plain transcript plus a keyframe appendix and the caller falls back to the
// ordinary text chunker. Returns (nil, false, nil) when processing must stop
// and the knowledge row has already been marked failed; a non-nil error means
// a retryable failure.
func (s *knowledgeService) prepareVideoTimeline(
	ctx context.Context,
	knowledge *types.Knowledge,
	eff types.EffectiveProcessConfig,
	convertResult *types.ReadResult,
	isLastRetry bool,
) ([]videoTimelineWindow, bool, error) {
	var transcript string
	var segments []asr.Segment

	// The audio track arrives as fixed-duration segments. Each segment is
	// transcribed independently: provider timestamps are shifted by the
	// segment's offset, and providers without timestamp support (e.g.
	// DashScope Qwen-ASR) still yield a segment-level timeline.
	tracks := convertResult.AudioSegments

	if len(tracks) > 0 {
		if !eff.ASRConfig.IsASREnabled() {
			if !eff.EnableMultimodel {
				// The entry guard requires ASR or multimodal, so this can only
				// happen when the effective config changed mid-flight.
				knowledge.ParseStatus = types.ParseStatusFailed
				knowledge.ErrorMessage = "上传视频文件需要设置ASR语音识别模型（转写音轨）或启用多模态（描述关键帧）"
				knowledge.UpdatedAt = time.Now()
				s.repo.UpdateKnowledge(ctx, knowledge)
				return nil, false, nil
			}
			logger.Warnf(ctx,
				"[Video] Audio track present but ASR not configured for %s; building frame-only timeline",
				knowledge.ID)
		} else {
			asrModel, err := s.modelService.GetASRModel(ctx, eff.ASRConfig.ModelID)
			if err != nil {
				logger.Errorf(ctx, "[Video] Failed to get ASR model: %v", err)
				knowledge.ParseStatus = types.ParseStatusFailed
				knowledge.ErrorMessage = fmt.Sprintf("failed to get ASR model: %v", err)
				knowledge.UpdatedAt = time.Now()
				s.repo.UpdateKnowledge(ctx, knowledge)
				return nil, false, nil
			}

			audioName := audioTrackFileName(knowledge.FileName, convertResult.AudioMimeType)
			var textParts []string
			for i, track := range tracks {
				logger.Infof(ctx, "[Video] Transcribing audio segment %d/%d for %s: %d bytes [%s - %s]",
					i+1, len(tracks), knowledge.ID, len(track.Data),
					formatVideoTimestamp(track.StartMs), formatVideoTimestamp(track.EndMs))
				result, err := asrModel.Transcribe(ctx, track.Data, audioName)
				if err != nil {
					logger.Errorf(ctx, "[Video] Transcription of segment %d failed: %v", i+1, err)
					if isLastRetry {
						knowledge.ParseStatus = types.ParseStatusFailed
						knowledge.ErrorMessage = fmt.Sprintf("video audio transcription failed: %v", err)
						knowledge.UpdatedAt = time.Now()
						s.repo.UpdateKnowledge(ctx, knowledge)
					}
					return nil, false, fmt.Errorf("video audio transcription failed: %w", err)
				}
				if result == nil {
					continue
				}
				text := strings.TrimSpace(result.Text)
				if text == "" {
					continue
				}
				textParts = append(textParts, text)

				if len(result.Segments) > 0 {
					offsetSec := float64(track.StartMs) / 1000
					for _, seg := range result.Segments {
						segments = append(segments, asr.Segment{
							Start: seg.Start + offsetSec,
							End:   seg.End + offsetSec,
							Text:  seg.Text,
						})
					}
				} else {
					endMs := track.EndMs
					if endMs <= track.StartMs {
						endMs = convertResult.VideoDurationMs
					}
					segments = append(segments, asr.Segment{
						Start: float64(track.StartMs) / 1000,
						End:   float64(endMs) / 1000,
						Text:  text,
					})
				}
			}
			transcript = strings.Join(textParts, " ")
			logger.Infof(ctx, "[Video] Transcription done: %d tracks, %d chars, %d segments",
				len(tracks), len(transcript), len(segments))
		}
	}

	// Audio bytes are consumed; never let the audio-file path re-transcribe.
	convertResult.AudioSegments = nil
	convertResult.IsAudio = false

	windows := buildVideoTimelineWindows(
		segments, convertResult.ImageRefs, convertResult.VideoDurationMs,
		videoWindowTargetChars(eff.ChunkingConfig), defaultVideoWindowMaxMs)

	switch {
	case len(windows) > 0:
		convertResult.MarkdownContent = buildVideoMarkdown(windows)
	case transcript != "":
		// Transcript without timestamps: plain text plus a keyframe appendix.
		convertResult.MarkdownContent = transcript + buildVideoFrameAppendix(convertResult.ImageRefs)
	default:
		knowledge.ParseStatus = types.ParseStatusFailed
		knowledge.ErrorMessage = "视频解析未产出可入库内容（无可转写音轨且未提取到关键帧）"
		knowledge.UpdatedAt = time.Now()
		s.repo.UpdateKnowledge(ctx, knowledge)
		return nil, false, nil
	}
	return windows, true, nil
}

// videoWindowTargetChars derives the transcript window size from the KB's
// chunking config so video chunks stay comparable to document chunks.
func videoWindowTargetChars(cc types.ChunkingConfig) int {
	if cc.ChunkSize > 0 {
		return cc.ChunkSize
	}
	return defaultVideoWindowTargetChars
}

// audioTrackFileName names the extracted track for the ASR upload; the
// extension drives MIME detection in OpenAI-compatible transcription APIs.
func audioTrackFileName(videoFileName, audioMime string) string {
	base := strings.TrimSuffix(videoFileName, "."+getFileType(videoFileName))
	if base == "" {
		base = "video_audio"
	}
	ext := ".mp3"
	if strings.Contains(audioMime, "wav") {
		ext = ".wav"
	}
	return base + ext
}

// buildVideoFrameAppendix renders keyframes as a trailing markdown section
// for the no-timestamp fallback document.
func buildVideoFrameAppendix(frames []types.ImageRef) string {
	if len(frames) == 0 {
		return "\n"
	}
	var sb strings.Builder
	sb.WriteString("\n\n")
	for _, f := range frames {
		sb.WriteString(fmt.Sprintf("**[%s]**\n\n![%s](%s)\n\n",
			formatVideoTimestamp(f.TimestampMs), f.Filename, f.OriginalRef))
	}
	return sb.String()
}

// buildVideoParsedChunks converts timeline windows into ParsedChunks whose
// content matches the stored markdown (frame refs replaced with serving
// URLs) and whose Metadata carries the window's time range. Start/End are
// rune offsets consistent with the joined markdown produced by
// buildVideoMarkdown after the same replacements.
func buildVideoParsedChunks(
	windows []videoTimelineWindow, storedImages []docparser.StoredImage,
) []types.ParsedChunk {
	replacements := make([]string, 0, len(storedImages)*2)
	for _, img := range storedImages {
		if img.OriginalRef != "" && img.ServingURL != "" {
			replacements = append(replacements, img.OriginalRef, img.ServingURL)
		}
	}
	replacer := strings.NewReplacer(replacements...)

	chunks := make([]types.ParsedChunk, 0, len(windows))
	offset := 0
	for i, w := range windows {
		content := strings.TrimRight(replacer.Replace(videoWindowMarkdown(w)), "\n")
		runeLen := len([]rune(content))
		chunks = append(chunks, types.ParsedChunk{
			Content:  content,
			Seq:      i,
			Start:    offset,
			End:      offset + runeLen,
			Metadata: types.NewVideoSegmentChunkMetadata(w.StartMs, w.EndMs),
		})
		// +2 for the blank-line separator between sections.
		offset += runeLen + 2
	}
	return chunks
}
