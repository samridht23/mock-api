package utils

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/samridht23/mock-api/internal/apperror"
)

const MaxBodySize = 10 << 20 // 10MB

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type APIResponse struct {
	Status int         `json:"status"`
	Data   interface{} `json:"data"`
	Error  *APIError   `json:"error"`
}

func WriteResponse(w http.ResponseWriter, status int, data interface{}, apiErr *APIError) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	resp := APIResponse{
		Status: status,
		Data:   data,
		Error:  apiErr,
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func WriteSuccess(w http.ResponseWriter, status int, data interface{}) {
	WriteResponse(w, status, data, nil)
}

func WriteError(w http.ResponseWriter, errDef apperror.ErrorDef) {
	WriteResponse(w, errDef.HTTPStatus, nil, &APIError{
		Code:    errDef.Code,
		Message: errDef.Message,
	})
}

func WriteErrorWithMessage(w http.ResponseWriter, errDef apperror.ErrorDef, message string) {
	WriteResponse(w, errDef.HTTPStatus, nil, &APIError{
		Code:    errDef.Code,
		Message: message,
	})
}

func DecodeJSON(r *http.Request, dst interface{}) *apperror.ErrorDef {
	if r.Body == nil {
		slog.Warn("decode_json: empty body")
		return &apperror.ErrBadRequest
	}

	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		slog.Warn("decode_json: invalid content type", "content_type", contentType)
		return &apperror.ErrInvalidJSON
	}
	// Limit body size
	r.Body = http.MaxBytesReader(nil, r.Body, MaxBodySize)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {

		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			slog.Warn("decode_json: syntax error",
				"offset", syntaxError.Offset,
			)
			return &apperror.ErrInvalidJSON

		case errors.As(err, &unmarshalTypeError):
			slog.Warn("decode_json: type mismatch",
				"field", unmarshalTypeError.Field,
				"expected", unmarshalTypeError.Type,
			)
			return &apperror.ErrBadRequest

		case strings.HasPrefix(err.Error(), "json: unknown field"):
			slog.Warn("decode_json: unknown field",
				"error", err.Error(),
			)
			return &apperror.ErrBadRequest

		case errors.Is(err, io.EOF):
			slog.Warn("decode_json: empty json body")
			return &apperror.ErrBadRequest

		case err.Error() == "http: request body too large":
			slog.Warn("decode_json: body too large")
			return &apperror.ErrBadRequest

		default:
			slog.Error("decode_json: unexpected error", "error", err)
			return &apperror.ErrInternal
		}
	}
	if dec.More() {
		slog.Warn("decode_json: multiple json objects detected")
		return &apperror.ErrBadRequest
	}
	return nil
}
