package market

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

func TestSelectPrimaryPool_CompletedPumpfunPrefersMigrated(t *testing.T) {
	curve := &Pool{Address: "curve", Protocol: ProtocolPumpfun, IsComplete: true, IsActive: true, IsVerified: true, BaseReserve: decimal.NewFromInt(100), QuoteReserve: decimal.NewFromInt(1), LiquidityInSOL: decimal.NewFromInt(1), LastUpdatedAt: time.Now()}
	migrated := &Pool{Address: "amm", Protocol: ProtocolPumpswap, IsActive: true, IsVerified: true, BaseReserve: decimal.NewFromInt(1000), QuoteReserve: decimal.NewFromInt(50), LiquidityInSOL: decimal.NewFromInt(50), LastUpdatedAt: time.Now()}
	p, err := SelectPrimaryPool([]*Pool{curve, migrated}, "mint", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Address != "amm" {
		t.Fatalf("expected migrated pool primary, got %s", p.Address)
	}
}

func TestSelectPrimaryPool_RaydiumMostFunded(t *testing.T) {
	a := &Pool{Address: "a", Protocol: ProtocolRaydiumCPMM, QuoteMint: solana.SolMint.String(), IsActive: true, IsVerified: true, BaseReserve: decimal.NewFromInt(100), QuoteReserve: decimal.NewFromInt(5), LiquidityInSOL: decimal.NewFromInt(5), LastUpdatedAt: time.Now()}
	b := &Pool{Address: "b", Protocol: ProtocolRaydiumCPMM, QuoteMint: solana.SolMint.String(), IsActive: true, IsVerified: true, BaseReserve: decimal.NewFromInt(100), QuoteReserve: decimal.NewFromInt(12), LiquidityInSOL: decimal.NewFromInt(12), LastUpdatedAt: time.Now()}
	p, err := SelectPrimaryPool([]*Pool{a, b}, "mint", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Address != "b" {
		t.Fatalf("expected b, got %s", p.Address)
	}
}

func TestSelectPrimaryPool_Deterministic(t *testing.T) {
	pools := []*Pool{{Address: "x", Protocol: ProtocolRaydiumV4, IsActive: true, IsVerified: true, BaseReserve: decimal.NewFromInt(10), QuoteReserve: decimal.NewFromInt(3), LiquidityInSOL: decimal.NewFromInt(3), LastUpdatedAt: time.Now()}, {Address: "y", Protocol: ProtocolRaydiumV4, IsActive: true, IsVerified: true, BaseReserve: decimal.NewFromInt(10), QuoteReserve: decimal.NewFromInt(2), LiquidityInSOL: decimal.NewFromInt(2), LastUpdatedAt: time.Now()}}
	p1, err := SelectPrimaryPool(pools, "mint", nil)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := SelectPrimaryPool(pools, "mint", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p1.Address != p2.Address {
		t.Fatalf("expected deterministic primary, got %s then %s", p1.Address, p2.Address)
	}
}

func TestGoldenKnownMintPools(t *testing.T) {
	bytes, err := os.ReadFile("../internal/testdata/known_mint_pools.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Mint        string `json:"mint"`
		PrimaryPool string `json:"primary_pool"`
		Pools       []struct {
			Address string `json:"address"`
		} `json:"pools"`
	}
	if err := json.Unmarshal(bytes, &got); err != nil {
		t.Fatal(err)
	}
	if got.PrimaryPool == "" || len(got.Pools) == 0 {
		t.Fatal("invalid golden fixture")
	}
}
