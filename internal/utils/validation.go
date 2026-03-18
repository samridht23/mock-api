package utils

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Global validator instance
var validate *validator.Validate

func init() {
	validate = validator.New()
}

// ValidationErrors represents field-level validation errors
type ValidationErrors map[string]string

// Validate validates a struct and returns structured errors
func Validate(s interface{}) (ValidationErrors, error) {
	err := validate.Struct(s)
	if err == nil {
		return nil, nil
	}

	validationErrors := make(ValidationErrors)

	if validatorErrs, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validatorErrs {
			field := strings.ToLower(e.Field())
			validationErrors[field] = getFieldError(e)
		}
	}

	// Structured logging using slog
	slog.Warn("validation failed",
		slog.Any("errors", validationErrors),
	)

	return validationErrors, err
}

// getFieldError returns user-friendly error message for a field
func getFieldError(e validator.FieldError) string {
	field := strings.ToLower(e.Field())

	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, e.Param())
	case "max":
		return fmt.Sprintf("%s must not exceed %s characters", field, e.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email", field)
	case "uuid", "uuid4":
		return fmt.Sprintf("%s must be a valid UUID", field)
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}
