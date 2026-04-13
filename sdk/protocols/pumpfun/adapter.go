package pumpfun

import (
	"context"
	"encoding/binary"
	"time"

	"github.com/TokensHive/solana-token-market-go/sdk/internal/pubkeyx"
	"github.com/TokensHive/solana-token-market-go/sdk/market"
	"github.com/TokensHive/solana-token-market-go/sdk/parser"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/gagliardetto/solana-go"
)

var defaultProgramID = mustPubkey("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")

type Adapter struct {
	rpc    rpc.Client
	parser parser.Adapter
}

func NewAdapter(c rpc.Client, p parser.Adapter) *Adapter { return &Adapter{rpc: c, parser: p} }
func (a *Adapter) Protocol() market.Protocol             { return market.ProtocolPumpfun }

func (a *Adapter) Discover(ctx context.Context, req market.DiscoveryRequest) ([]*market.Pool, error) {
	pool, err := a.deriveBondingCurve(ctx, req.Mint)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, parser.ErrNoEvidence
	}
	if pool.IsComplete {
		edges, _ := a.parser.FindPoolsByMint(req.Mint.String())
		for _, e := range edges {
			if e.Protocol == string(market.ProtocolPumpswap) || e.Protocol == string(market.ProtocolRaydiumV4) || e.Protocol == string(market.ProtocolRaydiumCPMM) {
				pool.Metadata["migrated_edge_pool"] = e.PoolAddress
			}
		}
	}
	return []*market.Pool{pool}, nil
}

func (a *Adapter) GetByAddress(ctx context.Context, pool solana.PublicKey) (*market.Pool, error) {
	info, err := a.rpc.GetAccount(ctx, pool)
	if err != nil || info == nil || !info.Exists {
		return nil, err
	}
	complete := false
	if len(info.Data) > 0 {
		complete = info.Data[len(info.Data)-1]%2 == 1
	}
	p := &market.Pool{Address: pool.String(), Protocol: market.ProtocolPumpfun, MarketType: market.MarketTypeBondingCurve, IsActive: true, IsVerified: true, IsComplete: complete, LastUpdatedAt: time.Now().UTC(), Metadata: map[string]any{}}
	return p, nil
}

func (a *Adapter) deriveBondingCurve(ctx context.Context, mint solana.PublicKey) (*market.Pool, error) {
	curve := DeriveBondingCurvePDA(mint)
	if curve.IsZero() {
		return nil, nil
	}
	acc, err := a.rpc.GetAccount(ctx, curve)
	if err != nil {
		return nil, err
	}
	if acc == nil || !acc.Exists {
		return nil, nil
	}
	state := DecodeCurveState(acc.Data)
	price, liq := ComputeCurvePriceAndLiquidity(state)
	return &market.Pool{
		Address:           curve.String(),
		Protocol:          market.ProtocolPumpfun,
		MarketType:        market.MarketTypeBondingCurve,
		BaseMint:          mint.String(),
		QuoteMint:         pubkeyx.WrappedSOLMintStr,
		BaseReserve:       state.TokenReserve,
		QuoteReserve:      state.SOLReserve,
		PriceOfTokenInSOL: price,
		LiquidityInSOL:    liq,
		LiquidityInQuote:  liq,
		IsVerified:        true,
		IsActive:          true,
		IsComplete:        state.Complete,
		LastUpdatedAt:     time.Now().UTC(),
		Derived:           market.DerivedAddresses{BondingCurve: curve.String()},
		Metadata:          map[string]any{"curve_type": "pumpfun_bonding_curve"},
	}, nil
}

func mustPubkey(v string) solana.PublicKey {
	p, err := solana.PublicKeyFromBase58(v)
	if err != nil {
		return solana.PublicKey{}
	}
	return p
}

func readU64(b []byte, off int) uint64 {
	if len(b) < off+8 {
		return 0
	}
	return binary.LittleEndian.Uint64(b[off : off+8])
}
