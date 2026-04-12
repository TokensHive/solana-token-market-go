package orca

import (
	"context"
	"time"

	"github.com/TokensHive/solana-token-market-go/sdk/market"
	"github.com/TokensHive/solana-token-market-go/sdk/parser"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

type Adapter struct {
	rpc    rpc.Client
	parser parser.Adapter
}

func NewAdapter(c rpc.Client, p parser.Adapter) *Adapter { return &Adapter{rpc: c, parser: p} }
func (a *Adapter) Protocol() market.Protocol             { return market.ProtocolOrcaWhirlpool }

func (a *Adapter) Discover(ctx context.Context, req market.DiscoveryRequest) ([]*market.Pool, error) {
	edges, err := a.parser.FindPoolsByMint(req.Mint.String())
	if err != nil {
		return nil, err
	}
	out := make([]*market.Pool, 0, len(edges))
	for _, e := range edges {
		if e.Protocol != string(market.ProtocolOrcaWhirlpool) {
			continue
		}
		pk, err := solana.PublicKeyFromBase58(e.PoolAddress)
		if err != nil {
			continue
		}
		acc, err := a.rpc.GetAccount(ctx, pk)
		if err != nil || acc == nil || !acc.Exists {
			continue
		}
		price := DecodeWhirlpoolPrice(acc.Data)
		p := &market.Pool{Address: pk.String(), Protocol: market.ProtocolOrcaWhirlpool, MarketType: market.MarketTypeConcentratedLiquidity, BaseMint: req.Mint.String(), QuoteMint: solana.SolMint.String(), PriceOfTokenInSOL: price, LiquidityInSOL: decimal.Zero, LiquidityInQuote: decimal.Zero, IsVerified: true, IsActive: true, LastUpdatedAt: time.Now().UTC(), Metadata: map[string]any{"estimated_liquidity": true}}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, parser.ErrNoEvidence
	}
	return out, nil
}

func (a *Adapter) GetByAddress(ctx context.Context, pool solana.PublicKey) (*market.Pool, error) {
	acc, err := a.rpc.GetAccount(ctx, pool)
	if err != nil || acc == nil || !acc.Exists {
		return nil, err
	}
	p := &market.Pool{Address: pool.String(), Protocol: market.ProtocolOrcaWhirlpool, MarketType: market.MarketTypeConcentratedLiquidity, QuoteMint: solana.SolMint.String(), PriceOfTokenInSOL: DecodeWhirlpoolPrice(acc.Data), IsVerified: true, IsActive: true, LastUpdatedAt: time.Now().UTC(), Metadata: map[string]any{"estimated_liquidity": true}}
	return p, nil
}
