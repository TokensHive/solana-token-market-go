package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/TokensHive/solana-token-market-go/sdk/market"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/gagliardetto/solana-go"
)

const defaultRPCURL = "https://api.mainnet-beta.solana.com"

type metricsClient interface {
	GetMetricsByPool(ctx context.Context, req market.GetMetricsByPoolRequest) (*market.GetMetricsByPoolResponse, error)
	LastRequestDebug() map[string]any
}

type poolPreset struct {
	Name        string
	PoolVersion market.PoolVersion
	MintA       string
	MintB       string
	PoolAddress string
}

type clientBuilder func(rpcURL string, debug bool) (metricsClient, error)

var (
	bondingCurvePreset = poolPreset{
		Name:        "Pumpfun Bonding Curve",
		PoolVersion: market.PoolVersionPumpfunBondingCurve,
		MintA:       "8NLTeNTuKxXPqtVBTMMUaZjhQz3N7VLUfCxsKZsHpump",
		MintB:       solana.SolMint.String(),
		PoolAddress: "FXP71sea7EDaiLKbK8SepaCzJnBAdFxSrQz9KBkY41kR",
	}
	pumpSwapPreset = poolPreset{
		Name:        "Pumpfun PumpSwap AMM",
		PoolVersion: market.PoolVersionPumpfunAmm,
		MintA:       "9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump",
		MintB:       solana.SolMint.String(),
		PoolAddress: "EQqvZi6mSaQL95wWkP5vGBX6ZsAkVTqZCV88rQU1fbcY",
	}
	exitFunc          = os.Exit
	runFunc           = run
	defaultClientFunc = newClient
)

func main() {
	exitFunc(runFunc(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, defaultClientFunc))
}

func run(args []string, in io.Reader, out io.Writer, errOut io.Writer, buildClient clientBuilder) int {
	flags := flag.NewFlagSet("metrics-cli", flag.ContinueOnError)
	flags.SetOutput(errOut)

	rpcURL := flags.String("rpc", defaultRPCURL, "Solana RPC endpoint")
	timeout := flags.Duration("timeout", 45*time.Second, "Per-request timeout")
	interactive := flags.Bool("interactive", true, "Run in interactive mode")
	printDebug := flags.Bool("debug", true, "Print request debug telemetry")

	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 2
	}

	client, err := buildClient(*rpcURL, *printDebug)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	fmt.Fprintf(out, "RPC: %s\n", *rpcURL)
	fmt.Fprintf(out, "Interactive: %v | Debug: %v\n", *interactive, *printDebug)

	if !*interactive {
		if err := runWithPreset(client, *timeout, bondingCurvePreset, *printDebug, out); err != nil {
			fmt.Fprintf(errOut, "error: %v\n", err)
			return 1
		}
		if err := runWithPreset(client, *timeout, pumpSwapPreset, *printDebug, out); err != nil {
			fmt.Fprintf(errOut, "error: %v\n", err)
			return 1
		}
		return 0
	}

	runInteractive(in, out, client, *timeout, *printDebug)
	return 0
}

func newClient(rpcURL string, debug bool) (metricsClient, error) {
	return market.NewClient(
		market.WithRPCClient(rpc.NewSolanaRPCClient(rpcURL)),
		market.WithDebugRequests(debug),
	)
}

func runInteractive(in io.Reader, out io.Writer, client metricsClient, timeout time.Duration, printDebug bool) {
	reader := bufio.NewReader(in)
	for {
		fmt.Fprintln(out, "\n=== Metrics CLI ===")
		fmt.Fprintln(out, "1) Pumpfun Bonding Curve")
		fmt.Fprintln(out, "2) Pumpfun PumpSwap AMM")
		fmt.Fprintln(out, "3) Run both")
		fmt.Fprintln(out, "4) Exit")

		choice := prompt(reader, out, "Choose option", "3")
		switch choice {
		case "1":
			_ = runWithPreset(client, timeout, bondingCurvePreset, printDebug, out)
		case "2":
			_ = runWithPreset(client, timeout, pumpSwapPreset, printDebug, out)
		case "3":
			_ = runWithPreset(client, timeout, bondingCurvePreset, printDebug, out)
			_ = runWithPreset(client, timeout, pumpSwapPreset, printDebug, out)
		case "4":
			return
		default:
			fmt.Fprintln(out, "Unknown option.")
		}
	}
}

func runWithPreset(client metricsClient, timeout time.Duration, preset poolPreset, printDebug bool, out io.Writer) error {
	req, err := buildRequest(preset)
	if err != nil {
		return fmt.Errorf("preset %q invalid: %w", preset.Name, err)
	}
	fmt.Fprintf(out, "\n--- %s ---\n", preset.Name)
	return runRequest(client, timeout, req, printDebug, out)
}

func buildRequest(preset poolPreset) (market.GetMetricsByPoolRequest, error) {
	mintAPK, err := solana.PublicKeyFromBase58(strings.TrimSpace(preset.MintA))
	if err != nil {
		return market.GetMetricsByPoolRequest{}, fmt.Errorf("invalid mintA: %w", err)
	}
	mintBPK, err := solana.PublicKeyFromBase58(strings.TrimSpace(preset.MintB))
	if err != nil {
		return market.GetMetricsByPoolRequest{}, fmt.Errorf("invalid mintB: %w", err)
	}
	poolPK, err := solana.PublicKeyFromBase58(strings.TrimSpace(preset.PoolAddress))
	if err != nil {
		return market.GetMetricsByPoolRequest{}, fmt.Errorf("invalid pool address: %w", err)
	}
	return market.GetMetricsByPoolRequest{
		Pool: market.PoolIdentifier{
			Dex:         market.DexPumpfun,
			PoolVersion: preset.PoolVersion,
			MintA:       mintAPK,
			MintB:       mintBPK,
			PoolAddress: poolPK,
		},
	}, nil
}

func runRequest(client metricsClient, timeout time.Duration, req market.GetMetricsByPoolRequest, printDebug bool, out io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Fprintf(out, "dex=%s version=%s pool=%s\n", req.Pool.Dex, req.Pool.PoolVersion, req.Pool.PoolAddress.String())
	fmt.Fprintf(out, "pair=%s / %s\n", req.Pool.MintA.String(), req.Pool.MintB.String())

	resp, err := client.GetMetricsByPool(ctx, req)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "price_a_in_b=%s\n", resp.PriceOfAInB)
	fmt.Fprintf(out, "price_a_in_sol=%s\n", resp.PriceOfAInSOL)
	fmt.Fprintf(out, "liquidity_in_b=%s\n", resp.LiquidityInB)
	fmt.Fprintf(out, "liquidity_in_sol=%s\n", resp.LiquidityInSOL)
	fmt.Fprintf(out, "total_supply=%s\n", resp.TotalSupply)
	fmt.Fprintf(out, "circulating_supply=%s\n", resp.CirculatingSupply)
	fmt.Fprintf(out, "market_cap_in_sol=%s\n", resp.MarketCapInSOL)
	fmt.Fprintf(out, "supply_method=%s\n", resp.SupplyMethod)

	if !printDebug {
		return nil
	}
	debug := client.LastRequestDebug()
	if len(debug) == 0 {
		fmt.Fprintln(out, "debug: <empty>")
		return nil
	}
	encoded, err := json.MarshalIndent(debug, "", "  ")
	if err != nil {
		return fmt.Errorf("debug marshal error: %w", err)
	}
	fmt.Fprintln(out, "debug:")
	fmt.Fprintln(out, string(encoded))
	return nil
}

func prompt(reader *bufio.Reader, out io.Writer, label string, defaultValue string) string {
	fmt.Fprintf(out, "%s [%s]: ", label, defaultValue)
	line, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return defaultValue
	}
	return value
}
