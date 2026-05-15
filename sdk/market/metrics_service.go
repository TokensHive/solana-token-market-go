package market

import (
	"context"
	"errors"
	"fmt"
	"sync"

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
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/TokensHive/solana-token-market-go/sdk/supply"
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

func (c *Client) GetMetricsByPools(ctx context.Context, req GetMetricsByPoolsRequest) (*GetMetricsByPoolsResponse, error) {
	ctx, recorder := c.startDebug(ctx, "GetMetricsByPools")
	defer c.finishDebug(recorder)
	return c.service.GetMetricsByPools(ctx, req)
}

func (c *Client) GetMetricsByPumpfunBondingCurves(ctx context.Context, req GetMetricsByPumpfunBondingCurvesRequest) (*GetMetricsByPumpfunBondingCurvesResponse, error) {
	ctx, recorder := c.startDebug(ctx, "GetMetricsByPumpfunBondingCurves")
	defer c.finishDebug(recorder)
	return c.service.GetMetricsByPumpfunBondingCurves(ctx, req)
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
		return nil, NewError(ErrCodeUnsupportedRoute, fmt.Sprintf("unsupported pool route: %s/%s", req.Pool.Dex, req.Pool.PoolVersion), nil)
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
		return nil, NewError(classifyErrorCode(err), "derive pumpfun bonding curve address", err)
	}

	calculator := pumpcurve.NewCalculator(s.cfg.RPCClient, s.cfg.QuoteBridge)
	result, err := calculator.Compute(ctx, pumpcurve.Request{
		PoolAddress: poolAddress,
		MintA:       req.MintA,
		MintB:       req.MintB,
	})
	if err != nil {
		return nil, NewError(classifyErrorCode(err), "pumpfun bonding curve metrics failed", err)
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

func (s *Service) GetMetricsByPools(ctx context.Context, req GetMetricsByPoolsRequest) (*GetMetricsByPoolsResponse, error) {
	if len(req.Pools) == 0 {
		return &GetMetricsByPoolsResponse{Results: []GetMetricsByPoolItemResult{}}, nil
	}

	results := make([]GetMetricsByPoolItemResult, len(req.Pools))
	for i := range req.Pools {
		results[i] = GetMetricsByPoolItemResult{Pool: req.Pools[i]}
	}

	maxConcurrency, itemCtx := s.resolveBulkExecution(req.MaxConcurrency, req.ChunkSize, len(req.Pools), ctx)
	grouped := make(map[PoolRoute][]int, len(req.Pools))
	for i, pool := range req.Pools {
		if err := validateMetricsRequest(GetMetricsByPoolRequest{Pool: pool}); err != nil {
			results[i].Error = toSDKError(err)
			continue
		}
		if pool.Dex == DexPumpfun && pool.PoolVersion == PoolVersionPumpfunBondingCurve {
			results[i].Error = toSDKError(NewError(
				ErrCodeInvalidArgument,
				"pumpfun bonding_curve requires GetMetricsByPumpfunBondingCurve (mint-based) instead of GetMetricsByPool",
				nil,
			))
			continue
		}
		route := PoolRoute{Dex: pool.Dex, PoolVersion: pool.PoolVersion}
		if _, ok := s.calculators[route]; !ok {
			results[i].Error = toSDKError(NewError(
				ErrCodeUnsupportedRoute,
				fmt.Sprintf("unsupported pool route: %s/%s", pool.Dex, pool.PoolVersion),
				nil,
			))
			continue
		}
		grouped[route] = append(grouped[route], i)
	}

	var groups []PoolRoute
	for route := range grouped {
		groups = append(groups, route)
	}
	if len(groups) == 0 {
		s.annotateBulkDebug(itemCtx, "GetMetricsByPools", nil, results, rpc.PreloadStats{})
		return &GetMetricsByPoolsResponse{Results: results}, nil
	}
	if maxConcurrency > len(groups) {
		maxConcurrency = len(groups)
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var resultMu sync.Mutex
	totalStats := rpc.PreloadStats{}
	groupNames := make([]string, len(groups))

	for groupIndex := range groups {
		route := groups[groupIndex]
		groupNames[groupIndex] = fmt.Sprintf("%s/%s", route.Dex, route.PoolVersion)
		indices := grouped[route]

		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			preloaded := rpc.NewPreloadedClient(s.cfg.RPCClient)
			addresses := make([]solana.PublicKey, len(indices))
			for i, index := range indices {
				addresses[i] = req.Pools[index].PoolAddress
			}
			if err := preloaded.PrimeAccounts(itemCtx, addresses); err != nil {
				for _, index := range indices {
					resultMu.Lock()
					results[index].Error = toSDKError(err)
					resultMu.Unlock()
				}
				resultMu.Lock()
				stats := preloaded.Stats()
				totalStats.GetAccountCalls += stats.GetAccountCalls
				totalStats.GetMultipleAccountsCalls += stats.GetMultipleAccountsCalls
				totalStats.TotalAccountsRequested += stats.TotalAccountsRequested
				resultMu.Unlock()
				return
			}

			groupService := s.withRPCClient(preloaded)
			computeService := groupService
			if !s.canRebuildRoute(route) {
				computeService = s
			}
			for _, index := range indices {
				pool := req.Pools[index]
				metrics, err := computeService.GetMetricsByPool(itemCtx, GetMetricsByPoolRequest{Pool: pool})
				resultMu.Lock()
				if err != nil {
					results[index].Error = toSDKError(err)
				} else {
					results[index].Metrics = metrics
				}
				resultMu.Unlock()
			}

			resultMu.Lock()
			stats := preloaded.Stats()
			totalStats.GetAccountCalls += stats.GetAccountCalls
			totalStats.GetMultipleAccountsCalls += stats.GetMultipleAccountsCalls
			totalStats.TotalAccountsRequested += stats.TotalAccountsRequested
			resultMu.Unlock()
		}()
	}

	wg.Wait()
	s.annotateBulkDebug(itemCtx, "GetMetricsByPools", groupNames, results, totalStats)
	return &GetMetricsByPoolsResponse{Results: results}, nil
}

func (s *Service) GetMetricsByPumpfunBondingCurves(ctx context.Context, req GetMetricsByPumpfunBondingCurvesRequest) (*GetMetricsByPumpfunBondingCurvesResponse, error) {
	if len(req.Items) == 0 {
		return &GetMetricsByPumpfunBondingCurvesResponse{Results: []GetMetricsByPumpfunBondingCurveItemResult{}}, nil
	}

	_, itemCtx := s.resolveBulkExecution(req.MaxConcurrency, req.ChunkSize, len(req.Items), ctx)
	results := make([]GetMetricsByPumpfunBondingCurveItemResult, len(req.Items))
	validIndices := make([]int, 0, len(req.Items))
	validAddresses := make([]solana.PublicKey, 0, len(req.Items))

	for i := range req.Items {
		item := req.Items[i]
		results[i] = GetMetricsByPumpfunBondingCurveItemResult{Item: item}
		if err := validatePumpfunBondingCurveBulkItem(item); err != nil {
			results[i].Error = toSDKError(err)
			continue
		}
		poolAddress, _, err := findProgramAddress([][]byte{
			[]byte(pumpfunBondingCurveSeed),
			item.TokenMint.Bytes(),
		}, pumpfunProgramID)
		if err != nil {
			results[i].Error = toSDKError(NewError(classifyErrorCode(err), "derive pumpfun bonding curve address", err))
			continue
		}
		validIndices = append(validIndices, i)
		validAddresses = append(validAddresses, poolAddress)
	}

	preloaded := rpc.NewPreloadedClient(s.cfg.RPCClient)
	if len(validAddresses) > 0 {
		if err := preloaded.PrimeAccounts(itemCtx, validAddresses); err != nil {
			for _, index := range validIndices {
				results[index].Error = toSDKError(err)
			}
		}
	}
	groupService := s.withRPCClient(preloaded)
	if len(validAddresses) > 0 {
		for _, index := range validIndices {
			if results[index].Error != nil {
				continue
			}
			item := req.Items[index]
			metrics, err := groupService.GetMetricsByPumpfunBondingCurve(itemCtx, GetMetricsByPumpfunBondingCurveRequest{
				MintA: item.TokenMint,
				MintB: solana.SolMint,
			})
			if err != nil {
				results[index].Error = toSDKError(err)
			} else {
				results[index].Metrics = metrics
			}
		}
	}
	s.annotateBulkBondingDebug(itemCtx, results, preloaded.Stats())
	return &GetMetricsByPumpfunBondingCurvesResponse{Results: results}, nil
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

func validatePumpfunBondingCurveBulkItem(item GetMetricsByPumpfunBondingCurveItem) error {
	if item.TokenMint.IsZero() {
		return NewError(ErrCodeInvalidArgument, "token mint is required", nil)
	}
	if solana.SolMint.Equals(item.TokenMint) {
		return NewError(ErrCodeInvalidArgument, "token mint cannot be SOL", nil)
	}
	return nil
}

func toSDKError(err error) *SDKError {
	if err == nil {
		return nil
	}
	var sdkErr *SDKError
	if errors.As(err, &sdkErr) {
		if sdkErr.Code == ErrCodeInternal && sdkErr.Err != nil {
			return &SDKError{
				Code:    classifyErrorCode(sdkErr.Err),
				Message: sdkErr.Message,
				Err:     sdkErr.Err,
			}
		}
		return sdkErr
	}
	return &SDKError{
		Code:    classifyErrorCode(err),
		Message: err.Error(),
		Err:     err,
	}
}

func (s *Service) resolveBulkExecution(maxConcurrency, chunkSize, totalItems int, ctx context.Context) (int, context.Context) {
	if maxConcurrency <= 0 {
		maxConcurrency = s.cfg.MaxBulkConcurrency
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}
	if maxConcurrency > totalItems {
		maxConcurrency = totalItems
	}
	itemCtx := ctx
	if chunkSize <= 0 {
		chunkSize = s.cfg.BulkChunkSize
	}
	if chunkSize > 0 {
		itemCtx = rpc.WithGetMultipleAccountsChunkSize(itemCtx, chunkSize)
	}
	return maxConcurrency, itemCtx
}

func (s *Service) withRPCClient(client rpc.Client) *Service {
	cfg := s.cfg
	cfg.RPCClient = client
	if _, ok := s.cfg.SupplyProvider.(*supply.DefaultProvider); ok {
		cfg.SupplyProvider = supply.NewDefaultProvider(client)
	}
	return NewService(cfg)
}

func (s *Service) canRebuildRoute(route PoolRoute) bool {
	if _, ok := s.cfg.PoolCalculatorFactories[route]; ok {
		return true
	}
	if _, ok := defaultPoolCalculatorFactories()[route]; ok {
		return true
	}
	return false
}

func (s *Service) annotateBulkDebug(ctx context.Context, operation string, groups []string, results []GetMetricsByPoolItemResult, stats rpc.PreloadStats) {
	failedByCode := map[string]int{}
	for i := range results {
		if results[i].Error == nil {
			continue
		}
		failedByCode[string(results[i].Error.Code)]++
	}
	annotateRequestDebug(ctx, map[string]any{
		"bulk_operation":              operation,
		"protocol_groups":             groups,
		"failed_item_reasons":         failedByCode,
		"get_multiple_accounts_calls": stats.GetMultipleAccountsCalls,
		"get_account_calls":           stats.GetAccountCalls,
		"total_accounts_requested":    stats.TotalAccountsRequested,
	})
}

func (s *Service) annotateBulkBondingDebug(ctx context.Context, results []GetMetricsByPumpfunBondingCurveItemResult, stats rpc.PreloadStats) {
	failedByCode := map[string]int{}
	for i := range results {
		if results[i].Error == nil {
			continue
		}
		failedByCode[string(results[i].Error.Code)]++
	}
	annotateRequestDebug(ctx, map[string]any{
		"bulk_operation":              "GetMetricsByPumpfunBondingCurves",
		"protocol_groups":             []string{fmt.Sprintf("%s/%s", DexPumpfun, PoolVersionPumpfunBondingCurve)},
		"failed_item_reasons":         failedByCode,
		"get_multiple_accounts_calls": stats.GetMultipleAccountsCalls,
		"get_account_calls":           stats.GetAccountCalls,
		"total_accounts_requested":    stats.TotalAccountsRequested,
	})
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
					return nil, NewError(classifyErrorCode(err), "pumpfun amm metrics failed", err)
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
					return nil, NewError(classifyErrorCode(err), "raydium liquidity v4 metrics failed", err)
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
					return nil, NewError(classifyErrorCode(err), "raydium cpmm metrics failed", err)
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
					return nil, NewError(classifyErrorCode(err), "raydium clmm metrics failed", err)
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
					return nil, NewError(classifyErrorCode(err), "raydium launchpad metrics failed", err)
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
					return nil, NewError(classifyErrorCode(err), "meteora dlmm metrics failed", err)
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
					return nil, NewError(classifyErrorCode(err), "meteora dbc metrics failed", err)
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
					return nil, NewError(classifyErrorCode(err), "meteora damm v1 metrics failed", err)
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
					return nil, NewError(classifyErrorCode(err), "meteora damm v2 metrics failed", err)
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
					return nil, NewError(classifyErrorCode(err), "orca whirlpool metrics failed", err)
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
