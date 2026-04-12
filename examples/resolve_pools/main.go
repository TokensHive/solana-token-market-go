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
	mint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	resp, err := client.ResolvePools(context.Background(), market.ResolvePoolsRequest{Mint: mint, SelectPrimary: true})
	if err != nil {
		panic(err)
	}
	fmt.Printf("mint=%s pools=%d primary=%v\n", resp.Mint, len(resp.Pools), resp.PrimaryPool != nil)
}
