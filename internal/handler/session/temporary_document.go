package session

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

// UploadTemporaryDocument accepts one multipart file and immediately returns
// a session-scoped document ID. Parsing continues in the document worker.
func (h *Handler) UploadTemporaryDocument(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := c.Param("session_id")
	// Uploading attaches content to the session, so use the strict owner scope:
	// a tenant admin may read an API-key session but must not add attachments.
	if _, err := h.sessionService.GetOwnedSession(ctx, sessionID); err != nil {
		c.Error(apperrors.NewNotFoundError("Session not found"))
		return
	}
	maxBytes := secutils.GetMaxFileSizeMB()*1024*1024 + 1024*1024
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.Error(apperrors.NewBadRequestError(fmt.Sprintf("invalid attachment upload: %v", err)))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.Error(apperrors.NewBadRequestError("failed to open attachment"))
		return
	}
	defer file.Close()

	options := types.TemporaryDocumentCreateOptions{ParserEngine: strings.TrimSpace(c.PostForm("parser_engine"))}
	document, err := h.temporaryDocuments.Create(
		ctx, c.GetUint64(types.TenantIDContextKey.String()), sessionID,
		fileHeader.Filename, fileHeader.Header.Get("Content-Type"), fileHeader.Size, file, options,
	)
	if err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": document})
}

func (h *Handler) ListTemporaryDocuments(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := sessionIDParam(c)
	if _, err := h.sessionService.GetSession(ctx, sessionID); err != nil {
		c.Error(apperrors.NewNotFoundError("Session not found"))
		return
	}
	documents, err := h.temporaryDocuments.List(ctx, c.GetUint64(types.TenantIDContextKey.String()), sessionID)
	if err != nil {
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": documents})
}

func (h *Handler) GetTemporaryDocument(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := sessionIDParam(c)
	if _, err := h.sessionService.GetSession(ctx, sessionID); err != nil {
		c.Error(apperrors.NewNotFoundError("Session not found"))
		return
	}
	document, err := h.temporaryDocuments.Get(ctx, c.GetUint64(types.TenantIDContextKey.String()), sessionID, c.Param("attachment_id"))
	if err != nil {
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	if document == nil {
		c.Error(apperrors.NewNotFoundError("Attachment not found"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": document})
}

func (h *Handler) PreviewTemporaryDocument(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := sessionIDParam(c)
	if _, err := h.sessionService.GetSession(ctx, sessionID); err != nil {
		c.Error(apperrors.NewNotFoundError("Session not found"))
		return
	}
	attachmentID := secutils.SanitizeForLog(c.Param("attachment_id"))
	if attachmentID == "" {
		c.Error(apperrors.NewBadRequestError("Attachment ID cannot be empty"))
		return
	}
	file, filename, err := h.temporaryDocuments.OpenFile(
		ctx, c.GetUint64(types.TenantIDContextKey.String()), sessionID, attachmentID,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			c.Error(apperrors.NewNotFoundError("Attachment not found"))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError("Failed to retrieve attachment").WithDetails(err.Error()))
		return
	}
	defer file.Close()

	contentType, inline := secutils.SafeContentTypeByFilename(filename)
	c.Header("Content-Type", contentType)
	c.Header("X-Content-Type-Options", "nosniff")
	disposition := "inline"
	if !inline {
		disposition = "attachment"
	}
	c.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": filename}))
	c.Header("Cache-Control", "private, max-age=3600")

	c.Stream(func(w io.Writer) bool {
		if _, err := io.Copy(w, file); err != nil {
			logger.Errorf(ctx, "Failed to stream attachment preview: %v", err)
			return false
		}
		return false
	})
}

func (h *Handler) DeleteTemporaryDocument(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := sessionIDParam(c)
	// Deleting mutates the session's attachments, so use the strict owner scope:
	// a tenant admin may read an API-key session but must not remove attachments.
	if _, err := h.sessionService.GetOwnedSession(ctx, sessionID); err != nil {
		c.Error(apperrors.NewNotFoundError("Session not found"))
		return
	}
	if err := h.temporaryDocuments.Delete(ctx, c.GetUint64(types.TenantIDContextKey.String()), sessionID, c.Param("attachment_id")); err != nil {
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.Status(http.StatusNoContent)
}

func sessionIDParam(c *gin.Context) string {
	if value := c.Param("session_id"); value != "" {
		return value
	}
	return c.Param("id")
}

func containsFileType(supported []string, ext string) bool {
	for _, item := range supported {
		if strings.TrimPrefix(strings.ToLower(strings.TrimSpace(item)), ".") == ext {
			return true
		}
	}
	return false
}

func isAudioExtension(ext string) bool {
	switch ext {
	case "mp3", "wav", "m4a", "flac", "ogg", "aac":
		return true
	default:
		return false
	}
}
