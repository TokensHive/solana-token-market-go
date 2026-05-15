package rpc

import (
	"context"
	"sync"

	"github.com/gagliardetto/solana-go"
	rpcclient "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

type PreloadStats struct {
	GetAccountCalls          int
	GetMultipleAccountsCalls int
	TotalAccountsRequested   int
}

type preloadedClient struct {
	inner Client

	mu    sync.RWMutex
	cache map[string]*AccountInfo
	stats PreloadStats
}

func NewPreloadedClient(inner Client) *preloadedClient {
	if inner == nil {
		inner = NewNoopClient()
	}
	return &preloadedClient{
		inner: inner,
		cache: map[string]*AccountInfo{},
	}
}

func (c *preloadedClient) PrimeAccounts(ctx context.Context, addresses []solana.PublicKey) error {
	_, err := c.fetchMissingAccounts(ctx, addresses)
	return err
}

func (c *preloadedClient) Stats() PreloadStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

func (c *preloadedClient) GetAccount(ctx context.Context, address solana.PublicKey) (*AccountInfo, error) {
	c.mu.Lock()
	c.stats.GetAccountCalls++
	c.stats.TotalAccountsRequested++
	c.mu.Unlock()

	if info, ok := c.getCached(address); ok {
		return info, nil
	}
	infos, err := c.fetchMissingAccounts(ctx, []solana.PublicKey{address})
	if err != nil {
		return nil, err
	}
	return infos[0], nil
}

func (c *preloadedClient) GetMultipleAccounts(ctx context.Context, addresses []solana.PublicKey) ([]*AccountInfo, error) {
	c.mu.Lock()
	c.stats.GetMultipleAccountsCalls++
	c.stats.TotalAccountsRequested += len(addresses)
	c.mu.Unlock()
	return c.fetchMissingAccounts(ctx, addresses)
}

func (c *preloadedClient) GetTokenSupply(ctx context.Context, mint solana.PublicKey) (decimal.Decimal, uint8, error) {
	return c.inner.GetTokenSupply(ctx, mint)
}

func (c *preloadedClient) GetSignaturesForAddress(ctx context.Context, address solana.PublicKey, opts *SignaturesForAddressOptions) ([]*rpcclient.TransactionSignature, error) {
	return c.inner.GetSignaturesForAddress(ctx, address, opts)
}

func (c *preloadedClient) GetTransaction(ctx context.Context, signature solana.Signature) (*rpcclient.GetTransactionResult, error) {
	return c.inner.GetTransaction(ctx, signature)
}

func (c *preloadedClient) GetTransactionRaw(ctx context.Context, signature solana.Signature) ([]byte, error) {
	return c.inner.GetTransactionRaw(ctx, signature)
}

func (c *preloadedClient) fetchMissingAccounts(ctx context.Context, addresses []solana.PublicKey) ([]*AccountInfo, error) {
	if len(addresses) == 0 {
		return []*AccountInfo{}, nil
	}

	missingUnique := make([]solana.PublicKey, 0, len(addresses))
	missingSeen := make(map[string]struct{}, len(addresses))
	for _, address := range addresses {
		key := address.String()
		if _, ok := c.getCached(address); ok {
			continue
		}
		if _, exists := missingSeen[key]; exists {
			continue
		}
		missingSeen[key] = struct{}{}
		missingUnique = append(missingUnique, address)
	}

	if len(missingUnique) > 0 {
		infos, err := c.inner.GetMultipleAccounts(ctx, missingUnique)
		if err != nil {
			return nil, err
		}
		if len(infos) == 0 {
			infos = make([]*AccountInfo, len(missingUnique))
		}
		if len(infos) < len(missingUnique) {
			padding := make([]*AccountInfo, len(missingUnique))
			copy(padding, infos)
			infos = padding
		}
		for i := range missingUnique {
			if infos[i] != nil && infos[i].Exists {
				continue
			}
			info, getErr := c.inner.GetAccount(ctx, missingUnique[i])
			if getErr != nil {
				return nil, getErr
			}
			infos[i] = info
		}
		c.mu.Lock()
		for i, key := range missingUnique {
			info := &AccountInfo{Address: key, Exists: false}
			if i < len(infos) && infos[i] != nil {
				info = infos[i]
			}
			c.cache[key.String()] = info
		}
		c.mu.Unlock()
	}

	out := make([]*AccountInfo, len(addresses))
	for i, address := range addresses {
		info, _ := c.getCached(address)
		out[i] = info
	}
	return out, nil
}

func (c *preloadedClient) getCached(address solana.PublicKey) (*AccountInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	info, ok := c.cache[address.String()]
	return info, ok
}
