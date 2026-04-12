package main

import (
	"context"
	"fmt"

	"github.com/TokensHive/solana-token-market-go/sdk/discovery"
	"github.com/TokensHive/solana-token-market-go/sdk/market"
	"github.com/TokensHive/solana-token-market-go/sdk/parser"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/gagliardetto/solana-go"
)

func main() {
	rpcClient := rpc.NewSolanaRPCClient("https://api.mainnet-beta.solana.com")
	engine := discovery.NewEngine(rpcClient, parser.NewNoopAdapter())
	client, err := market.NewClient(
		market.WithRPCClient(rpcClient),
		market.WithParserAdapter(parser.NewNoopAdapter()),
		market.WithDiscoveryEngine(engine),
	)
	if err != nil {
		panic(err)
	}
	tokenMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	resolved, err := client.ResolvePools(context.Background(), market.ResolvePoolsRequest{
		Mint:          tokenMint,
		SelectPrimary: true,
	})
	if err != nil || len(resolved.Pools) == 0 {
		fmt.Println("no pools found for mint:", tokenMint.String())
		return
	}
	pool := solana.MustPublicKeyFromBase58(resolved.Pools[0].Address)
	p, err := client.GetPool(context.Background(), market.GetPoolRequest{PoolAddress: pool})
	if err != nil {
		fmt.Println("not found:", err)
		return
	}
	fmt.Printf("pool=%s protocol=%s\n", p.Address, p.Protocol)
}
