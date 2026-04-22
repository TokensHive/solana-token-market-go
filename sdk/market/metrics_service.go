package market

import (
	"context"
	"fmt"

	meteordammv1 "github.com/TokensHive/solana-token-market-go/sdk/protocols/meteora/damm_v1"
	meteordammv2 "github.com/TokensHive/solana-token-market-go/sdk/protocols/meteora/damm_v2"
	meteordbc "github.com/TokensHive/solana-token-market-go/sdk/protocols/meteora/dbc"
	meteordlmm "github.com/TokensHive/solana-token-market-go/sdk/protocols/meteora/dlmm"
	orcawhirlpool "github.com/TokensHive/solana-token-market-go/sdk/protocols/orca/whirlpool"
	pumpcurve "github.com/TokensHive/solana-token-market-go/sdk/protocols/pumpfun/bonding_curve"
	pumpamm "github.com/TokensHive/solana-token-market-go/sdk/protocols/pumpfun/pumpswap_amm"
	raydiumclmm "github.com/TokensHive/solana-token-market-go/sdk/protocols/raydium/clmm"
	raydiumcpmm "github.com/TokensHive/solana-token-market-go/sdk/protocols/raydium/cpmm"
	raydiumlaunchpad "github.com/TokensHive/solana-token-market-go/sdk/protocols/raydium/launchpad"
	raydiumv4 "github.com/TokensHive/solana-token-market-go/sdk/protocols/raydium/liquidity_v4"
	"github.com/gagliardetto/solana-go"
)

const pumpfunBondingCurveSeed = "bonding-curve"

var pumpfunProgramID = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
var findProgramAddress = solana.FindProgramAddress

func (c *Client) GetMetricsByPool(ctx context.Context, req GetMetricsByPoolRequest) (*GetMetricsByPoolResponse, error) {
	ctx, recorder := c.startDebug(ctx, "GetMetricsByPool")
	defer c.finishDebug(recorder)
	return c.service.GetMetricsByPool(ctx, req)
}

func (c *Client) GetMetricsByPumpfunBondingCurve(ctx context.Context, req GetMetricsByPumpfunBondingCurveRequest) (*GetMetricsByPoolResponse, error) {
	ctx, recorder := c.startDebug(ctx, "GetMetricsByPumpfunBondingCurve")
	defer c.finishDebug(recorder)
	return c.service.GetMetricsByPumpfunBondingCurve(ctx, req)
}

func (s *Service) GetMetricsByPool(ctx context.Context, req GetMetricsByPoolRequest) (*GetMetricsByPoolResponse, error) {
	if err := validateMetricsRequest(req); err != nil {
		return nil, err
	}
	if req.Pool.Dex == DexPumpfun && req.Pool.PoolVersion == PoolVersionPumpfunBondingCurve {
		return nil, NewError(
			ErrCodeInvalidArgument,
			"pumpfun bonding_curve requires GetMetricsByPumpfunBondingCurve (mint-based) instead of GetMetricsByPool",
			nil,
		)
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

func (s *Service) GetMetricsByPumpfunBondingCurve(ctx context.Context, req GetMetricsByPumpfunBondingCurveRequest) (*GetMetricsByPoolResponse, error) {
	if err := validatePumpfunBondingCurveRequest(req); err != nil {
		return nil, err
	}

	tokenMint := req.MintA
	if solana.SolMint.Equals(req.MintA) {
		tokenMint = req.MintB
	}
	poolAddress, _, err := findProgramAddress([][]byte{
		[]byte(pumpfunBondingCurveSeed),
		tokenMint.Bytes(),
	}, pumpfunProgramID)
	if err != nil {
		return nil, NewError(ErrCodeInternal, "derive pumpfun bonding curve address", err)
	}

	calculator := pumpcurve.NewCalculator(s.cfg.RPCClient, s.cfg.QuoteBridge)
	result, err := calculator.Compute(ctx, pumpcurve.Request{
		PoolAddress: poolAddress,
		MintA:       req.MintA,
		MintB:       req.MintB,
	})
	if err != nil {
		return nil, NewError(ErrCodeInternal, "pumpfun bonding curve metrics failed", err)
	}
	resp := buildMetricsResponse(
		PoolIdentifier{
			Dex:         DexPumpfun,
			PoolVersion: PoolVersionPumpfunBondingCurve,
			PoolAddress: poolAddress,
		},
		req.MintA,
		req.MintB,
		result.PriceOfAInB,
		result.PriceOfAInSOL,
		result.LiquidityInB,
		result.LiquidityInSOL,
		result.MarketCapInSOL,
		result.FDVInSOL,
		result.TotalSupply,
		result.CirculatingSupply,
		result.SupplyMethod,
		result.Metadata,
	)
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
	return nil
}

func validatePumpfunBondingCurveRequest(req GetMetricsByPumpfunBondingCurveRequest) error {
	if req.MintA.IsZero() || req.MintB.IsZero() {
		return NewError(ErrCodeInvalidArgument, "mintA and mintB are required", nil)
	}
	if !solana.SolMint.Equals(req.MintA) && !solana.SolMint.Equals(req.MintB) {
		return NewError(ErrCodeInvalidArgument, "pumpfun bonding curve requires one side to be SOL", nil)
	}
	return nil
}

func defaultPoolCalculatorFactories() map[PoolRoute]PoolCalculatorFactory {
	return map[PoolRoute]PoolCalculatorFactory{
		{
			Dex:         DexPumpfun,
			PoolVersion: PoolVersionPumpfunAmm,
		}: func(cfg Config) PoolCalculator {
			return poolCalculatorFunc(func(ctx context.Context, pool PoolIdentifier) (*GetMetricsByPoolResponse, error) {
				calculator := pumpamm.NewCalculator(cfg.RPCClient, cfg.QuoteBridge, cfg.SupplyProvider)
				result, err := calculator.Compute(ctx, pumpamm.Request{
					PoolAddress: pool.PoolAddress,
				})
				if err != nil {
					return nil, NewError(ErrCodeInternal, "pumpfun amm metrics failed", err)
				}
				return buildMetricsResponse(pool, result.MintA, result.MintB, result.PriceOfAInB, result.PriceOfAInSOL, result.LiquidityInB, result.LiquidityInSOL, result.MarketCapInSOL, result.FDVInSOL, result.TotalSupply, result.CirculatingSupply, result.SupplyMethod, result.Metadata), nil
			})
		},
		{
			Dex:         DexRaydium,
			PoolVersion: PoolVersionRaydiumLiquidityV4,
		}: func(cfg Config) PoolCalculator {
			return poolCalculatorFunc(func(ctx context.Context, pool PoolIdentifier) (*GetMetricsByPoolResponse, error) {
				calculator := raydiumv4.NewCalculator(cfg.RPCClient, cfg.QuoteBridge, cfg.SupplyProvider)
				result, err := calculator.Compute(ctx, raydiumv4.Request{
					PoolAddress: pool.PoolAddress,
				})
				if err != nil {
					return nil, NewError(ErrCodeInternal, "raydium liquidity v4 metrics failed", err)
				}
				return buildMetricsResponse(pool, result.MintA, result.MintB, result.PriceOfAInB, result.PriceOfAInSOL, result.LiquidityInB, result.LiquidityInSOL, result.MarketCapInSOL, result.FDVInSOL, result.TotalSupply, result.CirculatingSupply, result.SupplyMethod, result.Metadata), nil
			})
		},
		{
			Dex:         DexRaydium,
			PoolVersion: PoolVersionRaydiumCPMM,
		}: func(cfg Config) PoolCalculator {
			return poolCalculatorFunc(func(ctx context.Context, pool PoolIdentifier) (*GetMetricsByPoolResponse, error) {
				calculator := raydiumcpmm.NewCalculator(cfg.RPCClient, cfg.QuoteBridge, cfg.SupplyProvider)
				result, err := calculator.Compute(ctx, raydiumcpmm.Request{
					PoolAddress: pool.PoolAddress,
				})
				if err != nil {
					return nil, NewError(ErrCodeInternal, "raydium cpmm metrics failed", err)
				}
				return buildMetricsResponse(pool, result.MintA, result.MintB, result.PriceOfAInB, result.PriceOfAInSOL, result.LiquidityInB, result.LiquidityInSOL, result.MarketCapInSOL, result.FDVInSOL, result.TotalSupply, result.CirculatingSupply, result.SupplyMethod, result.Metadata), nil
			})
		},
		{
			Dex:         DexRaydium,
			PoolVersion: PoolVersionRaydiumCLMM,
		}: func(cfg Config) PoolCalculator {
			return poolCalculatorFunc(func(ctx context.Context, pool PoolIdentifier) (*GetMetricsByPoolResponse, error) {
				calculator := raydiumclmm.NewCalculator(cfg.RPCClient, cfg.QuoteBridge, cfg.SupplyProvider)
				result, err := calculator.Compute(ctx, raydiumclmm.Request{
					PoolAddress: pool.PoolAddress,
				})
				if err != nil {
					return nil, NewError(ErrCodeInternal, "raydium clmm metrics failed", err)
				}
				return buildMetricsResponse(pool, result.MintA, result.MintB, result.PriceOfAInB, result.PriceOfAInSOL, result.LiquidityInB, result.LiquidityInSOL, result.MarketCapInSOL, result.FDVInSOL, result.TotalSupply, result.CirculatingSupply, result.SupplyMethod, result.Metadata), nil
			})
		},
		{
			Dex:         DexRaydium,
			PoolVersion: PoolVersionRaydiumLaunchpad,
		}: func(cfg Config) PoolCalculator {
			return poolCalculatorFunc(func(ctx context.Context, pool PoolIdentifier) (*GetMetricsByPoolResponse, error) {
				calculator := raydiumlaunchpad.NewCalculator(cfg.RPCClient, cfg.QuoteBridge, cfg.SupplyProvider)
				result, err := calculator.Compute(ctx, raydiumlaunchpad.Request{
					PoolAddress: pool.PoolAddress,
				})
				if err != nil {
					return nil, NewError(ErrCodeInternal, "raydium launchpad metrics failed", err)
				}
				return buildMetricsResponse(pool, result.MintA, result.MintB, result.PriceOfAInB, result.PriceOfAInSOL, result.LiquidityInB, result.LiquidityInSOL, result.MarketCapInSOL, result.FDVInSOL, result.TotalSupply, result.CirculatingSupply, result.SupplyMethod, result.Metadata), nil
			})
		},
		{
			Dex:         DexMeteora,
			PoolVersion: PoolVersionMeteoraDLMM,
		}: func(cfg Config) PoolCalculator {
			return poolCalculatorFunc(func(ctx context.Context, pool PoolIdentifier) (*GetMetricsByPoolResponse, error) {
				calculator := meteordlmm.NewCalculator(cfg.RPCClient, cfg.QuoteBridge, cfg.SupplyProvider)
				result, err := calculator.Compute(ctx, meteordlmm.Request{
					PoolAddress: pool.PoolAddress,
				})
				if err != nil {
					return nil, NewError(ErrCodeInternal, "meteora dlmm metrics failed", err)
				}
				return buildMetricsResponse(pool, result.MintA, result.MintB, result.PriceOfAInB, result.PriceOfAInSOL, result.LiquidityInB, result.LiquidityInSOL, result.MarketCapInSOL, result.FDVInSOL, result.TotalSupply, result.CirculatingSupply, result.SupplyMethod, result.Metadata), nil
			})
		},
		{
			Dex:         DexMeteora,
			PoolVersion: PoolVersionMeteoraDBC,
		}: func(cfg Config) PoolCalculator {
			return poolCalculatorFunc(func(ctx context.Context, pool PoolIdentifier) (*GetMetricsByPoolResponse, error) {
				calculator := meteordbc.NewCalculator(cfg.RPCClient, cfg.QuoteBridge, cfg.SupplyProvider)
				result, err := calculator.Compute(ctx, meteordbc.Request{
					PoolAddress: pool.PoolAddress,
				})
				if err != nil {
					return nil, NewError(ErrCodeInternal, "meteora dbc metrics failed", err)
				}
				return buildMetricsResponse(pool, result.MintA, result.MintB, result.PriceOfAInB, result.PriceOfAInSOL, result.LiquidityInB, result.LiquidityInSOL, result.MarketCapInSOL, result.FDVInSOL, result.TotalSupply, result.CirculatingSupply, result.SupplyMethod, result.Metadata), nil
			})
		},
		{
			Dex:         DexMeteora,
			PoolVersion: PoolVersionMeteoraDAMMV1,
		}: func(cfg Config) PoolCalculator {
			return poolCalculatorFunc(func(ctx context.Context, pool PoolIdentifier) (*GetMetricsByPoolResponse, error) {
				calculator := meteordammv1.NewCalculator(cfg.RPCClient, cfg.QuoteBridge, cfg.SupplyProvider)
				result, err := calculator.Compute(ctx, meteordammv1.Request{
					PoolAddress: pool.PoolAddress,
				})
				if err != nil {
					return nil, NewError(ErrCodeInternal, "meteora damm v1 metrics failed", err)
				}
				return buildMetricsResponse(pool, result.MintA, result.MintB, result.PriceOfAInB, result.PriceOfAInSOL, result.LiquidityInB, result.LiquidityInSOL, result.MarketCapInSOL, result.FDVInSOL, result.TotalSupply, result.CirculatingSupply, result.SupplyMethod, result.Metadata), nil
			})
		},
		{
			Dex:         DexMeteora,
			PoolVersion: PoolVersionMeteoraDAMMV2,
		}: func(cfg Config) PoolCalculator {
			return poolCalculatorFunc(func(ctx context.Context, pool PoolIdentifier) (*GetMetricsByPoolResponse, error) {
				calculator := meteordammv2.NewCalculator(cfg.RPCClient, cfg.QuoteBridge, cfg.SupplyProvider)
				result, err := calculator.Compute(ctx, meteordammv2.Request{
					PoolAddress: pool.PoolAddress,
				})
				if err != nil {
					return nil, NewError(ErrCodeInternal, "meteora damm v2 metrics failed", err)
				}
				return buildMetricsResponse(pool, result.MintA, result.MintB, result.PriceOfAInB, result.PriceOfAInSOL, result.LiquidityInB, result.LiquidityInSOL, result.MarketCapInSOL, result.FDVInSOL, result.TotalSupply, result.CirculatingSupply, result.SupplyMethod, result.Metadata), nil
			})
		},
		{
			Dex:         DexOrca,
			PoolVersion: PoolVersionOrcaWhirlpool,
		}: func(cfg Config) PoolCalculator {
			return poolCalculatorFunc(func(ctx context.Context, pool PoolIdentifier) (*GetMetricsByPoolResponse, error) {
				calculator := orcawhirlpool.NewCalculator(cfg.RPCClient, cfg.QuoteBridge, cfg.SupplyProvider)
				result, err := calculator.Compute(ctx, orcawhirlpool.Request{
					PoolAddress: pool.PoolAddress,
				})
				if err != nil {
					return nil, NewError(ErrCodeInternal, "orca whirlpool metrics failed", err)
				}
				return buildMetricsResponse(pool, result.MintA, result.MintB, result.PriceOfAInB, result.PriceOfAInSOL, result.LiquidityInB, result.LiquidityInSOL, result.MarketCapInSOL, result.FDVInSOL, result.TotalSupply, result.CirculatingSupply, result.SupplyMethod, result.Metadata), nil
			})
		},
	}
}

func buildMetricsResponse(
	pool PoolIdentifier,
	mintA solana.PublicKey,
	mintB solana.PublicKey,
	priceOfAInB Decimal,
	priceOfAInSOL Decimal,
	liquidityInB Decimal,
	liquidityInSOL Decimal,
	marketCapInSOL Decimal,
	fdvInSOL Decimal,
	totalSupply Decimal,
	circulatingSupply Decimal,
	supplyMethod string,
	metadata map[string]any,
) *GetMetricsByPoolResponse {
	return &GetMetricsByPoolResponse{
		Pool:              pool,
		MintA:             mintA,
		MintB:             mintB,
		PriceOfAInB:       priceOfAInB,
		PriceOfAInSOL:     priceOfAInSOL,
		LiquidityInB:      liquidityInB,
		LiquidityInSOL:    liquidityInSOL,
		MarketCapInSOL:    marketCapInSOL,
		FDVInSOL:          fdvInSOL,
		TotalSupply:       totalSupply,
		CirculatingSupply: circulatingSupply,
		SupplyMethod:      supplyMethod,
		Metadata:          metadata,
	}
}
