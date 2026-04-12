package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/TokensHive/solana-token-market-go/examples/internal/examplecli"
)

func main() {
	poolFlag := flag.String("pool", "6PiyjiAPkp2KdZtqkyQYzVsD1Prv7t8v4TaYd8ip4YFd", "Pool address")
	rpcURLFlag := flag.String("rpc", "https://api.mainnet-beta.solana.com", "Solana RPC URL")
	timeoutFlag := flag.Duration("timeout", 30*time.Second, "Request timeout")
	flag.Parse()

	runner, err := examplecli.NewRunner(*rpcURLFlag)
	if err != nil {
		exitErr(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	if err := runner.GetPool(ctx, *poolFlag); err != nil {
		exitErr(err)
	}
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
