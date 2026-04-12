package examplecli

import (
	"context"
	"errors"
	"fmt"
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
}

func NewRunner(rpcURL string) (*Runner, error) {
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
	)
	if err != nil {
		return nil, err
	}
	return &Runner{client: client}, nil
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
	if err != nil {
		return err
	}
	fmt.Printf("mint=%s pools=%d primary=%v mode=%s\n", resp.Mint, len(resp.Pools), resp.PrimaryPool != nil, resp.Metadata.DiscoveryMode)
	if resp.PrimaryPool != nil {
		fmt.Printf("primaryPool=%s protocol=%s liqSOL=%s\n", resp.PrimaryPool.Address, resp.PrimaryPool.Protocol, resp.PrimaryPool.LiquidityInSOL)
	}
	return nil
}

func (r *Runner) GetPool(ctx context.Context, poolStr string) error {
	pool, err := ParsePublicKey(poolStr)
	if err != nil {
		return err
	}
	resp, err := r.client.GetPool(ctx, market.GetPoolRequest{PoolAddress: pool})
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
	if err != nil {
		return err
	}
	fmt.Printf("mint=%s priceSOL=%s liqSOL=%s marketCapSOL=%s pools=%d\n", resp.Mint, resp.PriceInSOL, resp.LiquidityInSOL, resp.MarketCapInSOL, len(resp.Pools))
	if resp.PrimaryPool != nil {
		fmt.Printf("primaryPool=%s protocol=%s\n", resp.PrimaryPool.Address, resp.PrimaryPool.Protocol)
	}
	return nil
}

func (r *Runner) RunAllPublicMethods(ctx context.Context, mintStr string, protocolStr string) error {
	mint, err := ParsePublicKey(mintStr)
	if err != nil {
		return err
	}
	fmt.Printf("== Running public market methods for mint=%s ==\n", mint.String())
	resolved, resolvePoolsErr := r.client.ResolvePools(ctx, market.ResolvePoolsRequest{Mint: mint, IncludeUnverified: true, SelectPrimary: true})
	if resolvePoolsErr != nil {
		fmt.Printf("ResolvePools: error=%v\n", resolvePoolsErr)
	} else {
		fmt.Printf("ResolvePools: pools=%d primary=%v\n", len(resolved.Pools), resolved.PrimaryPool != nil)
	}

	poolsByMint, byMintErr := r.client.FindPoolsByMint(ctx, mint)
	if byMintErr != nil {
		fmt.Printf("FindPoolsByMint: error=%v\n", byMintErr)
	} else {
		fmt.Printf("FindPoolsByMint: pools=%d\n", len(poolsByMint))
	}

	marketResp, tokenMarketErr := r.client.GetTokenMarket(ctx, market.GetTokenMarketRequest{Mint: mint, IncludeUnverified: true})
	if tokenMarketErr != nil {
		fmt.Printf("GetTokenMarket: error=%v\n", tokenMarketErr)
	} else {
		fmt.Printf("GetTokenMarket: priceSOL=%s liqSOL=%s marketCapSOL=%s\n", marketResp.PriceInSOL, marketResp.LiquidityInSOL, marketResp.MarketCapInSOL)
	}

	var pool *market.Pool
	if resolved != nil && len(resolved.Pools) > 0 {
		poolKey, parseErr := ParsePublicKey(resolved.Pools[0].Address)
		if parseErr != nil {
			fmt.Printf("GetPool: parse error=%v\n", parseErr)
		} else {
			gotPool, getPoolErr := r.client.GetPool(ctx, market.GetPoolRequest{PoolAddress: poolKey})
			if getPoolErr != nil {
				fmt.Printf("GetPool: error=%v\n", getPoolErr)
			} else {
				pool = gotPool
				fmt.Printf("GetPool: address=%s protocol=%s\n", pool.Address, pool.Protocol)
			}
		}
	} else {
		fmt.Println("GetPool: skipped (no resolved pools)")
	}

	if pool != nil {
		baseMint, baseErr := ParsePublicKey(pool.BaseMint)
		quoteMint, quoteErr := ParsePublicKey(pool.QuoteMint)
		if baseErr == nil && quoteErr == nil {
			byPair, pairErr := r.client.FindPoolsByPair(ctx, baseMint, quoteMint)
			if pairErr != nil {
				fmt.Printf("FindPoolsByPair: error=%v\n", pairErr)
			} else {
				fmt.Printf("FindPoolsByPair: pools=%d\n", len(byPair))
			}
		} else {
			fmt.Printf("FindPoolsByPair: skipped (base parse err=%v quote parse err=%v)\n", baseErr, quoteErr)
		}
	} else {
		fmt.Println("FindPoolsByPair: skipped (no pool)")
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
	if byProtocolErr != nil {
		fmt.Printf("FindPoolsByProtocol(%s): error=%v\n", protocol, byProtocolErr)
	} else {
		fmt.Printf("FindPoolsByProtocol(%s): pools=%d\n", protocol, len(byProtocol))
	}

	if pool != nil && marketResp != nil {
		metricsFromPool, metricsErr := r.client.ComputePoolMetrics(ctx, pool, marketResp.TotalSupply, marketResp.CirculatingSupply)
		if metricsErr != nil {
			fmt.Printf("ComputePoolMetrics: error=%v\n", metricsErr)
		} else {
			fmt.Printf("ComputePoolMetrics: marketCapSOL=%s\n", metricsFromPool.MarketCapInSOL)
		}
	} else {
		fmt.Println("ComputePoolMetrics: skipped (missing pool or market response)")
	}

	if pool != nil {
		tokenMetrics, tokenMetricsErr := r.client.ComputeTokenMetricsFromPool(ctx, mint, pool)
		if tokenMetricsErr != nil {
			fmt.Printf("ComputeTokenMetricsFromPool: error=%v\n", tokenMetricsErr)
		} else {
			fmt.Printf("ComputeTokenMetricsFromPool: marketCapSOL=%s\n", tokenMetrics.MarketCapInSOL)
		}
	} else {
		fmt.Println("ComputeTokenMetricsFromPool: skipped (no pool)")
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
