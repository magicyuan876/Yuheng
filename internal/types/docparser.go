package types

import "strings"

// ReadRequest is the unified transport-agnostic request for document reading.
// Set FileContent for file mode, URL for URL mode. FilePath is a
// shared-volume alternative to FileContent for large media: it must be a
// path valid inside the docreader container (both containers mount the same
// storage volume), letting multi-GB videos bypass gRPC message memory.
type ReadRequest struct {
	FileContent           []byte
	FileName              string
	FileType              string
	URL                   string
	Title                 string
	ParserEngine          string
	RequestID             string
	ParserEngineOverrides map[string]string
	FilePath              string
}

// ReadResult is the transport-agnostic result of document reading.
type ReadResult struct {
	MarkdownContent string
	ImageRefs       []ImageRef
	ImageDirPath    string
	Metadata        map[string]string
	Error           string
	IsAudio         bool   // true when the result contains raw audio data needing ASR transcription
	AudioData       []byte // raw audio bytes for ASR processing

	// Video fields. IsVideo marks a parsed video file: ImageRefs then carry
	// timestamped keyframes and AudioSegments carries the extracted audio
	// track split into fixed-duration pieces. Segment-wise transcription
	// keeps each ASR request under provider payload caps and yields a
	// segment-level timeline even for providers without timestamp support.
	IsVideo         bool
	VideoDurationMs int64
	AudioMimeType   string
	AudioSegments   []AudioTrackSegment
}

// AudioTrackSegment is one fixed-duration piece of an extracted audio track.
type AudioTrackSegment struct {
	StartMs int64
	EndMs   int64
	Data    []byte
}

// ImageRef represents an image reference extracted from the document.
type ImageRef struct {
	Filename    string
	OriginalRef string
	MimeType    string
	StorageKey  string
	ImageData   []byte // inline image bytes (universal fallback for cross-machine deployments)
	// IsOriginal marks references that point to the originally uploaded file
	// itself (e.g. when the user uploads a standalone image). Such references
	// must not be dropped by the icon/size filter — otherwise a small image
	// upload would be silently discarded before multimodal processing.
	IsOriginal bool
	// TimestampMs is this frame's position in the source media (video
	// keyframes only; 0 for ordinary document images).
	TimestampMs int64
}

// ParserEngineInfo describes a registered parser engine.
type ParserEngineInfo struct {
	Name              string
	Description       string
	FileTypes         []string
	Available         bool
	UnavailableReason string
}

// --- Internal types used by chunking pipeline ---

type DocParserStorageConfig struct {
	Provider        string
	Region          string
	BucketName      string
	AccessKeyID     string
	SecretAccessKey string
	AppID           string
	PathPrefix      string
	Endpoint        string
}

type DocParserVLMConfig struct {
	ModelName     string
	BaseURL       string
	APIKey        string
	InterfaceType string
}

type ParsedChunk struct {
	Content string
	// ContextHeader is an optional context string (e.g. a Markdown heading
	// breadcrumb) that should be prepended at embedding time but is NOT
	// part of the stored Content. Lets retrieval pipelines see section
	// context without breaking End-Start == len(Content) invariants.
	ContextHeader string
	Seq           int
	Start         int
	End           int
	Images        []ParsedImage
	ChunkID       string // populated by processChunks with the actual DB UUID

	// ParentIndex is set when using parent-child chunking strategy.
	// -1 (or unset/0 for flat chunks) means this is a top-level chunk.
	// >= 0 means this is a child chunk referencing the parent at this index
	// in the ParentChunks slice of ProcessChunksOptions.
	ParentIndex int

	// Metadata, when non-nil, is persisted verbatim as the chunk's Metadata
	// column (e.g. video timeline chunks carry their start/end milliseconds).
	Metadata JSON
}

// EmbeddingContent returns the text that should be sent to the embedding
// model: ContextHeader (if any) prepended to Content. Mirrors
// chunker.Chunk.EmbeddingContent so the choice is consistent across the
// chunker output and the indexing pipeline. Surrounding whitespace on
// Content is trimmed so leading/trailing newlines from boundary slicing
// don't dilute the embedded vector.
func (c ParsedChunk) EmbeddingContent() string {
	body := strings.TrimSpace(c.Content)
	if c.ContextHeader == "" {
		return body
	}
	return c.ContextHeader + "\n\n" + body
}

// ParsedParentChunk represents a parent chunk in the parent-child strategy.
// Parent chunks are stored in DB for context retrieval but NOT vector-indexed.
type ParsedParentChunk struct {
	Content string
	Seq     int
	Start   int
	End     int
}

type ParsedImage struct {
	URL         string
	Caption     string
	OCRText     string
	OriginalURL string
	Start       int
	End         int
}
