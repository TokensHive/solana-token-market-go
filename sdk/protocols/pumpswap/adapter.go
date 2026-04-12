package pumpswap

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
func (a *Adapter) Protocol() market.Protocol             { return market.ProtocolPumpswap }

func (a *Adapter) Discover(ctx context.Context, req market.DiscoveryRequest) ([]*market.Pool, error) {
	edges, err := a.parser.FindPoolsByMint(req.Mint.String())
	if err != nil {
		return nil, err
	}
	out := make([]*market.Pool, 0, len(edges))
	for _, e := range edges {
		if e.Protocol != string(market.ProtocolPumpswap) {
			continue
		}
		addr, err := solana.PublicKeyFromBase58(e.PoolAddress)
		if err != nil {
			continue
		}
		acc, err := a.rpc.GetAccount(ctx, addr)
		if err != nil || acc == nil || !acc.Exists {
			continue
		}
		p := DecodePool(addr, req.Mint, acc.Data)
		p.LastUpdatedAt = time.Now().UTC()
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
	p := DecodePool(pool, solana.PublicKey{}, acc.Data)
	p.LastUpdatedAt = time.Now().UTC()
	return p, nil
}

func DecodePool(address, mint solana.PublicKey, data []byte) *market.Pool {
	base := decimal.NewFromInt(1)
	quote := decimal.NewFromInt(1)
	if len(data) >= 16 {
		base = decimal.NewFromInt(int64(data[0]) + 1)
		quote = decimal.NewFromInt(int64(data[1]) + 1)
	}
	price := quote.Div(base)
	liq := quote.Mul(decimal.NewFromInt(2))
	return &market.Pool{
		Address:           address.String(),
		Protocol:          market.ProtocolPumpswap,
		MarketType:        market.MarketTypeConstantProduct,
		BaseMint:          mint.String(),
		QuoteMint:         solana.SolMint.String(),
		BaseReserve:       base,
		QuoteReserve:      quote,
		PriceOfTokenInSOL: price,
		LiquidityInSOL:    liq,
		LiquidityInQuote:  liq,
		// Placeholder byte-derived decoding is intentionally unverified.
		IsVerified: false,
		IsActive:   true,
		Metadata: map[string]any{
			"estimated_from_placeholder_decode": true,
		},
	}
}
