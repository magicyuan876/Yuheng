package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsKBExportsPath(t *testing.T) {
	const tenantID uint64 = 10008

	assert.True(t, IsKBExportsPath("local://10008/exports/img.jpg", tenantID))
	assert.True(t, IsKBExportsPath("s3://bucket/10008/exports/uuid.png", tenantID))
	assert.True(t, IsKBExportsPath("s3://bucket/yuheng/10008/exports/uuid.png", tenantID))
	// A numeric bucket or prefix segment ahead of the tenant does not confuse it.
	assert.True(t, IsKBExportsPath("s3://2024/7/10008/exports/uuid.png", tenantID))
	assert.True(t, IsKBExportsPath("storage://backend-a/local://10008/exports/img.jpg", tenantID))

	assert.False(t, IsKBExportsPath("local://10008/knowledge-id/123.pdf", tenantID))
	assert.False(t, IsKBExportsPath("local://9999/exports/img.jpg", tenantID))
	assert.False(t, IsKBExportsPath("local://10008/other/img.jpg", tenantID))
	assert.False(t, IsKBExportsPath("storage://backend-a/local://9999/exports/img.jpg", tenantID))
	assert.False(t, IsKBExportsPath("no-scheme/10008/exports/a.png", tenantID))
}
