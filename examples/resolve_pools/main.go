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
	mintFlag := flag.String("mint", examplecli.SampleMints[0], "Token mint address")
	rpcURLFlag := flag.String("rpc", "https://api.mainnet-beta.solana.com", "Solana RPC URL")
	timeoutFlag := flag.Duration("timeout", 30*time.Second, "Request timeout")
	flag.Parse()

	runner, err := examplecli.NewRunner(*rpcURLFlag, false)
	if err != nil {
		exitErr(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	if err := runner.ResolvePools(ctx, *mintFlag); err != nil {
		exitErr(err)
	}
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
