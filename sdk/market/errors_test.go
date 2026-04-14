package market

import (
	"errors"
	"strings"
	"testing"
)

func TestSDKError(t *testing.T) {
	e := &SDKError{Code: ErrCodeInvalidArgument, Message: "bad input"}
	if got := e.Error(); !strings.Contains(got, "invalid_argument: bad input") {
		t.Fatalf("unexpected error string: %s", got)
	}
	if e.Unwrap() != nil {
		t.Fatal("expected nil unwrap when inner error is nil")
	}

	inner := errors.New("inner")
	e = &SDKError{Code: ErrCodeInternal, Message: "failed", Err: inner}
	if got := e.Error(); !strings.Contains(got, "internal: failed: inner") {
		t.Fatalf("unexpected wrapped error string: %s", got)
	}
	if !errors.Is(e, inner) {
		t.Fatal("expected unwrap to match inner error")
	}
}

func TestNewError(t *testing.T) {
	err := NewError(ErrCodeRPC, "rpc failed", nil)
	if err == nil {
		t.Fatal("expected sdk error")
	}
	if _, ok := err.(*SDKError); !ok {
		t.Fatalf("expected SDKError type, got %T", err)
	}
}
