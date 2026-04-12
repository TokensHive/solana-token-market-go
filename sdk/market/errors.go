package market

import "fmt"

type ErrorCode string

const (
ErrCodeInvalidArgument ErrorCode = "invalid_argument"
ErrCodeNotFound        ErrorCode = "not_found"
ErrCodeRPC             ErrorCode = "rpc_error"
ErrCodeDecode          ErrorCode = "decode_error"
ErrCodeInternal        ErrorCode = "internal"
)

type SDKError struct {
Code    ErrorCode
Message string
Err     error
}

func (e *SDKError) Error() string {
if e.Err == nil {
return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
}

func (e *SDKError) Unwrap() error { return e.Err }

func NewError(code ErrorCode, message string, err error) error {
return &SDKError{Code: code, Message: message, Err: err}
}
