package search

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrInvalidRequest  = errors.New("invalid request")
	ErrTimeout         = errors.New("timeout")
	ErrExternalService = errors.New("external service error")
	ErrInternal        = errors.New("internal error")
)

type classifiedError struct {
	kind error
	err  error
}

func (e classifiedError) Error() string {
	if e.err == nil {
		return e.kind.Error()
	}
	return e.err.Error()
}

func (e classifiedError) Unwrap() error { return e.err }

func classifyError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "deadline exceeded") || strings.Contains(strings.ToLower(err.Error()), "timeout") {
		return classifiedError{kind: ErrTimeout, err: err}
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "meili") || strings.Contains(msg, "external") {
		return classifiedError{kind: ErrExternalService, err: err}
	}
	return classifiedError{kind: ErrInternal, err: err}
}
