package market

import (
	"strings"

	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
)

func classifyErrorCode(err error) ErrorCode {
	if err == nil {
		return ErrCodeInternal
	}

	switch rpc.ClassifyError(err) {
	case rpc.ErrorKindRateLimited:
		return ErrCodeRateLimited
	case rpc.ErrorKindTimeout:
		return ErrCodeTimeout
	case rpc.ErrorKindRPCUnavailable:
		return ErrCodeRPCUnavailable
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "unsupported pool route"):
		return ErrCodeUnsupportedRoute
	case strings.Contains(msg, "not found"),
		strings.Contains(msg, "missing account"),
		strings.Contains(msg, "account missing"):
		return ErrCodeAccountNotFound
	case strings.Contains(msg, "decode"),
		strings.Contains(msg, "invalid account data"),
		strings.Contains(msg, "invalid pool"),
		strings.Contains(msg, "invalid mint"),
		strings.Contains(msg, "invalid token"):
		return ErrCodeDecode
	case strings.Contains(msg, "rpc"):
		return ErrCodeRPC
	default:
		return ErrCodeInternal
	}
}
