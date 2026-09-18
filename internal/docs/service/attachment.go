package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/attachment"
	"github.com/magicyuan876/yuheng/internal/docs/audit"
	"github.com/magicyuan876/yuheng/internal/docs/events"
	"github.com/magicyuan876/yuheng/internal/docs/model"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// AttachmentService owns everything an uploaded file goes through: deciding
// what it really is, keeping the workspace inside its storage quota, writing
// the bytes to the space's storage backend, and handing them back only to
// somebody who may read the page they belong to.
//
// The URL of an attachment is not a capability. Every read re-resolves the
// caller's permission on the owning page, so a link pasted into a chat is
// useless to anybody who could not already open the page.
type AttachmentService struct {
	*base
	// variants caches rendered image sizes in memory. They are derived data:
	// losing them costs one re-render, never a byte of anybody's content.
	variants *variantCache
}

// DefaultMaxAttachmentBytes caps one upload when nothing is configured.
const DefaultMaxAttachmentBytes int64 = 200 << 20

// MaxBufferedBytes bounds what is read into memory to be inspected or
// rewritten. Anything larger is streamed straight to storage and keeps only
// its declared size and digest; it is never an image or an SVG at that size.
const MaxBufferedBytes = 32 << 20

// UploadInput is one multipart upload.
type UploadInput struct {
	// File is the uploaded part, as the HTTP layer parsed it.
	File *multipart.FileHeader
	// PageID optionally binds the attachment to a page immediately. Left
	// empty, the attachment is unbound until a page that references it is
	// saved, which is how paste-then-save works.
	PageID string
}

// AttachmentView is what a client is told about a stored file.
type AttachmentView struct {
	ID       string          `json:"id"`
	SpaceID  string          `json:"space_id"`
	PageID   string          `json:"page_id,omitempty"`
	FileName string          `json:"file_name"`
	Mime     string          `json:"mime"`
	Size     int64           `json:"size_bytes"`
	Kind     attachment.Kind `json:"kind"`
	Width    *int            `json:"width,omitempty"`
	Height   *int            `json:"height,omitempty"`
	URL      string          `json:"url"`
	Uploader UserView        `json:"uploader,omitzero"`
	Created  time.Time       `json:"created_at"`
	// Variants lists the widths this image can also be served at, so the
	// editor can build a srcset without guessing.
	Variants []int `json:"variants,omitempty"`
}

// ServeResult is one read of an attachment's bytes.
type ServeResult struct {
	// Redirect is a short-lived URL on the storage backend. When it is set,
	// nothing else is: the caller sends a redirect and the bytes never pass
	// through this server.
	Redirect string
	// Body streams the file; the caller must close it. Nil when Data is set.
	Body io.ReadCloser
	// Data holds a rendering small enough to have been built in memory.
	Data []byte

	FileName    string
	ContentType string
	Inline      bool
	Sandbox     bool
	Size        int64
}

// Close releases the body, if there is one.
func (s *ServeResult) Close() error {
	if s != nil && s.Body != nil {
		return s.Body.Close()
	}
	return nil
}

// ---- upload ---------------------------------------------------------------

// Upload stores one file for a space.
//
// The order matters and is the same every time: establish permission, decide
// what the bytes are, check the quota, write the object, then record the row.
// A failure at any step leaves nothing behind but, at worst, an unreferenced
// object the maintenance sweep collects.
func (s *AttachmentService) Upload(ctx context.Context, actor *acl.Identity, space *model.Space,
	role model.SpaceRole, in UploadInput,
) (*AttachmentView, error) {
	if in.File == nil {
		return nil, invalid("a file is required")
	}
	if !role.AtLeast(model.RoleWriter) {
		return nil, forbidden("uploading needs the writer role in this space")
	}
	limit := s.d.MaxAttachmentBytes
	if limit <= 0 {
		limit = DefaultMaxAttachmentBytes
	}
	if in.File.Size > limit {
		return nil, invalid("the file is %d bytes, over the %d byte limit", in.File.Size, limit)
	}
	if in.File.Size <= 0 {
		return nil, invalid("the file is empty")
	}

	// A page is bound only if the caller can write that page too; otherwise
	// an upload would be a way to attach a file to somebody else's page.
	var page *model.Page
	if in.PageID != "" {
		d, err := s.d.Resolver.Page(ctx, actor, in.PageID)
		if err != nil {
			return nil, err
		}
		if err := requireRole(d, model.RoleWriter); err != nil {
			return nil, err
		}
		if d.Page.SpaceID != space.ID {
			return nil, invalid("the page belongs to another space")
		}
		page = d.Page
	}

	inspected, err := inspect(in.File)
	if err != nil {
		return nil, err
	}

	tenant, fileSvc, err := s.storageFor(ctx, space)
	if err != nil {
		return nil, err
	}

	// A duplicate of something already in this space costs no new storage and
	// so is not charged against the quota either.
	existing, err := s.d.Repos.Files.FindByDigest(ctx, space.TenantID, space.ID, inspected.digest)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	fresh := existing == nil
	if fresh {
		if err := s.checkQuota(tenant, inspected.size); err != nil {
			return nil, err
		}
	}

	filePath := ""
	if fresh {
		filePath, err = s.store(ctx, fileSvc, space, inspected)
		if err != nil {
			return nil, err
		}
	} else {
		filePath = existing.FilePath
	}

	row := &model.Attachment{
		TenantID:  space.TenantID,
		SpaceID:   space.ID,
		FilePath:  filePath,
		FileName:  inspected.name,
		FileExt:   strings.TrimPrefix(inspected.sniff.Ext, "."),
		Mime:      inspected.sniff.Media,
		SizeBytes: inspected.size,
		Kind:      model.AttachmentKind(inspected.sniff.Kind),
	}
	if inspected.digest != "" {
		digest := inspected.digest
		row.SHA256 = &digest
	}
	if inspected.width > 0 && inspected.height > 0 {
		w, h := inspected.width, inspected.height
		row.Width, row.Height = &w, &h
	}
	if id := actorID(actor); id != "" {
		row.UploaderID = &id
	}
	if page != nil {
		row.PageID = &page.ID
	}
	if err := s.d.Repos.Files.Create(ctx, row); err != nil {
		if fresh {
			s.releaseObject(ctx, fileSvc, filePath)
		}
		return nil, err
	}

	if fresh {
		s.chargeStorage(ctx, space.TenantID, inspected.size)
	}
	s.audit(ctx, audit.Entry{
		TenantID: space.TenantID, ActorUserID: actorID(actor), ActorRole: actorRole(actor),
		Action: audit.AttachmentUploaded, SpaceID: space.ID,
		TargetType: audit.TargetAttachment, TargetID: row.ID,
	})
	if page != nil {
		s.publish(ctx, events.New(events.AttachmentAdded, space.TenantID).
			WithSpace(space.ID).WithPage(page.ID).WithActor(actorID(actor)).With("attachment_id", row.ID))
	}
	return s.view(ctx, row), nil
}

// inspected is everything reading the upload once told us about it.
type inspected struct {
	name   string
	size   int64
	digest string
	sniff  attachment.Result
	width  int
	height int
	// body holds the bytes when they were small enough to buffer, or the
	// sanitised replacement for an SVG. Nil means "stream from the upload".
	body []byte
	// header is the upload itself, used only for the streaming path.
	header *multipart.FileHeader
}

// inspect reads the upload to classify it, hash it and, where the type calls
// for it, rewrite it. Large files are hashed in a single streaming pass and
// never held in memory.
func inspect(fh *multipart.FileHeader) (*inspected, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, invalid("the upload could not be read: %v", err)
	}
	defer func() { _ = f.Close() }()

	out := &inspected{
		name:   attachment.CleanFileName(fh.Filename, "file"),
		size:   fh.Size,
		header: fh,
	}

	buffered := fh.Size <= MaxBufferedBytes
	hasher := sha256.New()
	if buffered {
		data, err := io.ReadAll(io.LimitReader(f, MaxBufferedBytes+1))
		if err != nil {
			return nil, invalid("the upload could not be read: %v", err)
		}
		out.size = int64(len(data))
		hasher.Write(data)
		out.sniff = attachment.Sniff(head(data), fh.Filename)
		out.body = data
	} else {
		head := make([]byte, attachment.SniffLimit)
		n, err := io.ReadFull(f, head)
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
			return nil, invalid("the upload could not be read: %v", err)
		}
		head = head[:n]
		out.sniff = attachment.Sniff(head, fh.Filename)
		hasher.Write(head)
		if _, err := io.Copy(hasher, f); err != nil {
			return nil, invalid("the upload could not be read: %v", err)
		}
	}
	out.digest = hex.EncodeToString(hasher.Sum(nil))

	if out.sniff.Media == attachment.MediaSVG {
		if !buffered {
			return nil, invalid("the SVG is larger than %d bytes", attachment.MaxSVGBytes)
		}
		// The stored bytes are the sanitised ones, so nothing downstream has
		// to remember that this file needed treatment.
		clean, err := attachment.SanitizeSVG(out.body)
		if err != nil {
			return nil, invalid("the SVG could not be sanitised: %v", err)
		}
		out.body = clean
		out.size = int64(len(clean))
		sum := sha256.Sum256(clean)
		out.digest = hex.EncodeToString(sum[:])
	}
	if out.sniff.Kind == attachment.KindImage && len(out.body) > 0 {
		if w, h, ok := attachment.Dimensions(out.body); ok {
			out.width, out.height = w, h
		}
	}
	return out, nil
}

func head(data []byte) []byte {
	if len(data) > attachment.SniffLimit {
		return data[:attachment.SniffLimit]
	}
	return data
}

// store writes the bytes and returns the provider path.
func (s *AttachmentService) store(ctx context.Context, fileSvc interfaces.FileService,
	space *model.Space, in *inspected,
) (string, error) {
	// The stored name carries the extension the bytes justify, not the one
	// the uploader supplied, so a path can never end in something executable
	// that the content is not.
	stored := strings.TrimSuffix(in.name, filepath.Ext(in.name)) + in.sniff.Ext
	if in.body != nil {
		return fileSvc.SaveBytes(ctx, in.body, space.TenantID, stored, false)
	}
	return fileSvc.SaveFile(ctx, in.header, space.TenantID, "docs-"+space.ID)
}

// ---- read -----------------------------------------------------------------

// Fetch resolves an attachment for reading. width, when one of the offered
// image widths, asks for a smaller rendering.
func (s *AttachmentService) Fetch(ctx context.Context, actor *acl.Identity, id string,
	width int,
) (*ServeResult, error) {
	row, err := s.d.Repos.Files.Get(ctx, actor.TenantID, id)
	if err != nil {
		return nil, err
	}
	if err := s.canRead(ctx, actor, row); err != nil {
		return nil, err
	}

	_, fileSvc, err := s.storageForSpaceID(ctx, actor.TenantID, row.SpaceID)
	if err != nil {
		return nil, err
	}
	serving := attachment.ServingFor(row.Mime)

	if width > 0 {
		data, media, err := s.variant(ctx, fileSvc, row, width)
		switch {
		case err == nil:
			return &ServeResult{
				Data: data, FileName: row.FileName, ContentType: media,
				Inline: true, Sandbox: false, Size: int64(len(data)),
			}, nil
		case errors.Is(err, attachment.ErrNoVariant):
			// The original is the smallest useful rendering; fall through.
		default:
			logger.Warnf(ctx, "[docs] rendering attachment %s at %dpx failed: %v", row.ID, width, err)
		}
	}

	// A redirect is only ever offered for a type this server would display
	// anyway. Everything else is proxied so the download headers that keep it
	// from being treated as a document are actually applied.
	if serving.Inline && !serving.Sandbox {
		if url, err := fileSvc.GetFileURL(ctx, row.FilePath); err == nil && url != "" {
			return &ServeResult{
				Redirect: url, FileName: row.FileName, ContentType: serving.ContentType,
				Inline: true, Size: row.SizeBytes,
			}, nil
		}
	}
	body, err := fileSvc.GetFile(ctx, row.FilePath)
	if err != nil {
		return nil, fmt.Errorf("docs: reading attachment %s: %w", row.ID, err)
	}
	return &ServeResult{
		Body: body, FileName: row.FileName, ContentType: serving.ContentType,
		Inline: serving.Inline, Sandbox: serving.Sandbox, Size: row.SizeBytes,
	}, nil
}

// canRead checks the caller against the page the attachment belongs to, or
// against the space when it is not bound to one yet.
func (s *AttachmentService) canRead(ctx context.Context, actor *acl.Identity, row *model.Attachment) error {
	if row.PageID != nil && *row.PageID != "" {
		d, err := s.d.Resolver.Page(ctx, actor, *row.PageID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return notFound("attachment")
			}
			return err
		}
		if d.Role == model.RoleNone {
			return notFound("attachment")
		}
		return nil
	}
	space, err := s.d.Repos.Spaces.Get(ctx, actor.TenantID, row.SpaceID)
	if err != nil {
		return notFound("attachment")
	}
	role, err := s.d.Resolver.SpaceRole(ctx, actor, space)
	if err != nil {
		return err
	}
	if role == model.RoleNone {
		return notFound("attachment")
	}
	return nil
}

// Delete removes an attachment. The row goes immediately; the object goes only
// when no other row still points at it, which is what makes deduplication safe.
func (s *AttachmentService) Delete(ctx context.Context, actor *acl.Identity, id string) error {
	row, err := s.d.Repos.Files.Get(ctx, actor.TenantID, id)
	if err != nil {
		return err
	}
	if err := s.canWriteAttachment(ctx, actor, row); err != nil {
		return err
	}
	if err := s.d.Repos.Files.SoftDelete(ctx, actor.TenantID, id); err != nil {
		return err
	}
	s.releaseIfUnreferenced(ctx, row)
	s.audit(ctx, audit.Entry{
		TenantID: actor.TenantID, ActorUserID: actorID(actor), ActorRole: actorRole(actor),
		Action: audit.AttachmentDeleted, SpaceID: row.SpaceID,
		TargetType: audit.TargetAttachment, TargetID: row.ID,
	})
	return nil
}

func (s *AttachmentService) canWriteAttachment(ctx context.Context, actor *acl.Identity,
	row *model.Attachment,
) error {
	if row.PageID != nil && *row.PageID != "" {
		d, err := s.d.Resolver.Page(ctx, actor, *row.PageID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return notFound("attachment")
			}
			return err
		}
		return requireRole(d, model.RoleWriter)
	}
	space, err := s.d.Repos.Spaces.Get(ctx, actor.TenantID, row.SpaceID)
	if err != nil {
		return notFound("attachment")
	}
	role, err := s.d.Resolver.SpaceRole(ctx, actor, space)
	if err != nil {
		return err
	}
	if role == model.RoleNone {
		return notFound("attachment")
	}
	if !role.AtLeast(model.RoleWriter) {
		return forbidden("deleting an attachment needs the writer role")
	}
	return nil
}

// ListForPage returns a page's attachments.
func (s *AttachmentService) ListForPage(ctx context.Context, d acl.Decision) ([]*AttachmentView, error) {
	rows, err := s.d.Repos.Files.ListByPage(ctx, d.Page.TenantID, d.Page.ID)
	if err != nil {
		return nil, err
	}
	out := make([]*AttachmentView, 0, len(rows))
	for _, row := range rows {
		out = append(out, s.view(ctx, row))
	}
	return out, nil
}

// ---- storage and quota ----------------------------------------------------

func (s *AttachmentService) storageFor(ctx context.Context, space *model.Space) (*types.Tenant,
	interfaces.FileService, error,
) {
	if s.d.Storage == nil {
		return nil, nil, fmt.Errorf("docs: no storage backend resolver is configured")
	}
	tenant := &types.Tenant{ID: space.TenantID}
	if s.d.Tenants != nil {
		loaded, err := s.d.Tenants.GetTenantByID(ctx, space.TenantID)
		if err != nil {
			return nil, nil, err
		}
		if loaded != nil {
			tenant = loaded
		}
	}
	backendID := ""
	if space.StorageBackendID != nil {
		backendID = *space.StorageBackendID
	}
	fileSvc, _, err := s.d.Storage.ResolveFileService(ctx, tenant, backendID, "", "")
	if err != nil {
		return nil, nil, err
	}
	if fileSvc == nil {
		return nil, nil, fmt.Errorf("docs: no file storage is configured")
	}
	return tenant, fileSvc, nil
}

func (s *AttachmentService) storageForSpaceID(ctx context.Context, tenantID uint64,
	spaceID string,
) (*types.Tenant, interfaces.FileService, error) {
	space, err := s.d.Repos.Spaces.Get(ctx, tenantID, spaceID)
	if err != nil {
		return nil, nil, err
	}
	return s.storageFor(ctx, space)
}

// checkQuota refuses an upload that would take the workspace over its limit.
// A zero quota means unlimited, as everywhere else in Yuheng.
func (s *AttachmentService) checkQuota(tenant *types.Tenant, size int64) error {
	if tenant == nil || tenant.StorageQuota <= 0 {
		return nil
	}
	if tenant.StorageUsed+size > tenant.StorageQuota {
		return conflict("the workspace storage quota of %d bytes would be exceeded", tenant.StorageQuota)
	}
	return nil
}

// chargeStorage records new bytes against the workspace. A failure is logged
// rather than surfaced: the file is already stored, and refusing the response
// now would leave the caller believing an upload failed that did not.
func (s *AttachmentService) chargeStorage(ctx context.Context, tenantID uint64, delta int64) {
	if s.d.Tenants == nil || delta == 0 {
		return
	}
	if err := s.d.Tenants.AdjustStorageUsed(ctx, tenantID, delta); err != nil {
		logger.Errorf(ctx, "[docs] adjusting storage used for tenant %d by %d failed: %v", tenantID, delta, err)
	}
}

// releaseIfUnreferenced deletes the stored object once the last row pointing
// at it is gone, and gives the workspace its bytes back.
func (s *AttachmentService) releaseIfUnreferenced(ctx context.Context, row *model.Attachment) {
	n, err := s.d.Repos.Files.CountByPath(ctx, row.TenantID, row.FilePath)
	if err != nil {
		logger.Warnf(ctx, "[docs] counting references to %s failed: %v", row.FilePath, err)
		return
	}
	if n > 0 {
		return
	}
	_, fileSvc, err := s.storageForSpaceID(ctx, row.TenantID, row.SpaceID)
	if err != nil {
		logger.Warnf(ctx, "[docs] resolving storage to release %s failed: %v", row.FilePath, err)
		return
	}
	s.releaseObject(ctx, fileSvc, row.FilePath)
	s.chargeStorage(ctx, row.TenantID, -row.SizeBytes)
	s.variants.dropPrefix(row.ID)
}

func (s *AttachmentService) releaseObject(ctx context.Context, fileSvc interfaces.FileService, path string) {
	if fileSvc == nil || path == "" {
		return
	}
	if err := fileSvc.DeleteFile(ctx, path); err != nil {
		logger.Warnf(ctx, "[docs] deleting stored object %s failed: %v", path, err)
	}
}

// ---- image variants -------------------------------------------------------

// variant renders and caches a smaller version of an image.
func (s *AttachmentService) variant(ctx context.Context, fileSvc interfaces.FileService,
	row *model.Attachment, width int,
) ([]byte, string, error) {
	if row.Kind != model.AttachmentKind(attachment.KindImage) || !attachment.IsVariantWidth(width) {
		return nil, "", attachment.ErrNoVariant
	}
	if row.Width != nil && *row.Width > 0 && *row.Width <= width {
		return nil, "", attachment.ErrNoVariant
	}
	key := fmt.Sprintf("%s@%d", row.ID, width)
	if data, media, ok := s.variants.get(key); ok {
		return data, media, nil
	}
	if row.SizeBytes > MaxBufferedBytes {
		return nil, "", attachment.ErrNoVariant
	}
	body, err := fileSvc.GetFile(ctx, row.FilePath)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = body.Close() }()
	source, err := io.ReadAll(io.LimitReader(body, MaxBufferedBytes+1))
	if err != nil {
		return nil, "", err
	}
	data, media, err := attachment.Variant(source, width)
	if err != nil {
		return nil, "", err
	}
	s.variants.put(key, data, media)
	return data, media, nil
}

// variantCache is a bounded in-memory store of rendered image sizes.
//
// It is deliberately not a persistent cache in the storage backend: a
// rendering is cheap to redo, and keeping derived objects alongside the
// originals would mean a second lifecycle to get right (invalidate on delete,
// count against the quota, clean up after a failed upload). Losing this cache
// on restart costs one re-render per image and nothing else.
type variantCache struct {
	mu      sync.Mutex
	entries map[string]variantEntry
	order   []string
	bytes   int
	limit   int
}

type variantEntry struct {
	data  []byte
	media string
}

func newVariantCache(limit int) *variantCache {
	if limit <= 0 {
		limit = 64 << 20
	}
	return &variantCache{entries: map[string]variantEntry{}, limit: limit}
}

func (c *variantCache) get(key string) ([]byte, string, bool) {
	if c == nil {
		return nil, "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	return e.data, e.media, ok
}

func (c *variantCache) put(key string, data []byte, media string) {
	if c == nil || len(data) > c.limit {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; exists {
		return
	}
	c.entries[key] = variantEntry{data: data, media: media}
	c.order = append(c.order, key)
	c.bytes += len(data)
	for c.bytes > c.limit && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		c.bytes -= len(c.entries[oldest].data)
		delete(c.entries, oldest)
	}
}

// dropPrefix forgets every rendering of one attachment.
func (c *variantCache) dropPrefix(attachmentID string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	kept := c.order[:0]
	for _, key := range c.order {
		if strings.HasPrefix(key, attachmentID+"@") {
			c.bytes -= len(c.entries[key].data)
			delete(c.entries, key)
			continue
		}
		kept = append(kept, key)
	}
	c.order = kept
}

// ---- page binding ---------------------------------------------------------

// bindAttachments claims the attachments a saved document references and
// recomputes the page's attachment total.
//
// It runs inside the one persist path both editing transports share, so an
// image pasted into a page becomes that page's attachment the moment the page
// is saved — whoever saved it and however they were connected. An attachment
// nobody references stays unbound and is collected later, which is what makes
// an abandoned paste self-cleaning.
func (b *base) bindAttachments(ctx context.Context, page *model.Page, ids []string) {
	if b.d.Repos.Files == nil {
		return
	}
	if len(ids) > 0 {
		if _, err := b.d.Repos.Files.BindToPage(ctx, page.TenantID, page.SpaceID, page.ID, ids); err != nil {
			logger.Warnf(ctx, "[docs] binding attachments to page %s failed: %v", page.ID, err)
			return
		}
	}
	totals, err := b.d.Repos.Files.SumBytesByPage(ctx, page.TenantID, []string{page.ID})
	if err != nil {
		logger.Warnf(ctx, "[docs] totalling attachments of page %s failed: %v", page.ID, err)
		return
	}
	total := totals[page.ID]
	if total == page.AttachmentBytes {
		return
	}
	if err := b.d.Repos.Pages.UpdateMeta(ctx, page.TenantID, page.ID,
		map[string]any{"attachment_bytes": total}); err != nil {
		logger.Warnf(ctx, "[docs] recording the attachment total of page %s failed: %v", page.ID, err)
	}
}

// collectPageAttachments lists every attachment bound to the given pages,
// deleted ones included. It is read-only and is called before a purge, while
// the page rows still exist: the foreign key nulls page_id the moment a page
// is hard-deleted, so afterwards there would be nothing left to look up.
func (b *base) collectPageAttachments(ctx context.Context, tenantID uint64,
	pageIDs []string,
) []*model.Attachment {
	if b.d.Repos.Files == nil || len(pageIDs) == 0 {
		return nil
	}
	rows, err := b.d.Repos.Files.ListForPages(ctx, tenantID, pageIDs)
	if err != nil {
		logger.Warnf(ctx, "[docs] listing attachments of pages about to be purged failed: %v", err)
		return nil
	}
	return rows
}

// releaseAttachments deletes attachment rows and, for each stored object no
// other row still points at, the object itself and its charge on the quota.
//
// Deleting a page only puts it in the trash, and its attachments stay with it
// so a restore brings back a complete page. It is the purge — the point of no
// return — that releases the bytes.
func (b *base) releaseAttachments(ctx context.Context, tenantID uint64, rows []*model.Attachment) {
	if b.d.Repos.Files == nil || len(rows) == 0 {
		return
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	if err := b.d.Repos.Files.DeleteRows(ctx, tenantID, ids); err != nil {
		logger.Warnf(ctx, "[docs] deleting attachment rows of purged pages failed: %v", err)
		return
	}
	svc := &AttachmentService{base: b}
	for _, row := range rows {
		if row.DeletedAt != nil {
			// Its object was already released when it was deleted on its own.
			continue
		}
		svc.releaseIfUnreferenced(ctx, row)
	}
}

// ---- views ----------------------------------------------------------------

func (s *AttachmentService) view(ctx context.Context, row *model.Attachment) *AttachmentView {
	out := &AttachmentView{
		ID: row.ID, SpaceID: row.SpaceID, FileName: row.FileName, Mime: row.Mime,
		Size: row.SizeBytes, Kind: attachment.Kind(row.Kind), Width: row.Width, Height: row.Height,
		URL: AttachmentURL(row.ID), Created: row.CreatedAt,
	}
	if row.PageID != nil {
		out.PageID = *row.PageID
	}
	if row.UploaderID != nil {
		out.Uploader = userView(*row.UploaderID, s.users(ctx, []string{*row.UploaderID}))
	}
	if row.Kind == model.AttachmentKind(attachment.KindImage) {
		for _, w := range attachment.VariantWidths {
			if row.Width == nil || *row.Width > w {
				out.Variants = append(out.Variants, w)
			}
		}
	}
	return out
}

// AttachmentURL is the address the editor and the renderer both use.
func AttachmentURL(id string) string { return "/api/v1/docs/attachments/" + id }
