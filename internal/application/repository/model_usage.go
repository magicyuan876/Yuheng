package repository

import (
	"gorm.io/gorm"
)

// scopeKnowledgeBasesByModelID filters knowledge_bases rows that reference
// modelID in any model-binding field.
func scopeKnowledgeBasesByModelID(db *gorm.DB, modelID string) *gorm.DB {
	if db.Dialector.Name() == "postgres" {
		return db.Where(
			"embedding_model_id = ? OR summary_model_id = ? OR "+
				"image_processing_config->>'model_id' = ? OR "+
				"vlm_config->>'model_id' = ? OR "+
				"asr_config->>'model_id' = ? OR "+
				"wiki_config->>'synthesis_model_id' = ?",
			modelID, modelID, modelID, modelID, modelID, modelID,
		)
	}
	return db.Where(
		"embedding_model_id = ? OR summary_model_id = ? OR "+
			"json_extract(image_processing_config, '$.model_id') = ? OR "+
			"json_extract(vlm_config, '$.model_id') = ? OR "+
			"json_extract(asr_config, '$.model_id') = ? OR "+
			"json_extract(wiki_config, '$.synthesis_model_id') = ?",
		modelID, modelID, modelID, modelID, modelID, modelID,
	)
}
