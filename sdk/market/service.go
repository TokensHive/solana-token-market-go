package market

import (
	"context"
	"sort"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

type Service struct {
	cfg Config
}

func NewService(cfg Config) *Service { return &Service{cfg: cfg} }

func (c *Client) ResolvePools(ctx context.Context, req ResolvePoolsRequest) (*ResolvePoolsResponse, error) {
	return c.service.ResolvePools(ctx, req)
}

func (s *Service) ResolvePools(ctx context.Context, req ResolvePoolsRequest) (*ResolvePoolsResponse, error) {
	if req.Mint.IsZero() {
		return nil, NewError(ErrCodeInvalidArgument, "mint is required", nil)
	}
	mode := req.DiscoveryMode
	if mode == "" {
		mode = s.cfg.DefaultMode
	}
	pools, meta, err := s.cfg.DiscoveryEngine.Discover(ctx, DiscoveryRequest{
		Mint:              req.Mint,
		QuoteMint:         req.QuoteMint,
		Protocols:         req.Protocols,
		MarketTypes:       req.MarketTypes,
		IncludeInactive:   req.IncludeInactive,
		IncludeUnverified: req.IncludeUnverified,
		DiscoveryMode:     mode,
		PoolAddresses:     req.PoolAddresses,
		DirectSOLOnly:     req.DirectSOLOnly,
	})
	if err != nil {
		return nil, err
	}
	resp := &ResolvePoolsResponse{Mint: req.Mint.String(), Pools: pools, Metadata: ResolveMetadata{DiscoveryMode: mode, PoolsFound: len(pools), RankedAt: time.Now().UTC(), Debug: meta}}
	if req.SelectPrimary {
		resp.PrimaryPool = selectPrimaryFromRankedPools(pools)
	}
	return resp, nil
}

func selectPrimaryFromRankedPools(pools []*Pool) *Pool {
	if len(pools) == 0 {
		return nil
	}
	for _, pool := range pools {
		if pool != nil && pool.IsPrimary {
			return pool
		}
	}
	return pools[0]
}
func (c *Client) GetPool(ctx context.Context, req GetPoolRequest) (*Pool, error) {
	return c.service.GetPool(ctx, req)
}

func (s *Service) GetPool(ctx context.Context, req GetPoolRequest) (*Pool, error) {
	if req.PoolAddress.IsZero() {
		return nil, NewError(ErrCodeInvalidArgument, "pool address is required", nil)
	}
	pools, _, err := s.cfg.DiscoveryEngine.FindByPoolAddress(ctx, req.PoolAddress)
	if err != nil {
		return nil, err
	}
	if len(pools) == 0 {
		return nil, NewError(ErrCodeNotFound, "pool not found", nil)
	}
	return pools[0], nil
}

func (c *Client) GetTokenMarket(ctx context.Context, req GetTokenMarketRequest) (*GetTokenMarketResponse, error) {
	return c.service.GetTokenMarket(ctx, req)
}

func (s *Service) GetTokenMarket(ctx context.Context, req GetTokenMarketRequest) (*GetTokenMarketResponse, error) {
	resolved, err := s.ResolvePools(ctx, ResolvePoolsRequest{
		Mint:              req.Mint,
		QuoteMint:         req.QuoteMint,
		Protocols:         req.Protocols,
		DiscoveryMode:     req.DiscoveryMode,
		IncludeInactive:   req.IncludeInactive,
		IncludeUnverified: req.IncludeUnverified,
		DirectSOLOnly:     req.DirectSOLOnly,
		SelectPrimary:     true,
	})
	if err != nil {
		return nil, err
	}
	if resolved.PrimaryPool == nil {
		return nil, NewError(ErrCodeNotFound, "no primary pool", nil)
	}
	total, circ, method, err := s.cfg.SupplyProvider.GetSupply(ctx, req.Mint)
	if err != nil {
		return nil, err
	}
	primary := resolved.PrimaryPool
	primary.TotalSupply = total
	primary.CirculatingSupply = circ
	primary.MarketCapInSOL = primary.PriceOfTokenInSOL.Mul(circ)

	return &GetTokenMarketResponse{
		Mint:              req.Mint.String(),
		PrimaryPool:       primary,
		Pools:             resolved.Pools,
		PriceInSOL:        primary.PriceOfTokenInSOL,
		LiquidityInSOL:    primary.LiquidityInSOL,
		MarketCapInSOL:    primary.MarketCapInSOL,
		TotalSupply:       total,
		CirculatingSupply: circ,
		Metadata:          MarketMetadata{DiscoveryMode: resolved.Metadata.DiscoveryMode, SupplyMethod: method, Debug: resolved.Metadata.Debug},
	}, nil
}

func (c *Client) FindPoolsByMint(ctx context.Context, mint solana.PublicKey) ([]*Pool, error) {
	res, err := c.ResolvePools(ctx, ResolvePoolsRequest{Mint: mint, IncludeUnverified: true, SelectPrimary: false})
	if err != nil {
		return nil, err
	}
	return res.Pools, nil
}

func (c *Client) FindPoolsByPair(ctx context.Context, baseMint, quoteMint solana.PublicKey) ([]*Pool, error) {
	res, err := c.ResolvePools(ctx, ResolvePoolsRequest{Mint: baseMint, QuoteMint: &quoteMint, IncludeUnverified: true, SelectPrimary: false})
	if err != nil {
		return nil, err
	}
	return res.Pools, nil
}

func (c *Client) FindPoolsByProtocol(ctx context.Context, mint solana.PublicKey, protocol Protocol) ([]*Pool, error) {
	res, err := c.ResolvePools(ctx, ResolvePoolsRequest{Mint: mint, Protocols: []Protocol{protocol}, IncludeUnverified: true, SelectPrimary: false})
	if err != nil {
		return nil, err
	}
	return res.Pools, nil
}

func (c *Client) ComputePoolMetrics(_ context.Context, p *Pool, totalSupply, circulatingSupply decimal.Decimal) (*PoolMetrics, error) {
	if p == nil {
		return nil, NewError(ErrCodeInvalidArgument, "pool is required", nil)
	}
	mc := p.PriceOfTokenInSOL.Mul(circulatingSupply)
	return &PoolMetrics{PriceOfTokenInSOL: p.PriceOfTokenInSOL, LiquidityInSOL: p.LiquidityInSOL, LiquidityInQuote: p.LiquidityInQuote, TotalSupply: totalSupply, CirculatingSupply: circulatingSupply, MarketCapInSOL: mc}, nil
}

func (c *Client) ComputeTokenMetricsFromPool(ctx context.Context, mint solana.PublicKey, p *Pool) (*PoolMetrics, error) {
	total, circ, _, err := c.cfg.SupplyProvider.GetSupply(ctx, mint)
	if err != nil {
		return nil, err
	}
	return c.ComputePoolMetrics(ctx, p, total, circ)
}

func SelectPrimaryPool(pools []*Pool, targetMint string, quoteMint *string) (*Pool, error) {
	if len(pools) == 0 {
		return nil, NewError(ErrCodeNotFound, "no pools found", nil)
	}
	candidates := make([]*Pool, 0, len(pools))
	for _, p := range pools {
		if p == nil {
			continue
		}
		if targetMint != "" && p.BaseMint != "" && p.BaseMint != targetMint && p.QuoteMint != targetMint {
			continue
		}
		candidates = append(candidates, p)
	}
	if len(candidates) == 0 {
		candidates = pools
	}
	ranked := sortByScore(candidates, quoteMint)
	if len(ranked) == 0 {
		return nil, NewError(ErrCodeNotFound, "no rankable pools", nil)
	}
	if ranked[0].Protocol == ProtocolPumpfun && ranked[0].IsComplete {
		for _, p := range ranked[1:] {
			if p.Protocol != ProtocolPumpfun && p.LiquidityInSOL.GreaterThan(decimal.Zero) {
				ranked[0].IsPrimary = false
				p.IsPrimary = true
				return p, nil
			}
		}
	}
	for i := range ranked {
		ranked[i].IsPrimary = i == 0
	}
	return ranked[0], nil
}

func normalize(v, max decimal.Decimal) decimal.Decimal {
	if max.IsZero() {
		return decimal.Zero
	}
	return v.Div(max)
}

func scoreForPool(p *Pool, maxLiq decimal.Decimal, preferredQuote *string) {
	wLiq := decimal.NewFromFloat(0.55)
	wFresh := decimal.NewFromFloat(0.15)
	wQuote := decimal.NewFromFloat(0.10)
	wVerified := decimal.NewFromFloat(0.10)
	wStalePenalty := decimal.NewFromFloat(0.05)
	wZeroReservePenalty := decimal.NewFromFloat(0.05)
	freshness := decimal.NewFromInt(1)
	if !p.LastUpdatedAt.IsZero() && time.Since(p.LastUpdatedAt) > 15*time.Minute {
		freshness = decimal.NewFromFloat(0.25)
	}
	quotePref := decimal.NewFromFloat(0.5)
	if p.QuoteMint == solana.SolMint.String() {
		quotePref = decimal.NewFromInt(1)
	}
	if preferredQuote != nil && p.QuoteMint == *preferredQuote {
		quotePref = decimal.NewFromFloat(1.1)
	}
	verified := decimal.Zero
	if p.IsVerified {
		verified = decimal.NewFromInt(1)
	}
	stalePenalty := decimal.Zero
	if !p.LastUpdatedAt.IsZero() && time.Since(p.LastUpdatedAt) > time.Hour {
		stalePenalty = decimal.NewFromInt(1)
	}
	zeroPenalty := decimal.Zero
	if p.BaseReserve.IsZero() || p.QuoteReserve.IsZero() {
		zeroPenalty = decimal.NewFromInt(1)
	}
	score := wLiq.Mul(normalize(p.LiquidityInSOL, maxLiq)).
		Add(wFresh.Mul(freshness)).
		Add(wQuote.Mul(quotePref)).
		Add(wVerified.Mul(verified)).
		Sub(wStalePenalty.Mul(stalePenalty)).
		Sub(wZeroReservePenalty.Mul(zeroPenalty))
	p.SelectionScore = score
}

func sortByScore(pools []*Pool, preferredQuote *string) []*Pool {
	cp := append([]*Pool{}, pools...)
	if len(cp) == 0 {
		return cp
	}
	maxLiq := decimal.Zero
	for _, p := range cp {
		if p.LiquidityInSOL.GreaterThan(maxLiq) {
			maxLiq = p.LiquidityInSOL
		}
	}
	for _, p := range cp {
		scoreForPool(p, maxLiq, preferredQuote)
	}
	sort.SliceStable(cp, func(i, j int) bool {
		return cp[i].SelectionScore.GreaterThan(cp[j].SelectionScore)
	})
	if len(cp) > 0 {
		cp[0].SelectionReason = "highest deterministic composite score"
	}
	return cp
}
