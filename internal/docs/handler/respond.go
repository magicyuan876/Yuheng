// Package handler holds the HTTP layer of the docs module: gin handlers, the
// idempotency middleware and the SSE event stream. Handlers own request
// parsing and response shaping; every decision lives in the service layer.
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/magicyuan876/yuheng/internal/docs/repository"
	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// ok writes Yuheng's standard success envelope.
func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// created writes the success envelope with 201.
func created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": data})
}

// accepted writes the success envelope with 202, for work that was started
// rather than done.
func accepted(c *gin.Context, data any) {
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": data})
}

// noContent ends a request that has nothing to return.
func noContent(c *gin.Context) { c.Status(http.StatusNoContent) }

// fail maps module errors onto Yuheng's AppError types so the shared error
// middleware renders them. Unknown errors are logged and become 500 without
// leaking their text.
func fail(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, repository.ErrNotFound):
		_ = c.Error(apperrors.NewNotFoundError("not found"))
	case errors.Is(err, repository.ErrDuplicate):
		_ = c.Error(apperrors.NewBadRequestError("already exists"))
	case errors.Is(err, repository.ErrConflict):
		_ = c.Error(apperrors.NewConflictError("version conflict; reload and retry"))
	case errors.Is(err, repository.ErrInvalidMove):
		_ = c.Error(apperrors.NewBadRequestError(err.Error()))
	default:
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			_ = c.Error(appErr)
			return
		}
		logger.Errorf(c.Request.Context(), "[docs] %s %s: %v", c.Request.Method, c.FullPath(), err)
		_ = c.Error(apperrors.NewInternalServerError("internal error"))
	}
}

// badRequest is a shorthand for validation failures.
func badRequest(c *gin.Context, msg string) {
	_ = c.Error(apperrors.NewBadRequestError(msg))
}

// NotImplemented is the placeholder for routes declared ahead of their work
// package. It answers 501 so a client can tell "not built yet" from "denied".
func NotImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"success": false, "error": "not implemented yet"})
	c.Abort()
}
