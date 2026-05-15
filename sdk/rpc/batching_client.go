package rpc

import (
	"context"

	"github.com/gagliardetto/solana-go"
	rpcclient "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

type chunkSizeContextKey struct{}

func WithGetMultipleAccountsChunkSize(ctx context.Context, size int) context.Context {
	if size <= 0 {
		return ctx
	}
	return context.WithValue(ctx, chunkSizeContextKey{}, size)
}

type chunkingClient struct {
	inner     Client
	chunkSize int
}

func NewChunkingClient(inner Client, chunkSize int) Client {
	if inner == nil {
		return nil
	}
	if chunkSize <= 0 {
		return inner
	}
	return &chunkingClient{
		inner:     inner,
		chunkSize: chunkSize,
	}
}

func (c *chunkingClient) GetAccount(ctx context.Context, address solana.PublicKey) (*AccountInfo, error) {
	return c.inner.GetAccount(ctx, address)
}

func (c *chunkingClient) GetMultipleAccounts(ctx context.Context, addresses []solana.PublicKey) ([]*AccountInfo, error) {
	if len(addresses) == 0 {
		return []*AccountInfo{}, nil
	}

	chunkSize := c.chunkSize
	if override, ok := ctx.Value(chunkSizeContextKey{}).(int); ok && override > 0 {
		chunkSize = override
	}
	if chunkSize <= 0 {
		return c.inner.GetMultipleAccounts(ctx, addresses)
	}

	unique := make([]solana.PublicKey, 0, len(addresses))
	seen := make(map[string]struct{}, len(addresses))
	for _, address := range addresses {
		key := address.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, address)
	}

	byAddress := make(map[string]*AccountInfo, len(unique))
	for _, chunk := range ChunkPubkeys(unique, chunkSize) {
		infos, err := c.inner.GetMultipleAccounts(ctx, chunk)
		if err != nil {
			return nil, err
		}
		for i := range infos {
			if infos[i] == nil {
				byAddress[chunk[i].String()] = &AccountInfo{
					Address: chunk[i],
					Exists:  false,
				}
				continue
			}
			byAddress[chunk[i].String()] = infos[i]
		}
	}

	out := make([]*AccountInfo, len(addresses))
	for i, address := range addresses {
		if info, ok := byAddress[address.String()]; ok {
			out[i] = info
			continue
		}
		out[i] = &AccountInfo{
			Address: address,
			Exists:  false,
		}
	}
	return out, nil
}

func (c *chunkingClient) GetTokenSupply(ctx context.Context, mint solana.PublicKey) (decimal.Decimal, uint8, error) {
	return c.inner.GetTokenSupply(ctx, mint)
}

func (c *chunkingClient) GetSignaturesForAddress(ctx context.Context, address solana.PublicKey, opts *SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error) {
	return c.inner.GetSignaturesForAddress(ctx, address, opts)
}

func (c *chunkingClient) GetTransaction(ctx context.Context, signature solana.Signature) (*rpcclient.GetTransactionResult, error) {
	return c.inner.GetTransaction(ctx, signature)
}

func (c *chunkingClient) GetTransactionRaw(ctx context.Context, signature solana.Signature) ([]byte, error) {
	return c.inner.GetTransactionRaw(ctx, signature)
}
