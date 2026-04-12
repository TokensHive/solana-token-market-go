package discovery

import (
	"testing"
	"time"

	"github.com/TokensHive/solana-token-market-go/sdk/market"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

func TestApplyFilters(t *testing.T) {
	a := &market.Pool{Address: "a", Protocol: market.ProtocolRaydiumCPMM, MarketType: market.MarketTypeConstantProduct, QuoteMint: solana.SolMint.String(), IsActive: true, IsVerified: true, BaseReserve: decimal.NewFromInt(1), QuoteReserve: decimal.NewFromInt(1), LastUpdatedAt: time.Now()}
	b := &market.Pool{Address: "b", Protocol: market.ProtocolOrcaWhirlpool, MarketType: market.MarketTypeConcentratedLiquidity, QuoteMint: solana.SolMint.String(), IsActive: true, IsVerified: true, BaseReserve: decimal.NewFromInt(1), QuoteReserve: decimal.NewFromInt(1), LastUpdatedAt: time.Now()}
	out := ApplyFilters([]*market.Pool{a, b}, FilterOptions{Protocols: []market.Protocol{market.ProtocolRaydiumCPMM}, IncludeInactive: true, IncludeUnverified: true})
	if len(out) != 1 || out[0].Address != "a" {
		t.Fatalf("unexpected filtered set: %#v", out)
	}
}
