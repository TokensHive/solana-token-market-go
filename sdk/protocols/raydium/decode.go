package raydium

import (
	"github.com/TokensHive/solana-token-market-go/sdk/internal/pubkeyx"
	"github.com/TokensHive/solana-token-market-go/sdk/market"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

func DecodeConstantProductPool(address, mint solana.PublicKey, protocol market.Protocol, marketType market.MarketType, data []byte) *market.Pool {
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
	return &market.Pool{
		Address:           address.String(),
		Protocol:          protocol,
		MarketType:        marketType,
		BaseMint:          mint.String(),
		QuoteMint:         pubkeyx.WrappedSOLMintStr,
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
