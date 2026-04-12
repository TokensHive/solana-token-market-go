package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/TokensHive/solana-token-market-go/examples/internal/examplecli"
)

func main() {
	commandFlag := flag.String("cmd", string(examplecli.CommandInteractive), "Command: resolve-pools|get-pool|get-token-market|all-methods|batch-all|interactive")
	mintFlag := flag.String("mint", examplecli.SampleMints[0], "Token mint address")
	poolFlag := flag.String("pool", "", "Pool address (for get-pool)")
	protocolFlag := flag.String("protocol", "", "Protocol override for all-methods")
	rpcURLFlag := flag.String("rpc", "https://api.mainnet-beta.solana.com", "Solana RPC URL")
	debugFlag := flag.Bool("debug", false, "Enable per-operation request debug stats")
	timeoutFlag := flag.Duration("timeout", 30*time.Second, "Request timeout")
	flag.Parse()

	runner, err := examplecli.NewRunner(*rpcURLFlag, *debugFlag)
	if err != nil {
		exitErr(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	command := examplecli.Command(strings.TrimSpace(*commandFlag))
	if command == examplecli.CommandInteractive {
		err = examplecli.RunInteractive(ctx, runner)
	} else {
		err = examplecli.RunCommand(ctx, runner, command, *mintFlag, *poolFlag, *protocolFlag)
	}
	if err != nil {
		exitErr(err)
	}
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
