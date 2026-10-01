package utils

import (
	"strconv"
	"strings"
)

// kbScopedExportsSegment is the only storage namespace served by the KB-scoped
// file proxy. Embedded wiki/chunk images land under {tenant}/exports/; raw
// knowledge uploads use {tenant}/{knowledgeID}/... and are served via
// /knowledge/{id}/download instead.
const kbScopedExportsSegment = "exports"

// storageBackendScheme wraps a driver path with the backend id:
// storage://<backendID>/<provider>://...  It is duplicated here (rather than
// reusing types.ParseStorageBackendPath) because internal/types already imports
// internal/utils, so a reverse import would create a cycle.
const storageBackendScheme = "storage://"

// IsKBExportsPath reports whether a driver-native object locator lies in
// tenantID's exports namespace: local://{tenant}/exports/... or
// s3://{bucket}/{prefix}/{tenant}/exports/.... The KB-scoped proxy serves a
// borrower only objects for which this holds, so sharing a knowledge base
// shares its rendered images and never the owner's raw uploads.
//
// The tenant itself is established by the resource row; this checks only
// where in the owner's key space the object sits.
func IsKBExportsPath(physicalPath string, tenantID uint64) bool {
	physicalPath = unwrapStorageBackendPath(physicalPath)
	_, rest, ok := strings.Cut(physicalPath, "://")
	if !ok {
		return false
	}
	tenantSeg := strconv.FormatUint(tenantID, 10)
	parts := strings.Split(rest, "/")
	for i, part := range parts {
		if part == tenantSeg && i+1 < len(parts) && parts[i+1] == kbScopedExportsSegment {
			return true
		}
	}
	return false
}

// unwrapStorageBackendPath strips a leading storage://<backendID>/ wrapper and
// returns the inner driver path. Non-wrapped paths are returned unchanged.
func unwrapStorageBackendPath(filePath string) string {
	rest, ok := strings.CutPrefix(filePath, storageBackendScheme)
	if !ok {
		return filePath
	}
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return filePath
	}
	return parts[1]
}
