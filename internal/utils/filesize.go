package utils

import (
	"os"
	"strconv"
	"sync/atomic"
)

// The upload size limits are runtime-tunable system settings
// (file.max_size_mb / file.video_max_size_mb, editable by SystemAdmin in
// the UI) with the MAX_FILE_SIZE_MB / MAX_VIDEO_FILE_SIZE_MB env vars as
// fallback defaults. The dynamic values flow through resolver hooks
// installed at startup (see cmd/server/bootstrap.go) so the many call
// sites of this package stay dependency-free; before the hook is
// installed — or in binaries that never install it (CLI, tests) — the
// env/default path below applies.
//
// Layering contract: these are the BUSINESS limits. The transport layers
// are deploy-time hard caps sized above them and are NOT runtime-tunable:
//   - docreader gRPC max message size (DOCREADER_GRPC_MAX_FILE_SIZE_MB,
//     app client and python server, default max(MAX_FILE_SIZE_MB, 512))
//   - frontend nginx client_max_body_size
//     (max(MAX_FILE_SIZE_MB, MAX_VIDEO_FILE_SIZE_MB), default 2048)
// Raising the UI setting beyond a hard cap surfaces as a transport error
// (413 / gRPC RESOURCE_EXHAUSTED); the setting description warns about it.

// maxFileSizeResolver / maxVideoSizeResolver hold func() int64 values
// returning the current limit in MB, or nil before installation.
var (
	maxFileSizeResolver  atomic.Value
	maxVideoSizeResolver atomic.Value
)

// RegisterMaxFileSizeResolvers installs the runtime resolvers for the
// document and video upload limits (in MB). Passing nil for either leaves
// the env/default behaviour for that limit. Safe for concurrent use.
func RegisterMaxFileSizeResolvers(fileMB, videoMB func() int64) {
	if fileMB != nil {
		maxFileSizeResolver.Store(fileMB)
	}
	if videoMB != nil {
		maxVideoSizeResolver.Store(videoMB)
	}
}

func resolvedMB(v *atomic.Value) (int64, bool) {
	f, ok := v.Load().(func() int64)
	if !ok || f == nil {
		return 0, false
	}
	if n := f(); n > 0 {
		return n, true
	}
	return 0, false
}

func envMB(name string, def int64) int64 {
	if sizeStr := os.Getenv(name); sizeStr != "" {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil && size > 0 {
			return size
		}
	}
	return def
}

// GetMaxFileSize returns the maximum file upload size in bytes.
func GetMaxFileSize() int64 {
	return GetMaxFileSizeMB() * 1024 * 1024
}

// GetMaxFileSizeMB returns the maximum file upload size in MB: the
// file.max_size_mb system setting when the resolver is installed,
// otherwise the MAX_FILE_SIZE_MB env (default 50).
func GetMaxFileSizeMB() int64 {
	if n, ok := resolvedMB(&maxFileSizeResolver); ok {
		return n
	}
	return envMB("MAX_FILE_SIZE_MB", 50)
}

// GetMaxVideoFileSizeMB returns the maximum VIDEO upload size in MB
// (file.video_max_size_mb setting / MAX_VIDEO_FILE_SIZE_MB env, default
// 2048). Videos get their own, much larger limit because they bypass the
// gRPC byte transport: local-storage deployments hand the docreader a
// shared-volume path and ffmpeg reads the file in place, so the ordinary
// document limit (which sizes gRPC message buffers) does not apply.
func GetMaxVideoFileSizeMB() int64 {
	if n, ok := resolvedMB(&maxVideoSizeResolver); ok {
		return n
	}
	return envMB("MAX_VIDEO_FILE_SIZE_MB", 2048)
}
