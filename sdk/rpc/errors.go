package rpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
)

type ErrorKind string

const (
	ErrorKindRateLimited    ErrorKind = "rate_limited"
	ErrorKindTimeout        ErrorKind = "timeout"
	ErrorKindRPCUnavailable ErrorKind = "rpc_unavailable"
	ErrorKindUnknown        ErrorKind = "unknown"
)

type ClientError struct {
	Operation string
	Kind      ErrorKind
	Err       error
}

func (e *ClientError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("%s: %s", e.Operation, e.Kind)
	}
	return fmt.Sprintf("%s: %s: %v", e.Operation, e.Kind, e.Err)
}

func (e *ClientError) Unwrap() error { return e.Err }

func ClassifyError(err error) ErrorKind {
	if err == nil {
		return ErrorKindUnknown
	}

	var ce *ClientError
	if errors.As(err, &ce) {
		return ce.Kind
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorKindTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ErrorKindTimeout
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "429"),
		strings.Contains(msg, "rate limit"),
		strings.Contains(msg, "ratelimit"):
		return ErrorKindRateLimited
	case strings.Contains(msg, "timeout"),
		strings.Contains(msg, "deadline exceeded"):
		return ErrorKindTimeout
	case strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "service unavailable"),
		strings.Contains(msg, "reset by peer"):
		return ErrorKindRPCUnavailable
	default:
		return ErrorKindUnknown
	}
}

func IsRetryableError(err error) bool {
	kind := ClassifyError(err)
	return kind == ErrorKindRateLimited || kind == ErrorKindTimeout || kind == ErrorKindRPCUnavailable
}

func WrapClientError(operation string, err error) error {
	if err == nil {
		return nil
	}
	var ce *ClientError
	if errors.As(err, &ce) {
		return err
	}
	return &ClientError{
		Operation: operation,
		Kind:      ClassifyError(err),
		Err:       err,
	}
}
