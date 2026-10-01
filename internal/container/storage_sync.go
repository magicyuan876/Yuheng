package container

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/storageallowlist"
	"github.com/magicyuan876/yuheng/internal/types"
	"gorm.io/gorm"
)

// syncEnvStorageBackend makes the deployment storage backend row (id
// types.EnvStorageBackendID) say what the environment says: STORAGE_TYPE and
// its location variables. It runs on every start.
//
// The row stores the location only; S3 credentials stay in the environment
// and are read whenever a driver is built, so rotating them is a restart and
// never a database write.
//
// Starting is refused when the environment cannot describe usable storage
// (an unsupported STORAGE_TYPE, one STORAGE_ALLOW_LIST excludes, an incomplete
// S3 configuration), and when the environment now names a different location
// than the stored row while files are still stored there: every resource on
// the row resolves through it, so silently repointing it would make all of
// them unreadable. Moving the deployment's storage needs a data migration,
// not an edited variable.
func syncEnvStorageBackend(ctx context.Context, db *gorm.DB) error {
	desired, err := types.EnvStorageBackend()
	if err != nil {
		return fmt.Errorf("deployment storage: %w", err)
	}
	if !storageallowlist.IsAllowed(desired.Provider) {
		return fmt.Errorf("deployment storage: STORAGE_TYPE=%s is not allowed by STORAGE_ALLOW_LIST (%v)",
			desired.Provider, storageallowlist.AllowedList())
	}
	// Only the location is persisted.
	desired.Config.AccessKeyID, desired.Config.SecretAccessKey = "", ""

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current types.StorageBackend
		err := tx.Where("id = ?", types.EnvStorageBackendID).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// The migration that introduces the row inserts a placeholder;
			// a database migrated out of band may still lack it.
			desired.CreatedAt, desired.UpdatedAt = time.Now(), time.Now()
			if err := tx.Create(desired).Error; err != nil {
				return fmt.Errorf("deployment storage: create the backend row: %w", err)
			}
			logger.Infof(ctx, "[storage] deployment storage backend created: %s", describeLocation(desired))
			return nil
		}
		if err != nil {
			return fmt.Errorf("deployment storage: load the backend row: %w", err)
		}

		before, after := current.Config.LocationKey(current.Provider), desired.Config.LocationKey(desired.Provider)
		if before == after && current.Name == desired.Name && current.Status == desired.Status &&
			current.IsBuiltin && current.Config == desired.Config {
			return nil
		}
		if before != after {
			var stored int64
			if err := tx.Model(&types.StoredResource{}).
				Where("storage_backend_id = ? AND state = ?", types.EnvStorageBackendID, types.ResourceStateActive).
				Count(&stored).Error; err != nil {
				return fmt.Errorf("deployment storage: count stored files: %w", err)
			}
			if stored > 0 {
				return fmt.Errorf(
					"deployment storage: the environment now points at %s, but %d stored file(s) live at %s; "+
						"restore the previous STORAGE_TYPE / location variables, "+
						"or migrate the files before changing them",
					describeLocation(desired), stored, describeLocation(&current))
			}
		}
		if err := tx.Model(&types.StorageBackend{}).Where("id = ?", types.EnvStorageBackendID).
			Updates(map[string]any{
				"name": desired.Name, "provider": desired.Provider, "config": desired.Config,
				"status": desired.Status, "is_builtin": true, "updated_at": time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("deployment storage: update the backend row: %w", err)
		}
		logger.Infof(ctx, "[storage] deployment storage backend synced from the environment: %s",
			describeLocation(desired))
		return nil
	})
}

// describeLocation names a backend's location for an operator, without
// credentials (LocationKey never includes them).
func describeLocation(b *types.StorageBackend) string {
	return b.Config.LocationKey(b.Provider)
}
