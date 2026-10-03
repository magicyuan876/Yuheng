package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode defines the error code type
type ErrorCode int

// System error codes
const (
	// Common error codes (1000-1999)
	ErrBadRequest         ErrorCode = 1000
	ErrUnauthorized       ErrorCode = 1001
	ErrForbidden          ErrorCode = 1002
	ErrNotFound           ErrorCode = 1003
	ErrMethodNotAllowed   ErrorCode = 1004
	ErrConflict           ErrorCode = 1005
	ErrTooManyRequests    ErrorCode = 1006
	ErrInternalServer     ErrorCode = 1007
	ErrServiceUnavailable ErrorCode = 1008
	ErrTimeout            ErrorCode = 1009
	ErrValidation         ErrorCode = 1010

	// Tenant related error codes (2000-2099)
	ErrTenantNotFound      ErrorCode = 2000
	ErrTenantAlreadyExists ErrorCode = 2001
	ErrTenantInactive      ErrorCode = 2002
	ErrTenantNameRequired  ErrorCode = 2003
	ErrTenantInvalidStatus ErrorCode = 2004
	// 2005 was ErrTenantCreationDisabled, the self-service creation policy
	// denial. Self-service creation no longer exists; the number stays
	// retired so an old client's "disabled" branch can never be hit by a
	// different condition.
	// ErrTenantOwnerRequired: a workspace cannot be created without an
	// Owner, because a workspace with no members is unreachable.
	ErrTenantOwnerRequired ErrorCode = 2006
	// ErrTenantHasMembers: a workspace is deleted only once everyone but the
	// caller has left it.
	ErrTenantHasMembers ErrorCode = 2007
	// ErrTenantLastWorkspace: the deployment's last workspace cannot be
	// deleted; a deployment without a workspace has nowhere to put anyone.
	ErrTenantLastWorkspace ErrorCode = 2008

	// VectorStore binding related error codes (2200-2299).
	// Both map to HTTP 400 with a generic message; the typed code lets
	// clients distinguish "wrong UUID / cross-tenant" from "store exists
	// but is currently unavailable" without parsing the message text.
	ErrVectorStoreBindingInvalid ErrorCode = 2200
	ErrVectorStoreUnavailable    ErrorCode = 2201

	// Add more error codes here
)

// AppError defines the application error structure
type AppError struct {
	Code     ErrorCode `json:"code"`
	Message  string    `json:"message"`
	Details  any       `json:"details,omitempty"`
	HTTPCode int       `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	return fmt.Sprintf("error code: %d, error message: %s", e.Code, e.Message)
}

// WithDetails adds error details
func (e *AppError) WithDetails(details any) *AppError {
	e.Details = details
	return e
}

// NewBadRequestError creates a bad request error
func NewBadRequestError(message string) *AppError {
	return &AppError{
		Code:     ErrBadRequest,
		Message:  message,
		HTTPCode: http.StatusBadRequest,
	}
}

// NewUnauthorizedError creates an unauthorized error
func NewUnauthorizedError(message string) *AppError {
	return &AppError{
		Code:     ErrUnauthorized,
		Message:  message,
		HTTPCode: http.StatusUnauthorized,
	}
}

// NewForbiddenError creates a forbidden error
func NewForbiddenError(message string) *AppError {
	return &AppError{
		Code:     ErrForbidden,
		Message:  message,
		HTTPCode: http.StatusForbidden,
	}
}

// NewNotFoundError creates a not found error
func NewNotFoundError(message string) *AppError {
	return &AppError{
		Code:     ErrNotFound,
		Message:  message,
		HTTPCode: http.StatusNotFound,
	}
}

// NewConflictError creates a conflict error
func NewConflictError(message string) *AppError {
	return &AppError{
		Code:     ErrConflict,
		Message:  message,
		HTTPCode: http.StatusConflict,
	}
}

// NewTooManyRequestsError creates a 429 error, used by quota-style
// guards (e.g. per-user self-service tenant creation cap).
func NewTooManyRequestsError(message string) *AppError {
	if message == "" {
		message = "too many requests"
	}
	return &AppError{
		Code:     ErrTooManyRequests,
		Message:  message,
		HTTPCode: http.StatusTooManyRequests,
	}
}

// NewInternalServerError creates an internal server error
func NewInternalServerError(message string) *AppError {
	if message == "" {
		message = "服务器内部错误"
	}
	return &AppError{
		Code:     ErrInternalServer,
		Message:  message,
		HTTPCode: http.StatusInternalServerError,
	}
}

// NewServiceUnavailableError creates a service unavailable (503)
// error. Used for transient failures where the caller can retry.
func NewServiceUnavailableError(message string) *AppError {
	if message == "" {
		message = "服务暂时不可用"
	}
	return &AppError{
		Code:     ErrServiceUnavailable,
		Message:  message,
		HTTPCode: http.StatusServiceUnavailable,
	}
}

// NewValidationError creates a validation error
func NewValidationError(message string) *AppError {
	return &AppError{
		Code:     ErrValidation,
		Message:  message,
		HTTPCode: http.StatusBadRequest,
	}
}

// Tenant related errors
func NewTenantNotFoundError() *AppError {
	return &AppError{
		Code:     ErrTenantNotFound,
		Message:  "空间不存在",
		HTTPCode: http.StatusNotFound,
	}
}

// NewTenantAlreadyExistsError creates a tenant already exists error
func NewTenantAlreadyExistsError() *AppError {
	return &AppError{
		Code:     ErrTenantAlreadyExists,
		Message:  "空间已存在",
		HTTPCode: http.StatusConflict,
	}
}

// NewTenantInactiveError creates a tenant inactive error
func NewTenantInactiveError() *AppError {
	return &AppError{
		Code:     ErrTenantInactive,
		Message:  "空间已停用",
		HTTPCode: http.StatusForbidden,
	}
}

// NewTenantOwnerRequiredError rejects a workspace creation that names no
// Owner. Only a non-human caller (a platform API key) can get here: a human
// caller is the default Owner of the workspace they create.
func NewTenantOwnerRequiredError() *AppError {
	return &AppError{
		Code:     ErrTenantOwnerRequired,
		Message:  "a workspace needs an owner; pass owner_email naming an existing user",
		HTTPCode: http.StatusBadRequest,
	}
}

// NewTenantHasMembersError refuses to delete a workspace that still has
// members other than the caller. The count lets the UI say how many people
// would lose access.
func NewTenantHasMembersError(otherMembers int) *AppError {
	return &AppError{
		Code: ErrTenantHasMembers,
		Message: fmt.Sprintf(
			"the workspace still has %d other member(s); remove them before deleting it", otherMembers),
		HTTPCode: http.StatusConflict,
	}
}

// NewTenantLastWorkspaceError refuses to delete the deployment's only
// remaining workspace.
func NewTenantLastWorkspaceError() *AppError {
	return &AppError{
		Code:     ErrTenantLastWorkspace,
		Message:  "this is the last workspace of the deployment and cannot be deleted",
		HTTPCode: http.StatusConflict,
	}
}

// NewVectorStoreBindingInvalidError signals that a knowledge base create
// request referenced a vector store that does not exist under the caller's
// tenant (or carried a malformed UUID). The user-facing message is
// intentionally generic to avoid enumeration oracles — see the structured
// log at the call site for the tenant/store pair.
func NewVectorStoreBindingInvalidError(message string) *AppError {
	if message == "" {
		message = "vector store not found"
	}
	return &AppError{
		Code:     ErrVectorStoreBindingInvalid,
		Message:  message,
		HTTPCode: http.StatusBadRequest,
	}
}

// NewVectorStoreUnavailableError signals that the requested vector store
// row exists in the database but is not currently wired into the in-memory
// engine registry (factory failure on CreateStore, dynamic config error,
// or stale state after a connection-config rotation).
func NewVectorStoreUnavailableError(message string) *AppError {
	if message == "" {
		message = "vector store is currently unavailable"
	}
	return &AppError{
		Code:     ErrVectorStoreUnavailable,
		Message:  message,
		HTTPCode: http.StatusBadRequest,
	}
}

// IsAppError checks if the error is an AppError type
func IsAppError(err error) (*AppError, bool) {
	appErr, ok := err.(*AppError)
	return appErr, ok
}
