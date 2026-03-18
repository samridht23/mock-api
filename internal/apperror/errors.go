package apperror

import "net/http"

type ErrorDef struct {
	Code       string
	Message    string
	HTTPStatus int
}

var (
	//  Generic / System

	ErrInternal = ErrorDef{
		Code:       "INTERNAL_ERROR",
		Message:    "Something went wrong",
		HTTPStatus: http.StatusInternalServerError,
	}

	ErrServiceUnavailable = ErrorDef{
		Code:       "SERVICE_UNAVAILABLE",
		Message:    "Service temporarily unavailable",
		HTTPStatus: http.StatusServiceUnavailable,
	}

	ErrTimeout = ErrorDef{
		Code:       "REQUEST_TIMEOUT",
		Message:    "Request timed out",
		HTTPStatus: http.StatusRequestTimeout,
	}

	//  Authentication / Auth

	ErrUnauthorized = ErrorDef{
		Code:       "UNAUTHORIZED",
		Message:    "Authentication required",
		HTTPStatus: http.StatusUnauthorized,
	}

	ErrInvalidToken = ErrorDef{
		Code:       "INVALID_TOKEN",
		Message:    "Invalid or expired token",
		HTTPStatus: http.StatusUnauthorized,
	}

	ErrSessionExpired = ErrorDef{
		Code:       "SESSION_EXPIRED",
		Message:    "Session has expired",
		HTTPStatus: http.StatusUnauthorized,
	}

	//  Authorization

	ErrForbidden = ErrorDef{
		Code:       "FORBIDDEN",
		Message:    "You do not have permission",
		HTTPStatus: http.StatusForbidden,
	}

	ErrOwnershipRequired = ErrorDef{
		Code:       "OWNERSHIP_REQUIRED",
		Message:    "You do not own this resource",
		HTTPStatus: http.StatusForbidden,
	}

	//  Resource

	ErrNotFound = ErrorDef{
		Code:       "NOT_FOUND",
		Message:    "Resource not found",
		HTTPStatus: http.StatusNotFound,
	}

	ErrAlreadyExists = ErrorDef{
		Code:       "ALREADY_EXISTS",
		Message:    "Resource already exists",
		HTTPStatus: http.StatusConflict,
	}

	ErrConflict = ErrorDef{
		Code:       "CONFLICT",
		Message:    "Resource conflict",
		HTTPStatus: http.StatusConflict,
	}

	//  Request / Validation

	ErrBadRequest = ErrorDef{
		Code:       "BAD_REQUEST",
		Message:    "Invalid request",
		HTTPStatus: http.StatusBadRequest,
	}

	ErrInvalidJSON = ErrorDef{
		Code:       "INVALID_JSON",
		Message:    "Malformed JSON body",
		HTTPStatus: http.StatusBadRequest,
	}

	ErrValidation = ErrorDef{
		Code:       "VALIDATION_FAILED",
		Message:    "Request validation failed",
		HTTPStatus: http.StatusUnprocessableEntity,
	}

	ErrMissingField = ErrorDef{
		Code:       "MISSING_REQUIRED_FIELD",
		Message:    "Required field missing",
		HTTPStatus: http.StatusUnprocessableEntity,
	}

	//  Business Logic

	ErrTestEmpty = ErrorDef{
		Code:       "TEST_EMPTY",
		Message:    "Cannot start empty test",
		HTTPStatus: http.StatusBadRequest,
	}

	ErrTestAlreadyStarted = ErrorDef{
		Code:       "TEST_ALREADY_STARTED",
		Message:    "Test already in progress",
		HTTPStatus: http.StatusConflict,
	}

	ErrTestAlreadySubmitted = ErrorDef{
		Code:       "TEST_ALREADY_SUBMITTED",
		Message:    "Test already submitted",
		HTTPStatus: http.StatusConflict,
	}

	ErrLimitExceeded = ErrorDef{
		Code:       "LIMIT_EXCEEDED",
		Message:    "Limit exceeded",
		HTTPStatus: http.StatusBadRequest,
	}

	//  Rate Limiting

	ErrRateLimited = ErrorDef{
		Code:       "RATE_LIMITED",
		Message:    "Too many requests",
		HTTPStatus: http.StatusTooManyRequests,
	}
)
