package discovery

import (
	"context"

	"github.com/TokensHive/solana-token-market-go/sdk/market"
	"github.com/TokensHive/solana-token-market-go/sdk/parser"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/gagliardetto/solana-go"
)

type Adapter interface {
	Protocol() market.Protocol
	Discover(ctx context.Context, req Request) ([]*market.Pool, error)
	GetByAddress(ctx context.Context, pool solana.PublicKey) (*market.Pool, error)
}

type Registry struct {
	adapters map[market.Protocol]Adapter
}

func NewRegistry() *Registry {
	return &Registry{adapters: map[market.Protocol]Adapter{}}
}

func (r *Registry) Register(a Adapter) {
	if a == nil {
		return
	}
	r.adapters[a.Protocol()] = a
}

func (r *Registry) Get(protocol market.Protocol) (Adapter, bool) {
	a, ok := r.adapters[protocol]
	return a, ok
}

func (r *Registry) List(protocols []market.Protocol) []Adapter {
	if len(protocols) == 0 {
		out := make([]Adapter, 0, len(r.adapters))
		for _, a := range r.adapters {
			out = append(out, a)
		}
		return out
	}
	out := make([]Adapter, 0, len(protocols))
	for _, p := range protocols {
		if a, ok := r.Get(p); ok {
			out = append(out, a)
		}
	}
	return out
}

func DefaultRegistry(c rpc.Client, p parser.Adapter) *Registry {
	return NewRegistryWithDefaults(c, p)
}
