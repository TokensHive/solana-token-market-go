package examplecli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/TokensHive/solana-token-market-go/sdk/discovery"
	"github.com/TokensHive/solana-token-market-go/sdk/market"
	"github.com/TokensHive/solana-token-market-go/sdk/parser"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/gagliardetto/solana-go"
)

var SampleMints = []string{
	"3y6kjbdG3ULceQMuJh3RWz68bGoEZ3U1YBeJyXbJpump",
	"9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump",
	"Dfh5DzRgSvvCFDoYc2ciTkMrbDfRKybA4SoFbPmApump",
	"3z2tRjNuQjoq6UDcw4zyEPD1Eb5KXMPYb4GWFzVT1DPg",
	"6xox4cWVMpzLbCXpoyBUjH5KxKCAQMV8bCoaEqH7PArA",
	"2bpT3ksMdwdZ6DuHyq3FDUr7HDwvZ5DRZoT1fUPALJaH",
	"HBoNJ5v8g71s2boRivrHnfSB5MVPLDHHyVjruPfhGkvL",
	"1zJX5gRnjLgmTpq5sVwkq69mNDQkCemqoasyjaPW6jm",
	"5552z6Qp2xr596ox1UVN4ppDwwyjCfY8cXwzHMXgMcaS",
	"2zMMhcVQEXDtdE6vsFS7S7D5oUodfJHE8vd1gnBouauv",
}

type Runner struct {
	client *market.Client
	debug  bool
}

func NewRunner(rpcURL string, debug bool) (*Runner, error) {
	rpcURL = strings.TrimSpace(rpcURL)
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}
	rpcClient := rpc.NewSolanaRPCClient(rpcURL)
	engine := discovery.NewEngine(rpcClient, parser.NewNoopAdapter())
	client, err := market.NewClient(
		market.WithRPCClient(rpcClient),
		market.WithParserAdapter(parser.NewNoopAdapter()),
		market.WithDiscoveryEngine(engine),
		market.WithAPIHints(true, false),
		market.WithDebugRequests(debug),
	)
	if err != nil {
		return nil, err
	}
	return &Runner{client: client, debug: debug}, nil
}

// callEntry holds telemetry for a single SDK method call within RunAllPublicMethods.
type callEntry struct {
	name       string
	status     string // "PASS", "FAIL", or "SKIP"
	durationMS int64
	rpcTotal   int
	apiTotal   int
}

func skipEntry(name string) callEntry { return callEntry{name: name, status: "SKIP"} }

// printDebugMap formats and prints a single debug line for the given label and
// debug map returned by LastRequestDebug.
func printDebugMap(label string, debug map[string]any) {
	if len(debug) == 0 {
		fmt.Printf("[debug] %s: no request stats\n", label)
		return
	}
	ms, _ := debug["duration_ms"].(int64)
	fmt.Printf("[debug] %s: total=%dms | rpc=%s | api=%s\n",
		label, ms, formatRPCDebug(debug), formatAPIDebug(debug))
}

func (r *Runner) printDebug(label string) {
	if !r.debug {
		return
	}
	printDebugMap(label, r.client.LastRequestDebug())
}

// captureCallStats reads LastRequestDebug, prints a formatted debug line, and
// returns a callEntry.  ok indicates whether the SDK call succeeded.
func (r *Runner) captureCallStats(name string, ok bool) callEntry {
	status := "PASS"
	if !ok {
		status = "FAIL"
	}
	e := callEntry{name: name, status: status}
	if !r.debug {
		return e
	}
	debug := r.client.LastRequestDebug()
	printDebugMap(name, debug)
	if len(debug) == 0 {
		return e
	}
	if ms, _ := debug["duration_ms"].(int64); ms > 0 {
		e.durationMS = ms
	}
	if rpc, _ := debug["rpc"].(map[string]any); rpc != nil {
		if t, _ := rpc["total"].(int); t > 0 {
			e.rpcTotal = t
		}
	}
	if api, _ := debug["api"].(map[string]any); api != nil {
		if t, _ := api["total"].(int); t > 0 {
			e.apiTotal = t
		}
	}
	return e
}

// printTelemetryBlock emits a machine-parseable TELEMETRY block to stdout so
// the CI benchmark script can extract per-call and aggregate metrics.
func (r *Runner) printTelemetryBlock(calls []callEntry) {
	if !r.debug {
		return
	}
	var totalMS int64
	totalRPC := 0
	totalAPI := 0
	fmt.Println("TELEMETRY_BEGIN")
	for _, e := range calls {
		fmt.Printf("op=%s status=%s duration_ms=%d rpc=%d api=%d\n",
			e.name, e.status, e.durationMS, e.rpcTotal, e.apiTotal)
		totalMS += e.durationMS
		totalRPC += e.rpcTotal
		totalAPI += e.apiTotal
	}
	fmt.Printf("TELEMETRY_TOTAL duration_ms=%d rpc=%d api=%d\n", totalMS, totalRPC, totalAPI)
	fmt.Println("TELEMETRY_END")
}

func formatRPCDebug(debug map[string]any) string {
	rpc, _ := debug["rpc"].(map[string]any)
	if rpc == nil {
		return "0"
	}
	total, _ := rpc["total"].(int)
	if total == 0 {
		return "0"
	}
	byType, _ := rpc["by_operation_type"].(map[string]int)
	durByType, _ := rpc["duration_by_type_ms"].(map[string]int64)
	if bd := formatBreakdown(byType, durByType); bd != "" {
		return fmt.Sprintf("%d (%s)", total, bd)
	}
	return fmt.Sprintf("%d", total)
}

func formatAPIDebug(debug map[string]any) string {
	api, _ := debug["api"].(map[string]any)
	if api == nil {
		return "0"
	}
	total, _ := api["total"].(int)
	if total == 0 {
		return "0"
	}
	bySource, _ := api["by_source"].(map[string]int)
	durBySource, _ := api["duration_by_source_ms"].(map[string]int64)
	if bd := formatBreakdown(bySource, durBySource); bd != "" {
		return fmt.Sprintf("%d (%s)", total, bd)
	}
	return fmt.Sprintf("%d", total)
}

// formatBreakdown renders a sorted "key:count@durationms" breakdown string.
func formatBreakdown(byKey map[string]int, durByKey map[string]int64) string {
	if len(byKey) == 0 {
		return ""
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d@%dms", k, byKey[k], durByKey[k]))
	}
	return strings.Join(parts, ",")
}

func ParsePublicKey(value string) (solana.PublicKey, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return solana.PublicKey{}, errors.New("value is required")
	}
	pk, err := solana.PublicKeyFromBase58(value)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("invalid base58 public key %q: %w", value, err)
	}
	return pk, nil
}

func ParseProtocol(value string) (market.Protocol, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	protocol := market.Protocol(value)
	switch protocol {
	case market.ProtocolPumpfun,
		market.ProtocolPumpswap,
		market.ProtocolRaydiumV4,
		market.ProtocolRaydiumCPMM,
		market.ProtocolRaydiumCLMM,
		market.ProtocolRaydiumLaunchLab,
		market.ProtocolOrcaWhirlpool,
		market.ProtocolMeteoraDLMM,
		market.ProtocolMeteoraDAMM,
		market.ProtocolMeteoraDBC:
		return protocol, nil
	default:
		return "", fmt.Errorf("unsupported protocol %q", value)
	}
}

func (r *Runner) ResolvePools(ctx context.Context, mintStr string) error {
	mint, err := ParsePublicKey(mintStr)
	if err != nil {
		return err
	}
	resp, err := r.client.ResolvePools(ctx, market.ResolvePoolsRequest{Mint: mint, IncludeUnverified: true, SelectPrimary: true})
	r.printDebug("ResolvePools")
	if err != nil {
		return err
	}
	fmt.Printf("mint=%s pools=%d primary=%v mode=%s\n", resp.Mint, len(resp.Pools), resp.PrimaryPool != nil, resp.Metadata.DiscoveryMode)
	if resp.PrimaryPool != nil {
		fmt.Printf("primaryPool=%s protocol=%s pair=%s/%s priceSOL=%s liqSOL=%s\n", resp.PrimaryPool.Address, resp.PrimaryPool.Protocol, resp.PrimaryPool.BaseMint, resp.PrimaryPool.QuoteMint, resp.PrimaryPool.PriceOfTokenInSOL, resp.PrimaryPool.LiquidityInSOL)
	}
	return nil
}

func (r *Runner) GetPool(ctx context.Context, poolStr string) error {
	pool, err := ParsePublicKey(poolStr)
	if err != nil {
		return err
	}
	resp, err := r.client.GetPool(ctx, market.GetPoolRequest{PoolAddress: pool})
	r.printDebug("GetPool")
	if err != nil {
		return err
	}
	fmt.Printf("pool=%s protocol=%s base=%s quote=%s priceSOL=%s liqSOL=%s\n", resp.Address, resp.Protocol, resp.BaseMint, resp.QuoteMint, resp.PriceOfTokenInSOL, resp.LiquidityInSOL)
	return nil
}

func (r *Runner) GetTokenMarket(ctx context.Context, mintStr string) error {
	mint, err := ParsePublicKey(mintStr)
	if err != nil {
		return err
	}
	resp, err := r.client.GetTokenMarket(ctx, market.GetTokenMarketRequest{Mint: mint, IncludeUnverified: true})
	r.printDebug("GetTokenMarket")
	if err != nil {
		return err
	}
	fmt.Printf("mint=%s priceSOL=%s liqSOL=%s marketCapSOL=%s pools=%d\n", resp.Mint, resp.PriceInSOL, resp.LiquidityInSOL, resp.MarketCapInSOL, len(resp.Pools))
	if resp.PrimaryPool != nil {
		fmt.Printf("primaryPool=%s protocol=%s pair=%s/%s priceSOL=%s\n", resp.PrimaryPool.Address, resp.PrimaryPool.Protocol, resp.PrimaryPool.BaseMint, resp.PrimaryPool.QuoteMint, resp.PrimaryPool.PriceOfTokenInSOL)
	}
	return nil
}

func (r *Runner) RunAllPublicMethods(ctx context.Context, mintStr string, protocolStr string) error {
	mint, err := ParsePublicKey(mintStr)
	if err != nil {
		return err
	}
	fmt.Printf("== Running public market methods for mint=%s ==\n", mint.String())

	var calls []callEntry

	resolved, resolvePoolsErr := r.client.ResolvePools(ctx, market.ResolvePoolsRequest{Mint: mint, IncludeUnverified: true, SelectPrimary: true})
	calls = append(calls, r.captureCallStats("ResolvePools", resolvePoolsErr == nil))
	if resolvePoolsErr != nil {
		fmt.Printf("ResolvePools: error=%v\n", resolvePoolsErr)
	} else {
		fmt.Printf("ResolvePools: pools=%d primary=%v\n", len(resolved.Pools), resolved.PrimaryPool != nil)
		if resolved.PrimaryPool != nil {
			fmt.Printf("ResolvePools primary: pair=%s/%s protocol=%s priceSOL=%s\n", resolved.PrimaryPool.BaseMint, resolved.PrimaryPool.QuoteMint, resolved.PrimaryPool.Protocol, resolved.PrimaryPool.PriceOfTokenInSOL)
		}
	}

	poolsByMint, byMintErr := r.client.FindPoolsByMint(ctx, mint)
	calls = append(calls, r.captureCallStats("FindPoolsByMint", byMintErr == nil))
	if byMintErr != nil {
		fmt.Printf("FindPoolsByMint: error=%v\n", byMintErr)
	} else {
		fmt.Printf("FindPoolsByMint: pools=%d\n", len(poolsByMint))
	}

	marketResp, tokenMarketErr := r.client.GetTokenMarket(ctx, market.GetTokenMarketRequest{Mint: mint, IncludeUnverified: true})
	calls = append(calls, r.captureCallStats("GetTokenMarket", tokenMarketErr == nil))
	if tokenMarketErr != nil {
		fmt.Printf("GetTokenMarket: error=%v\n", tokenMarketErr)
	} else {
		fmt.Printf("GetTokenMarket: priceSOL=%s liqSOL=%s marketCapSOL=%s\n", marketResp.PriceInSOL, marketResp.LiquidityInSOL, marketResp.MarketCapInSOL)
	}

	var pool *market.Pool
	if resolved != nil && resolved.PrimaryPool != nil {
		pool = resolved.PrimaryPool
	}
	if pool == nil && resolved != nil && len(resolved.Pools) > 0 {
		poolKey, parseErr := ParsePublicKey(resolved.Pools[0].Address)
		if parseErr != nil {
			fmt.Printf("GetPool: parse error=%v\n", parseErr)
			calls = append(calls, skipEntry("GetPool"))
		} else {
			gotPool, getPoolErr := r.client.GetPool(ctx, market.GetPoolRequest{PoolAddress: poolKey})
			calls = append(calls, r.captureCallStats("GetPool", getPoolErr == nil))
			if getPoolErr != nil {
				fmt.Printf("GetPool: error=%v\n", getPoolErr)
			} else {
				pool = gotPool
				fmt.Printf("GetPool: address=%s protocol=%s\n", gotPool.Address, gotPool.Protocol)
			}
		}
	} else if pool == nil {
		fmt.Println("GetPool: skipped (no resolved pools)")
		calls = append(calls, skipEntry("GetPool"))
	} else {
		fmt.Printf("GetPool: skipped (using resolved primary pool %s)\n", pool.Address)
		calls = append(calls, skipEntry("GetPool"))
	}

	if pool != nil {
		baseMint, baseErr := ParsePublicKey(pool.BaseMint)
		quoteMint, quoteErr := ParsePublicKey(pool.QuoteMint)
		if baseErr == nil && quoteErr == nil {
			byPair, pairErr := r.client.FindPoolsByPair(ctx, baseMint, quoteMint)
			calls = append(calls, r.captureCallStats("FindPoolsByPair", pairErr == nil))
			if pairErr != nil {
				fmt.Printf("FindPoolsByPair: error=%v\n", pairErr)
			} else {
				fmt.Printf("FindPoolsByPair: pools=%d\n", len(byPair))
			}
		} else {
			fmt.Printf("FindPoolsByPair: skipped (base parse err=%v quote parse err=%v)\n", baseErr, quoteErr)
			calls = append(calls, skipEntry("FindPoolsByPair"))
		}
	} else {
		fmt.Println("FindPoolsByPair: skipped (no pool)")
		calls = append(calls, skipEntry("FindPoolsByPair"))
	}

	protocol, protocolErr := ParseProtocol(protocolStr)
	if protocolErr != nil {
		return protocolErr
	}
	if protocol == "" {
		if pool != nil {
			protocol = pool.Protocol
		} else {
			protocol = market.ProtocolPumpfun
		}
	}
	byProtocol, byProtocolErr := r.client.FindPoolsByProtocol(ctx, mint, protocol)
	calls = append(calls, r.captureCallStats("FindPoolsByProtocol", byProtocolErr == nil))
	if byProtocolErr != nil {
		fmt.Printf("FindPoolsByProtocol(%s): error=%v\n", protocol, byProtocolErr)
	} else {
		fmt.Printf("FindPoolsByProtocol(%s): pools=%d\n", protocol, len(byProtocol))
	}

	if pool != nil && marketResp != nil {
		metricsFromPool, metricsErr := r.client.ComputePoolMetrics(ctx, pool, marketResp.TotalSupply, marketResp.CirculatingSupply)
		calls = append(calls, r.captureCallStats("ComputePoolMetrics", metricsErr == nil))
		if metricsErr != nil {
			fmt.Printf("ComputePoolMetrics: error=%v\n", metricsErr)
		} else {
			fmt.Printf("ComputePoolMetrics: marketCapSOL=%s\n", metricsFromPool.MarketCapInSOL)
		}
	} else {
		fmt.Println("ComputePoolMetrics: skipped (missing pool or market response)")
		calls = append(calls, skipEntry("ComputePoolMetrics"))
	}

	if pool != nil {
		tokenMetrics, tokenMetricsErr := r.client.ComputeTokenMetricsFromPool(ctx, mint, pool)
		calls = append(calls, r.captureCallStats("ComputeTokenMetricsFromPool", tokenMetricsErr == nil))
		if tokenMetricsErr != nil {
			fmt.Printf("ComputeTokenMetricsFromPool: error=%v\n", tokenMetricsErr)
		} else {
			fmt.Printf("ComputeTokenMetricsFromPool: marketCapSOL=%s\n", tokenMetrics.MarketCapInSOL)
		}
	} else {
		fmt.Println("ComputeTokenMetricsFromPool: skipped (no pool)")
		calls = append(calls, skipEntry("ComputeTokenMetricsFromPool"))
	}

	if resolved != nil {
		selected, selectedErr := market.SelectPrimaryPool(resolved.Pools, mint.String(), nil)
		if selectedErr != nil {
			fmt.Printf("SelectPrimaryPool: error=%v\n", selectedErr)
		} else {
			fmt.Printf("SelectPrimaryPool: address=%s protocol=%s\n", selected.Address, selected.Protocol)
		}
	} else {
		fmt.Println("SelectPrimaryPool: skipped (resolve failed)")
	}

	r.printTelemetryBlock(calls)

	if resolvePoolsErr != nil && byMintErr != nil && tokenMarketErr != nil {
		return fmt.Errorf(
			"core discovery methods failed (ResolvePools=%v, FindPoolsByMint=%v, GetTokenMarket=%v); check RPC endpoint and token activity",
			resolvePoolsErr,
			byMintErr,
			tokenMarketErr,
		)
	}
	fmt.Println("== Completed public market methods demo ==")
	return nil
}
