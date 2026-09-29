package repository

import (
	"strings"
	"testing"

	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types"
)

func TestKnowledgeSourceSchemaAllowsObjectStorageURLs(t *testing.T) {
	db := pgtest.New(t)

	// The column is read from the migrated schema rather than from the
	// struct tag, so a migration that narrows it again is caught here.
	var column struct {
		DataType  string
		MaxLength *int
	}
	if err := db.Raw(`SELECT data_type, character_maximum_length AS max_length
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'knowledges' AND column_name = 'source'`).
		Scan(&column).Error; err != nil {
		t.Fatalf("inspect knowledge schema: %v", err)
	}
	if column.DataType != "character varying" || column.MaxLength == nil || *column.MaxLength != 2048 {
		t.Fatalf("knowledge source column = %s(%v), want character varying(2048)", column.DataType, column.MaxLength)
	}

	longURL := "https://example-bucket.cos.ap-beijing.myqcloud.com/test/" +
		strings.Repeat("encoded-path-segment-", 20) + ".docx"
	knowledge := &types.Knowledge{
		TenantID:        1,
		KnowledgeBaseID: "kb-1",
		Type:            types.KnowledgeTypeManual,
		Title:           "long source",
		Source:          longURL,
		ParseStatus:     types.ParseStatusPending,
		EnableStatus:    "enabled",
	}
	if err := db.Create(knowledge).Error; err != nil {
		t.Fatalf("create knowledge with long source: %v", err)
	}
}
