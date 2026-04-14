package market

import (
	"context"
	"fmt"

	pumpcurve "github.com/TokensHive/solana-token-market-go/sdk/protocols/pumpfun/bonding_curve"
	pumpamm "github.com/TokensHive/solana-token-market-go/sdk/protocols/pumpfun/pumpswap_amm"
)

func (c *Client) GetMetricsByPool(ctx context.Context, req GetMetricsByPoolRequest) (*GetMetricsByPoolResponse, error) {
	ctx, recorder := c.startDebug(ctx, "GetMetricsByPool")
	defer c.finishDebug(recorder)
	return c.service.GetMetricsByPool(ctx, req)
}

func (s *Service) GetMetricsByPool(ctx context.Context, req GetMetricsByPoolRequest) (*GetMetricsByPoolResponse, error) {
	if err := validateMetricsRequest(req); err != nil {
		return nil, err
	}

	route := PoolRoute{
		Dex:         req.Pool.Dex,
		PoolVersion: req.Pool.PoolVersion,
	}
	calculator, ok := s.calculators[route]
	if !ok || calculator == nil {
		return nil, NewError(ErrCodeInvalidArgument, fmt.Sprintf("unsupported pool route: %s/%s", req.Pool.Dex, req.Pool.PoolVersion), nil)
	}

	resp, err := calculator.Compute(ctx, req.Pool)
	if err != nil {
		return nil, err
	}
	resp.Metadata = attachRequestDebug(ctx, resp.Metadata)
	return resp, nil
}

func validateMetricsRequest(req GetMetricsByPoolRequest) error {
	if req.Pool.Dex == "" {
		return NewError(ErrCodeInvalidArgument, "pool dex is required", nil)
	}
	if req.Pool.PoolVersion == "" {
		return NewError(ErrCodeInvalidArgument, "pool version is required", nil)
	}
	if req.Pool.PoolAddress.IsZero() {
		return NewError(ErrCodeInvalidArgument, "pool address is required", nil)
	}
	if req.Pool.MintA.IsZero() || req.Pool.MintB.IsZero() {
		return NewError(ErrCodeInvalidArgument, "mintA and mintB are required", nil)
	}
	return nil
}

func defaultPoolCalculatorFactories() map[PoolRoute]PoolCalculatorFactory {
	return map[PoolRoute]PoolCalculatorFactory{
		{
			Dex:         DexPumpfun,
			PoolVersion: PoolVersionPumpfunBondingCurve,
		}: func(cfg Config) PoolCalculator {
			return poolCalculatorFunc(func(ctx context.Context, pool PoolIdentifier) (*GetMetricsByPoolResponse, error) {
				calculator := pumpcurve.NewCalculator(cfg.RPCClient, cfg.QuoteBridge)
				result, err := calculator.Compute(ctx, pumpcurve.Request{
					PoolAddress: pool.PoolAddress,
					MintA:       pool.MintA,
					MintB:       pool.MintB,
				})
				if err != nil {
					return nil, NewError(ErrCodeInternal, "pumpfun bonding curve metrics failed", err)
				}
				return buildMetricsResponse(pool, result.PriceOfAInB, result.PriceOfAInSOL, result.LiquidityInB, result.LiquidityInSOL, result.MarketCapInSOL, result.TotalSupply, result.CirculatingSupply, result.SupplyMethod, result.Metadata), nil
			})
		},
		{
			Dex:         DexPumpfun,
			PoolVersion: PoolVersionPumpfunAmm,
		}: func(cfg Config) PoolCalculator {
			return poolCalculatorFunc(func(ctx context.Context, pool PoolIdentifier) (*GetMetricsByPoolResponse, error) {
				calculator := pumpamm.NewCalculator(cfg.RPCClient, cfg.QuoteBridge, cfg.SupplyProvider)
				result, err := calculator.Compute(ctx, pumpamm.Request{
					PoolAddress: pool.PoolAddress,
					MintA:       pool.MintA,
					MintB:       pool.MintB,
				})
				if err != nil {
					return nil, NewError(ErrCodeInternal, "pumpfun amm metrics failed", err)
				}
				return buildMetricsResponse(pool, result.PriceOfAInB, result.PriceOfAInSOL, result.LiquidityInB, result.LiquidityInSOL, result.MarketCapInSOL, result.TotalSupply, result.CirculatingSupply, result.SupplyMethod, result.Metadata), nil
			})
		},
	}
}

func buildMetricsResponse(
	pool PoolIdentifier,
	priceOfAInB Decimal,
	priceOfAInSOL Decimal,
	liquidityInB Decimal,
	liquidityInSOL Decimal,
	marketCapInSOL Decimal,
	totalSupply Decimal,
	circulatingSupply Decimal,
	supplyMethod string,
	metadata map[string]any,
) *GetMetricsByPoolResponse {
	return &GetMetricsByPoolResponse{
		Pool:              pool,
		PriceOfAInB:       priceOfAInB,
		PriceOfAInSOL:     priceOfAInSOL,
		LiquidityInB:      liquidityInB,
		LiquidityInSOL:    liquidityInSOL,
		MarketCapInSOL:    marketCapInSOL,
		TotalSupply:       totalSupply,
		CirculatingSupply: circulatingSupply,
		SupplyMethod:      supplyMethod,
		Metadata:          metadata,
	}
}
