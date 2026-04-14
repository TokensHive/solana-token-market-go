package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/TokensHive/solana-token-market-go/sdk/internal/reqdebug"
	"github.com/gagliardetto/solana-go"
	rpcclient "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

type AccountInfo struct {
	Address  solana.PublicKey
	Owner    solana.PublicKey
	Lamports uint64
	Data     []byte
	Slot     uint64
	Exists   bool
}

type Client interface {
	GetAccount(ctx context.Context, address solana.PublicKey) (*AccountInfo, error)
	GetMultipleAccounts(ctx context.Context, addresses []solana.PublicKey) ([]*AccountInfo, error)
	GetTokenSupply(ctx context.Context, mint solana.PublicKey) (decimal.Decimal, uint8, error)
	GetSignaturesForAddress(ctx context.Context, address solana.PublicKey, opts *SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error)
	GetTransaction(ctx context.Context, signature solana.Signature) (*rpcclient.GetTransactionResult, error)
	GetTransactionRaw(ctx context.Context, signature solana.Signature) ([]byte, error)
}

type SignaturesForAddressOptions struct {
	Limit  int
	Before *solana.Signature
	Until  *solana.Signature
}

type SolanaRPCClient struct {
	inner rpcBackend
}

type rpcBackend interface {
	GetAccountInfoWithOpts(ctx context.Context, account solana.PublicKey, opts *rpcclient.GetAccountInfoOpts) (*rpcclient.GetAccountInfoResult, error)
	GetMultipleAccountsWithOpts(ctx context.Context, accounts []solana.PublicKey, opts *rpcclient.GetMultipleAccountsOpts) (*rpcclient.GetMultipleAccountsResult, error)
	GetTokenSupply(ctx context.Context, tokenMint solana.PublicKey, commitment rpcclient.CommitmentType) (*rpcclient.GetTokenSupplyResult, error)
	GetSignaturesForAddressWithOpts(ctx context.Context, account solana.PublicKey, opts *rpcclient.GetSignaturesForAddressOpts) ([]*rpcclient.TransactionSignature, error)
	GetTransaction(ctx context.Context, txSig solana.Signature, opts *rpcclient.GetTransactionOpts) (*rpcclient.GetTransactionResult, error)
	RPCCallForInto(ctx context.Context, out interface{}, method string, params []interface{}) error
}

func NewSolanaRPCClient(endpoint string) *SolanaRPCClient {
	return &SolanaRPCClient{inner: rpcclient.New(endpoint)}
}

func (c *SolanaRPCClient) GetAccount(ctx context.Context, address solana.PublicKey) (*AccountInfo, error) {
	recorder := reqdebug.FromContext(ctx)
	startedAt := time.Now()
	res, err := c.inner.GetAccountInfoWithOpts(ctx, address, &rpcclient.GetAccountInfoOpts{Encoding: solana.EncodingBase64})
	if recorder != nil {
		recorder.RecordRPC("get_account", time.Since(startedAt))
	}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return &AccountInfo{Address: address, Exists: false}, nil
		}
		return nil, err
	}
	if res == nil || res.Value == nil {
		return &AccountInfo{Address: address, Exists: false}, nil
	}
	raw := res.Value.Data.GetBinary()
	return &AccountInfo{Address: address, Owner: res.Value.Owner, Lamports: res.Value.Lamports, Data: raw, Slot: res.Context.Slot, Exists: true}, nil
}

func (c *SolanaRPCClient) GetMultipleAccounts(ctx context.Context, addresses []solana.PublicKey) ([]*AccountInfo, error) {
	recorder := reqdebug.FromContext(ctx)
	startedAt := time.Now()
	res, err := c.inner.GetMultipleAccountsWithOpts(ctx, addresses, &rpcclient.GetMultipleAccountsOpts{Encoding: solana.EncodingBase64})
	if recorder != nil {
		recorder.RecordRPC("get_multiple_accounts", time.Since(startedAt))
	}
	if err != nil {
		return nil, err
	}
	out := make([]*AccountInfo, 0, len(addresses))
	for i, v := range res.Value {
		if v == nil {
			out = append(out, &AccountInfo{Address: addresses[i], Exists: false})
			continue
		}
		raw := v.Data.GetBinary()
		out = append(out, &AccountInfo{Address: addresses[i], Owner: v.Owner, Lamports: v.Lamports, Data: raw, Slot: res.Context.Slot, Exists: true})
	}
	return out, nil
}

func (c *SolanaRPCClient) GetTokenSupply(ctx context.Context, mint solana.PublicKey) (decimal.Decimal, uint8, error) {
	recorder := reqdebug.FromContext(ctx)
	startedAt := time.Now()
	res, err := c.inner.GetTokenSupply(ctx, mint, rpcclient.CommitmentFinalized)
	if recorder != nil {
		recorder.RecordRPC("get_token_supply", time.Since(startedAt))
	}
	if err != nil {
		return decimal.Zero, 0, err
	}
	amt, err := decimal.NewFromString(res.Value.Amount)
	if err != nil {
		return decimal.Zero, 0, err
	}
	decimals := int32(res.Value.Decimals)
	return amt.Shift(-decimals), res.Value.Decimals, nil
}

func (c *SolanaRPCClient) GetSignaturesForAddress(ctx context.Context, address solana.PublicKey, opts *SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error) {
	recorder := reqdebug.FromContext(ctx)
	startedAt := time.Now()
	reqOpts := &rpcclient.GetSignaturesForAddressOpts{
		Commitment: rpcclient.CommitmentFinalized,
	}
	if opts != nil {
		if opts.Limit > 0 {
			reqOpts.Limit = &opts.Limit
		}
		if opts.Before != nil {
			reqOpts.Before = *opts.Before
		}
		if opts.Until != nil {
			reqOpts.Until = *opts.Until
		}
	}
	res, err := c.inner.GetSignaturesForAddressWithOpts(ctx, address, reqOpts)
	if recorder != nil {
		recorder.RecordRPC("get_signatures_for_address", time.Since(startedAt))
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *SolanaRPCClient) GetTransaction(ctx context.Context, signature solana.Signature) (*rpcclient.GetTransactionResult, error) {
	recorder := reqdebug.FromContext(ctx)
	startedAt := time.Now()
	maxSupportedVersion := uint64(0)
	res, err := c.inner.GetTransaction(ctx, signature, &rpcclient.GetTransactionOpts{
		Encoding:                       solana.EncodingBase64,
		Commitment:                     rpcclient.CommitmentFinalized,
		MaxSupportedTransactionVersion: &maxSupportedVersion,
	})
	if recorder != nil {
		recorder.RecordRPC("get_transaction", time.Since(startedAt))
	}
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, fmt.Errorf("transaction not found: %s", signature.String())
	}
	return res, nil
}

func (c *SolanaRPCClient) GetTransactionRaw(ctx context.Context, signature solana.Signature) ([]byte, error) {
	recorder := reqdebug.FromContext(ctx)
	startedAt := time.Now()
	var raw json.RawMessage
	// Use a plain map so the JSON-RPC payload always uses lower-camel keys
	// required by providers (especially maxSupportedTransactionVersion).
	opts := map[string]any{
		"encoding":                       solana.EncodingJSONParsed,
		"commitment":                     rpcclient.CommitmentFinalized,
		"maxSupportedTransactionVersion": uint64(0),
	}
	err := c.inner.RPCCallForInto(ctx, &raw, "getTransaction", []any{
		signature.String(),
		opts,
	})
	if recorder != nil {
		recorder.RecordRPC("get_transaction_raw", time.Since(startedAt))
	}
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("transaction not found: %s", signature.String())
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out, nil
}

type noopClient struct{}

func NewNoopClient() Client { return &noopClient{} }

func (n *noopClient) GetAccount(_ context.Context, address solana.PublicKey) (*AccountInfo, error) {
	return &AccountInfo{Address: address, Exists: false}, nil
}
func (n *noopClient) GetMultipleAccounts(_ context.Context, addresses []solana.PublicKey) ([]*AccountInfo, error) {
	out := make([]*AccountInfo, 0, len(addresses))
	for _, a := range addresses {
		out = append(out, &AccountInfo{Address: a, Exists: false})
	}
	return out, nil
}
func (n *noopClient) GetTokenSupply(_ context.Context, _ solana.PublicKey) (decimal.Decimal, uint8, error) {
	return decimal.Zero, 0, nil
}
func (n *noopClient) GetSignaturesForAddress(_ context.Context, _ solana.PublicKey, _ *SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error) {
	return nil, nil
}
func (n *noopClient) GetTransaction(_ context.Context, _ solana.Signature) (*rpcclient.GetTransactionResult, error) {
	return nil, nil
}
func (n *noopClient) GetTransactionRaw(_ context.Context, _ solana.Signature) ([]byte, error) {
	return nil, nil
}
