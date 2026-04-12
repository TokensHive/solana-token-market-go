package market

import (
"context"

"github.com/gagliardetto/solana-go"
)

type noopDiscovery struct{}

func newNoopDiscoveryEngine() DiscoveryEngine { return &noopDiscovery{} }

func (n *noopDiscovery) Discover(_ context.Context, _ DiscoveryRequest) ([]*Pool, map[string]any, error) {
return nil, map[string]any{"source": "noop"}, nil
}

func (n *noopDiscovery) FindByPoolAddress(_ context.Context, _ solana.PublicKey) ([]*Pool, map[string]any, error) {
return nil, map[string]any{"source": "noop"}, nil
}
