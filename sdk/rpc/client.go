package rpc

import (
	"context"

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
}

type SolanaRPCClient struct {
	inner *rpcclient.Client
}

func NewSolanaRPCClient(endpoint string) *SolanaRPCClient {
	return &SolanaRPCClient{inner: rpcclient.New(endpoint)}
}

func (c *SolanaRPCClient) GetAccount(ctx context.Context, address solana.PublicKey) (*AccountInfo, error) {
	res, err := c.inner.GetAccountInfoWithOpts(ctx, address, &rpcclient.GetAccountInfoOpts{Encoding: solana.EncodingBase64})
	if err != nil {
		return nil, err
	}
	if res == nil || res.Value == nil {
		return &AccountInfo{Address: address, Exists: false}, nil
	}
	raw := res.Value.Data.GetBinary()
	return &AccountInfo{Address: address, Owner: res.Value.Owner, Lamports: res.Value.Lamports, Data: raw, Slot: res.Context.Slot, Exists: true}, nil
}

func (c *SolanaRPCClient) GetMultipleAccounts(ctx context.Context, addresses []solana.PublicKey) ([]*AccountInfo, error) {
	res, err := c.inner.GetMultipleAccountsWithOpts(ctx, addresses, &rpcclient.GetMultipleAccountsOpts{Encoding: solana.EncodingBase64})
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
	res, err := c.inner.GetTokenSupply(ctx, mint, rpcclient.CommitmentFinalized)
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
