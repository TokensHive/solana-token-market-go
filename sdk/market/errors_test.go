package market

import (
	"context"
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

func TestClassifyErrorCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want ErrorCode
	}{
		{name: "rate_limited", err: errors.New("HTTP 429 RateLimitExceeded"), want: ErrCodeRateLimited},
		{name: "timeout", err: context.DeadlineExceeded, want: ErrCodeTimeout},
		{name: "rpc_unavailable", err: errors.New("connection refused"), want: ErrCodeRPCUnavailable},
		{name: "unsupported_route", err: errors.New("unsupported pool route"), want: ErrCodeUnsupportedRoute},
		{name: "account_not_found", err: errors.New("account missing"), want: ErrCodeAccountNotFound},
		{name: "decode_error", err: errors.New("decode failed"), want: ErrCodeDecode},
		{name: "rpc_error", err: errors.New("rpc boom"), want: ErrCodeRPC},
		{name: "internal", err: errors.New("plain"), want: ErrCodeInternal},
		{name: "decode_error_alt", err: errors.New("invalid token account"), want: ErrCodeDecode},
		{name: "decode_error_account_data", err: errors.New("invalid account data"), want: ErrCodeDecode},
		{name: "account_not_found_alt", err: errors.New("not found"), want: ErrCodeAccountNotFound},
		{name: "timeout_text", err: errors.New("deadline exceeded"), want: ErrCodeTimeout},
	}
	for _, tc := range cases {
		if got := classifyErrorCode(tc.err); got != tc.want {
			t.Fatalf("%s: expected %s, got %s", tc.name, tc.want, got)
		}
	}
	if got := classifyErrorCode(nil); got != ErrCodeInternal {
		t.Fatalf("expected nil error classification fallback to internal, got %s", got)
	}
}
