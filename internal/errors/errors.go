// Package errors provides structured error types with classification codes.
package errors

import (
	"errors"
	"fmt"
)

// Error codes used across the application.
const (
	CodeAuth       = "AUTH_ERROR"
	CodeNotFound   = "NOT_FOUND"
	CodeValidation = "VALIDATION_ERROR"
	CodeProvider   = "PROVIDER_ERROR"
	CodeInternal   = "INTERNAL_ERROR"
	CodeRateLimit  = "RATE_LIMIT"
	CodeConfig     = "CONFIG_ERROR"
)

// Error represents a classified error.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// New creates a new Error without a wrapped cause.
func New(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap wraps an existing error with a code and message.
func Wrap(err error, code, message string) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

// IsCode checks whether err (or any in its chain) carries the given code.
func IsCode(err error, code string) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Code == code
	}
	return false
}
