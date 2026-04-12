package raydium

import (
	"context"
	"encoding/binary"
	"math"
	"time"

	"github.com/TokensHive/solana-token-market-go/sdk/market"
	"github.com/TokensHive/solana-token-market-go/sdk/parser"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

type clmmAdapter struct{ baseAdapter }

func (a *clmmAdapter) Discover(ctx context.Context, req market.DiscoveryRequest) ([]*market.Pool, error) {
	edges, err := a.parser.FindPoolsByMint(req.Mint.String())
	if err != nil {
		return nil, err
	}
	out := make([]*market.Pool, 0, len(edges))
	for _, e := range edges {
		if e.Protocol != string(market.ProtocolRaydiumCLMM) {
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
		price := DecodeCLMMPrice(acc.Data)
		p := &market.Pool{Address: pk.String(), Protocol: market.ProtocolRaydiumCLMM, MarketType: market.MarketTypeConcentratedLiquidity, BaseMint: req.Mint.String(), QuoteMint: solana.SolMint.String(), PriceOfTokenInSOL: price, LiquidityInSOL: decimal.Zero, LiquidityInQuote: decimal.Zero, IsVerified: true, IsActive: true, LastUpdatedAt: time.Now().UTC(), Metadata: map[string]any{"estimated_liquidity": true}}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, parser.ErrNoEvidence
	}
	return out, nil
}

func DecodeCLMMPrice(data []byte) decimal.Decimal {
	if len(data) < 24 {
		return decimal.Zero
	}
	sqrtX64 := binary.LittleEndian.Uint64(data[16:24])
	if sqrtX64 == 0 {
		return decimal.Zero
	}
	ratio := float64(sqrtX64) / math.Pow(2, 64)
	return decimal.NewFromFloat(ratio * ratio)
}
