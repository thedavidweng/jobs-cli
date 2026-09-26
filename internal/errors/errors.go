package errors

import (
	"errors"
	"fmt"
	"time"
)

type Error struct {
	Code         Code     `json:"code"`
	Message      string   `json:"message"`
	Category     Category `json:"category"`
	Retryable    bool     `json:"retryable"`
	RetryAfterMS int64    `json:"retry_after_ms,omitempty"`
	Err          error    `json:"-"`
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func New(code Code, message string, category Category, retryable bool, err error) *Error {
	return &Error{
		Code:      code,
		Message:   message,
		Category:  category,
		Retryable: retryable,
		Err:       err,
	}
}

func NewWithRetryAfter(code Code, message string, category Category, retryable bool, retryAfter time.Duration, err error) *Error {
	return &Error{
		Code:         code,
		Message:      message,
		Category:     category,
		Retryable:    retryable,
		RetryAfterMS: retryAfter.Milliseconds(),
		Err:          err,
	}
}

func From(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return New(InternalError, err.Error(), CatInternal, false, err)
}

func (e *Error) ExitCode() int {
	return ExitCodeFor(e.Code)
}
