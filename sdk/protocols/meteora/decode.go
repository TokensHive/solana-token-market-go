package meteora

import (
	"github.com/TokensHive/solana-token-market-go/sdk/internal/pubkeyx"
	"github.com/TokensHive/solana-token-market-go/sdk/market"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

func DecodePool(protocol market.Protocol, mtype market.MarketType, addr, mint solana.PublicKey, data []byte) *market.Pool {
	base := decimal.NewFromInt(1)
	quote := decimal.NewFromInt(1)
	if len(data) > 2 {
		base = decimal.NewFromInt(int64(data[0]) + 1)
		quote = decimal.NewFromInt(int64(data[1]) + 1)
	}
	price := decimal.Zero
	if !base.IsZero() {
		price = quote.Div(base)
	}
	liq := quote.Mul(decimal.NewFromInt(2))
	meta := map[string]any{
		"estimated_from_placeholder_decode": true,
	}
	if protocol == market.ProtocolMeteoraDLMM {
		meta["estimated_liquidity"] = true
	}
	// Placeholder byte-derived decoding is intentionally unverified.
	return &market.Pool{Address: addr.String(), Protocol: protocol, MarketType: mtype, BaseMint: mint.String(), QuoteMint: pubkeyx.WrappedSOLMintStr, BaseReserve: base, QuoteReserve: quote, PriceOfTokenInSOL: price, LiquidityInSOL: liq, LiquidityInQuote: liq, IsVerified: false, IsActive: true, Metadata: meta}
}
