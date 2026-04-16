package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/TokensHive/solana-token-market-go/sdk/market"
	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

const defaultRPCURL = "https://api.mainnet-beta.solana.com"
const wrappedSOLMint = "So11111111111111111111111111111111111111112"

type metricsClient interface {
	GetMetricsByPool(ctx context.Context, req market.GetMetricsByPoolRequest) (*market.GetMetricsByPoolResponse, error)
	LastRequestDebug() map[string]any
}

type poolPreset struct {
	Name        string
	Dex         market.Dex
	PoolVersion market.PoolVersion
	MintA       string
	MintB       string
	PoolAddress string
}

type clientBuilder func(rpcURL string, debug bool) (metricsClient, error)

var (
	bondingCurvePreset = poolPreset{
		Name:        "Pumpfun Bonding Curve",
		Dex:         market.DexPumpfun,
		PoolVersion: market.PoolVersionPumpfunBondingCurve,
		MintA:       "4rmmtFU6vmCLCuvuHsAWWcE5oNt8MrPbu1taatbdpump",
		MintB:       solana.SolMint.String(),
		PoolAddress: "4r6uiUerEQz8n2ycZnnv6zDvnrVMQKKgBQnjUiYtavGz",
	}
	pumpSwapPreset = poolPreset{
		Name:        "Pumpfun PumpSwap AMM",
		Dex:         market.DexPumpfun,
		PoolVersion: market.PoolVersionPumpfunAmm,
		MintA:       "9BHt7aq3DFCb74kZjPY5epgVtsWKCeYX1tUWxYwDpump",
		MintB:       solana.SolMint.String(),
		PoolAddress: "EQqvZi6mSaQL95wWkP5vGBX6ZsAkVTqZCV88rQU1fbcY",
	}
	raydiumV4Preset = poolPreset{
		Name:        "Raydium Liquidity V4",
		Dex:         market.DexRaydium,
		PoolVersion: market.PoolVersionRaydiumLiquidityV4,
		MintA:       "2nCeHpECQvnMfzjU5fDMAKws1vBxMzxvWr6qqLpApump",
		MintB:       solana.SolMint.String(),
		PoolAddress: "81BTnebmHFZdVMhFKHhQKAnEwgGPTNbMj1fezsbUjtkG",
	}
	raydiumCPMMPreset = poolPreset{
		Name:        "Raydium CPMM (SURGE/SOL)",
		Dex:         market.DexRaydium,
		PoolVersion: market.PoolVersionRaydiumCPMM,
		MintA:       "3z2tRjNuQjoq6UDcw4zyEPD1Eb5KXMPYb4GWFzVT1DPg",
		MintB:       solana.SolMint.String(),
		PoolAddress: "BScfGKZf9YDfpL11hZQnCQPskPrdeyFcvCjSA5qupEH5",
	}
	raydiumCLMMPreset = poolPreset{
		Name:        "Raydium CLMM (USDC/SOL)",
		Dex:         market.DexRaydium,
		PoolVersion: market.PoolVersionRaydiumCLMM,
		MintA:       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		MintB:       solana.SolMint.String(),
		PoolAddress: "2QdhepnKRTLjjSqPL1PtKNwqrUkoLee5Gqs8bvZhRdMv",
	}
	raydiumLaunchpadPreset = poolPreset{
		Name:        "Raydium Launchpad (VIBINGCAT/SOL)",
		Dex:         market.DexRaydium,
		PoolVersion: market.PoolVersionRaydiumLaunchpad,
		MintA:       "nXU1zKEAyqPJmsjfdSuc4bcFUt8huu6GFUVgziebonk",
		MintB:       solana.SolMint.String(),
		PoolAddress: "257urGqFaYq3BjCVrA6GS7MdfyZR4mb11RWEeuG73LYG",
	}
	meteoraDLMMPreset = poolPreset{
		Name:        "Meteora DLMM (SPIKE/SOL)",
		Dex:         market.DexMeteora,
		PoolVersion: market.PoolVersionMeteoraDLMM,
		MintA:       "BFiGUxnidogqcZAPVPDZRCfhx3nXnFLYqpQUaUGpump",
		MintB:       solana.SolMint.String(),
		PoolAddress: "6KvXWfjwZ7mfiFALRDmj4YvJw3LxfSnLadj1kZfBykYp",
	}
	meteoraDBCPreset = poolPreset{
		Name:        "Meteora DBC (FLYWHEEL/SOL)",
		Dex:         market.DexMeteora,
		PoolVersion: market.PoolVersionMeteoraDBC,
		MintA:       "8pmn9W36uuJDACuw4wVTtrnw4rGDhmiP9Kgsne5Hbrrr",
		MintB:       solana.SolMint.String(),
		PoolAddress: "Fc2rywpnDPrb4ik2V31tKTdo4EEWTZaHQJCWcLjAUvFD",
	}
	meteoraDAMMV1Preset = poolPreset{
		Name:        "Meteora DAMM V1 (NOBODY/SOL)",
		Dex:         market.DexMeteora,
		PoolVersion: market.PoolVersionMeteoraDAMMV1,
		MintA:       "C29ebrgYjYoJPMGPnPSGY1q3mMGk4iDSqnQeQQA7moon",
		MintB:       solana.SolMint.String(),
		PoolAddress: "7rQd8FhC1rimV3v9edCRZ6RNFsJN1puXM9UmjaURJRNj",
	}
	meteoraDAMMV2Preset = poolPreset{
		Name:        "Meteora DAMM V2 (PEPE/SOL)",
		Dex:         market.DexMeteora,
		PoolVersion: market.PoolVersionMeteoraDAMMV2,
		MintA:       "EkJuyYyD3to61CHVPJn6wHb7xANxvqApnVJ4o2SdBAGS",
		MintB:       solana.SolMint.String(),
		PoolAddress: "8Lvqv2jgNvcx1NDtMHd5Ahx8ZUjETfLwygq9MtDfPHxe",
	}
	orcaWhirlpoolPreset = poolPreset{
		Name:        "Orca Whirlpool (MOLT/SOL)",
		Dex:         market.DexOrca,
		PoolVersion: market.PoolVersionOrcaWhirlpool,
		MintA:       "5552z6Qp2xr596ox1UVN4ppDwwyjCfY8cXwzHMXgMcaS",
		MintB:       solana.SolMint.String(),
		PoolAddress: "EAzwsTfbdPmjW5eEWtxNpXaHLzVJmpTUGzkTPnCPrTHd",
	}
	defaultPresets = []poolPreset{
		bondingCurvePreset,
		pumpSwapPreset,
		raydiumV4Preset,
		raydiumCPMMPreset,
		raydiumCLMMPreset,
		raydiumLaunchpadPreset,
		meteoraDLMMPreset,
		meteoraDBCPreset,
		meteoraDAMMV1Preset,
		meteoraDAMMV2Preset,
		orcaWhirlpoolPreset,
	}
	exitFunc          = os.Exit
	runFunc           = run
	defaultClientFunc = newClient
)

func main() {
	exitFunc(runFunc(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, defaultClientFunc))
}

func run(args []string, in io.Reader, out io.Writer, errOut io.Writer, buildClient clientBuilder) int {
	out = withCRLF(out)
	errOut = withCRLF(errOut)

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
		for _, preset := range defaultPresets {
			if err := runWithPreset(client, *timeout, preset, *printDebug, out); err != nil {
				fmt.Fprintf(errOut, "error: %v\n", err)
				return 1
			}
		}
		return 0
	}

	runInteractive(in, out, client, *timeout, *printDebug)
	return 0
}

func runInteractive(in io.Reader, out io.Writer, client metricsClient, timeout time.Duration, printDebug bool) {
	reader := bufio.NewReader(in)
	runPreset := func(preset poolPreset) {
		if err := runWithPreset(client, timeout, preset, printDebug, out); err != nil {
			fmt.Fprintf(out, "error: %v\n", err)
		}
	}

	runAllChoice := len(defaultPresets) + 1
	exitChoice := len(defaultPresets) + 2
	for {
		fmt.Fprintln(out, "\n=== Metrics CLI ===")
		for i, preset := range defaultPresets {
			fmt.Fprintf(out, "%d) %s\n", i+1, preset.Name)
		}
		fmt.Fprintf(out, "%d) Run all\n", runAllChoice)
		fmt.Fprintf(out, "%d) Exit\n", exitChoice)

		choice := prompt(reader, out, "Choose option", strconv.Itoa(runAllChoice))
		selection, err := strconv.Atoi(strings.TrimSpace(choice))
		if err != nil {
			fmt.Fprintln(out, "Unknown option.")
			continue
		}
		switch {
		case selection >= 1 && selection <= len(defaultPresets):
			runPreset(defaultPresets[selection-1])
		case selection == runAllChoice:
			for _, preset := range defaultPresets {
				runPreset(preset)
			}
		case selection == exitChoice:
			return
		default:
			fmt.Fprintln(out, "Unknown option.")
		}
	}
}

func newClient(rpcURL string, debug bool) (metricsClient, error) {
	return market.NewClient(
		market.WithRPCClient(rpc.NewSolanaRPCClient(rpcURL)),
		market.WithDebugRequests(debug),
	)
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
	dex := preset.Dex
	if dex == "" {
		dex = market.DexPumpfun
	}
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
			Dex:         dex,
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
	fmt.Fprintf(out, "circ_supply_pct=%s\n", circulatingSupplyPct(resp.TotalSupply, resp.CirculatingSupply))
	fmt.Fprintf(out, "market_cap_in_sol=%s\n", resp.MarketCapInSOL)
	fmt.Fprintf(out, "fdv_in_sol=%s\n", resp.FDVInSOL)
	pooledSOL, pooledMint := pooledSOLAndMint(req.Pool, resp.Metadata)
	fmt.Fprintf(out, "pooled_sol=%s\n", pooledSOL)
	fmt.Fprintf(out, "pooled_mint=%s\n", pooledMint)
	if fdvMethod, ok := resp.Metadata["fdv_method"]; ok {
		fmt.Fprintf(out, "fdv_method=%v\n", fdvMethod)
	}
	if fdvSupply, ok := resp.Metadata["fdv_supply"]; ok {
		fmt.Fprintf(out, "fdv_supply=%v\n", fdvSupply)
	}
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
	line, err := readPromptLine(reader)
	if err != nil {
		return defaultValue
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return defaultValue
	}
	return value
}

func readPromptLine(reader *bufio.Reader) (string, error) {
	var builder strings.Builder
	for {
		ch, _, err := reader.ReadRune()
		if err != nil {
			if err == io.EOF && builder.Len() > 0 {
				return builder.String(), nil
			}
			return "", err
		}
		if ch == '\n' {
			return builder.String(), nil
		}
		if ch == '\r' {
			next, err := reader.Peek(1)
			if err == nil && len(next) == 1 && next[0] == '\n' {
				_, _ = reader.ReadByte()
			}
			return builder.String(), nil
		}
		if ch == 0x1b {
			if consumeEscapeAsEnter(reader) {
				return builder.String(), nil
			}
			if builder.Len() > 0 {
				return builder.String(), nil
			}
			continue
		}
		builder.WriteRune(ch)
	}
}

func consumeEscapeAsEnter(reader *bufio.Reader) bool {
	peek, err := reader.Peek(2)
	if err != nil || len(peek) < 2 {
		return false
	}
	// Keypad enter in some terminals when application keypad mode is enabled.
	if (peek[0] == 'O' || peek[0] == '[') && peek[1] == 'M' {
		_, _ = reader.ReadByte()
		_, _ = reader.ReadByte()
		return true
	}
	return false
}

func withCRLF(w io.Writer) io.Writer {
	return &crlfWriter{w: w}
}

type crlfWriter struct {
	w io.Writer
}

func (c *crlfWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	normalized := make([]byte, 0, len(p)+16)
	for i := 0; i < len(p); i++ {
		if p[i] == '\n' {
			if i > 0 && p[i-1] == '\r' {
				normalized = append(normalized, '\n')
				continue
			}
			normalized = append(normalized, '\r', '\n')
			continue
		}
		normalized = append(normalized, p[i])
	}
	_, err := c.w.Write(normalized)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func circulatingSupplyPct(totalSupply decimal.Decimal, circulatingSupply decimal.Decimal) decimal.Decimal {
	if totalSupply.IsZero() {
		return decimal.Zero
	}
	return circulatingSupply.Div(totalSupply).Mul(decimal.NewFromInt(100))
}

func pooledSOLAndMint(pool market.PoolIdentifier, metadata map[string]any) (decimal.Decimal, decimal.Decimal) {
	if len(metadata) == 0 {
		return decimal.Zero, decimal.Zero
	}

	if reserveToken, ok := parseDecimal(metadata["real_token_reserve"]); ok {
		if reserveSOL, ok := parseDecimal(metadata["real_sol_reserve"]); ok {
			return reserveSOL, reserveToken
		}
	}

	if pooledSOL, pooledMint, ok := pooledByMintMetadata(
		pool,
		metadata,
		"pool_base_mint",
		"pool_quote_mint",
		"pool_base_reserve",
		"pool_quote_reserve",
	); ok {
		return pooledSOL, pooledMint
	}

	if pooledSOL, pooledMint, ok := pooledByMintMetadata(
		pool,
		metadata,
		"pool_token0_mint",
		"pool_token1_mint",
		"pool_token0_reserve",
		"pool_token1_reserve",
	); ok {
		return pooledSOL, pooledMint
	}
	return decimal.Zero, decimal.Zero
}

func pooledByMintMetadata(
	pool market.PoolIdentifier,
	metadata map[string]any,
	mintAKey string,
	mintBKey string,
	reserveAKey string,
	reserveBKey string,
) (decimal.Decimal, decimal.Decimal, bool) {
	mintAStr, ok := metadata[mintAKey].(string)
	if !ok || strings.TrimSpace(mintAStr) == "" {
		return decimal.Zero, decimal.Zero, false
	}
	mintBStr, ok := metadata[mintBKey].(string)
	if !ok || strings.TrimSpace(mintBStr) == "" {
		return decimal.Zero, decimal.Zero, false
	}
	reserveA, ok := parseDecimal(metadata[reserveAKey])
	if !ok {
		return decimal.Zero, decimal.Zero, false
	}
	reserveB, ok := parseDecimal(metadata[reserveBKey])
	if !ok {
		return decimal.Zero, decimal.Zero, false
	}

	mintAReserve, ok := reserveForMint(pool.MintA, mintAStr, reserveA, mintBStr, reserveB)
	if !ok {
		return decimal.Zero, decimal.Zero, false
	}
	mintBReserve, ok := reserveForMint(pool.MintB, mintAStr, reserveA, mintBStr, reserveB)
	if !ok {
		return decimal.Zero, decimal.Zero, false
	}

	pooledMint := mintAReserve
	pooledSOL := decimal.Zero
	if isSOLMint(pool.MintA) {
		pooledSOL = mintAReserve
		pooledMint = mintBReserve
	} else if isSOLMint(pool.MintB) {
		pooledSOL = mintBReserve
	}
	return pooledSOL, pooledMint, true
}

func reserveForMint(
	target solana.PublicKey,
	mintAStr string,
	reserveA decimal.Decimal,
	mintBStr string,
	reserveB decimal.Decimal,
) (decimal.Decimal, bool) {
	if publicKeyMatchesString(target, mintAStr) {
		return reserveA, true
	}
	if publicKeyMatchesString(target, mintBStr) {
		return reserveB, true
	}
	return decimal.Zero, false
}

func publicKeyMatchesString(target solana.PublicKey, candidate string) bool {
	candidatePK, err := solana.PublicKeyFromBase58(strings.TrimSpace(candidate))
	if err != nil {
		return false
	}
	return mintsEquivalent(target, candidatePK)
}

func mintsEquivalent(a, b solana.PublicKey) bool {
	if a.Equals(b) {
		return true
	}
	return (isSOLMint(a) && isWrappedSOLMint(b)) || (isWrappedSOLMint(a) && isSOLMint(b))
}

func isSOLMint(mint solana.PublicKey) bool {
	return mint.Equals(solana.SolMint)
}

func isWrappedSOLMint(mint solana.PublicKey) bool {
	return mint.String() == wrappedSOLMint
}

func parseDecimal(value any) (decimal.Decimal, bool) {
	switch v := value.(type) {
	case decimal.Decimal:
		return v, true
	case string:
		d, err := decimal.NewFromString(strings.TrimSpace(v))
		if err != nil {
			return decimal.Zero, false
		}
		return d, true
	case float64:
		return decimal.NewFromFloat(v), true
	case float32:
		return decimal.NewFromFloat32(v), true
	case int:
		return decimal.NewFromInt(int64(v)), true
	case int64:
		return decimal.NewFromInt(v), true
	case uint64:
		return decimal.NewFromUint64(v), true
	default:
		return decimal.Zero, false
	}
}
