// Package repository is the persistence layer of the docs module.
//
// Every method takes the tenant ID explicitly and pins each statement to it;
// a caller can never reach another tenant's rows by guessing an ID. Soft
// deletion is explicit (deleted_at), not GORM's automatic scope, so that
// trash listing and restore can reason about it.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// Sentinel errors. Services translate them into API errors.
var (
	// ErrNotFound: the row does not exist in this tenant (or is soft-deleted
	// where the method excludes the trash).
	ErrNotFound = errors.New("docs: not found")
	// ErrConflict: an optimistic-concurrency check failed; reload and retry.
	ErrConflict = errors.New("docs: version conflict")
	// ErrDuplicate: a uniqueness rule was violated (slug, short id, member).
	ErrDuplicate = errors.New("docs: duplicate")
	// ErrInvalidMove: the requested tree move would create a cycle or cross
	// into a page that cannot be a parent.
	ErrInvalidMove = errors.New("docs: invalid move")
)

// Repositories bundles the module's repositories over one *gorm.DB.
type Repositories struct {
	db        *gorm.DB
	Spaces    SpaceRepository
	Members   SpaceMemberRepository
	Groups    GroupRepository
	Pages     PageRepository
	Access    PageAccessRepository
	Leases    LeaseRepository
	Files     AttachmentRepository
	Links     LinkRepository
	Blocks    TransclusionRepository
	History   RevisionRepository
	Comments  CommentRepository
	Watchers  WatcherRepository
	Notices   NotificationRepository
	Labels    LabelRepository
	Shares    ShareRepository
	Templates TemplateRepository
	Search    SearchRepository
	Exports   ExportJobRepository
	Imports   ImportJobRepository
}

// New wires the repositories.
func New(db *gorm.DB) *Repositories {
	return &Repositories{
		db:        db,
		Spaces:    &spaceRepository{db: db},
		Members:   &spaceMemberRepository{db: db},
		Groups:    &groupRepository{db: db},
		Pages:     &pageRepository{db: db},
		Access:    &pageAccessRepository{db: db},
		Leases:    &leaseRepository{db: db},
		Files:     &attachmentRepository{db: db},
		Links:     &linkRepository{db: db},
		Blocks:    &transclusionRepository{db: db},
		History:   &revisionRepository{db: db},
		Comments:  &commentRepository{db: db},
		Watchers:  &watcherRepository{db: db},
		Notices:   &notificationRepository{db: db},
		Labels:    &labelRepository{db: db},
		Shares:    &shareRepository{db: db},
		Templates: &templateRepository{db: db},
		Search:    &searchRepository{db: db},
		Exports:   &exportJobRepository{db: db},
		Imports:   &importJobRepository{db: db},
	}
}

// Transaction runs fn with repositories bound to one database transaction.
func (r *Repositories) Transaction(ctx context.Context, fn func(tx *Repositories) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(New(tx))
	})
}

// DB exposes the underlying handle for callers that need raw access (tests,
// migrations); services should not use it.
func (r *Repositories) DB() *gorm.DB { return r.db }

// NewID returns a fresh UUID string for primary keys.
func NewID() string { return uuid.NewString() }

// now returns the wall clock truncated to microseconds, the finest precision
// a Postgres timestamp stores, so a value round-trips unchanged.
func now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

// isUniqueViolation recognises a unique-constraint failure (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// isForeignKeyViolation recognises a missing referenced row (SQLSTATE 23503).
// Derived rows such as links are written optimistically; a target that has just
// been deleted means the reference does not exist, not that the save failed.
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func translateWriteError(err error) error {
	if err == nil {
		return nil
	}
	if isUniqueViolation(err) {
		return fmt.Errorf("%w: %v", ErrDuplicate, err)
	}
	return err
}

// mapNotFound converts GORM's sentinel into the module's.
func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
