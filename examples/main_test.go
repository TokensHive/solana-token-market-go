package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/TokensHive/solana-token-market-go/sdk/market"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

type fakeClient struct {
	resp  *market.GetMetricsByPoolResponse
	err   error
	debug map[string]any
}

type flakyClient struct {
	calls int
}

type thirdCallFailClient struct {
	calls int
}

type fourthCallFailClient struct {
	calls int
}

type fifthCallFailClient struct {
	calls int
}

type sixthCallFailClient struct {
	calls int
}

type seventhCallFailClient struct {
	calls int
}

type eighthCallFailClient struct {
	calls int
}

type ninthCallFailClient struct {
	calls int
}

type tenthCallFailClient struct {
	calls int
}

type eleventhCallFailClient struct {
	calls int
}

func (f *flakyClient) GetMetricsByPool(context.Context, market.GetMetricsByPoolRequest) (*market.GetMetricsByPoolResponse, error) {
	f.calls++
	if f.calls == 2 {
		return nil, errors.New("second call failure")
	}
	return &market.GetMetricsByPoolResponse{}, nil
}

func (f *flakyClient) LastRequestDebug() map[string]any { return nil }

func (f *thirdCallFailClient) GetMetricsByPool(context.Context, market.GetMetricsByPoolRequest) (*market.GetMetricsByPoolResponse, error) {
	f.calls++
	if f.calls == 3 {
		return nil, errors.New("third call failure")
	}
	return &market.GetMetricsByPoolResponse{}, nil
}

func (f *thirdCallFailClient) LastRequestDebug() map[string]any { return nil }

func (f *fourthCallFailClient) GetMetricsByPool(context.Context, market.GetMetricsByPoolRequest) (*market.GetMetricsByPoolResponse, error) {
	f.calls++
	if f.calls == 4 {
		return nil, errors.New("fourth call failure")
	}
	return &market.GetMetricsByPoolResponse{}, nil
}

func (f *fourthCallFailClient) LastRequestDebug() map[string]any { return nil }

func (f *fifthCallFailClient) GetMetricsByPool(context.Context, market.GetMetricsByPoolRequest) (*market.GetMetricsByPoolResponse, error) {
	f.calls++
	if f.calls == 5 {
		return nil, errors.New("fifth call failure")
	}
	return &market.GetMetricsByPoolResponse{}, nil
}

func (f *fifthCallFailClient) LastRequestDebug() map[string]any { return nil }

func (f *sixthCallFailClient) GetMetricsByPool(context.Context, market.GetMetricsByPoolRequest) (*market.GetMetricsByPoolResponse, error) {
	f.calls++
	if f.calls == 6 {
		return nil, errors.New("sixth call failure")
	}
	return &market.GetMetricsByPoolResponse{}, nil
}

func (f *sixthCallFailClient) LastRequestDebug() map[string]any { return nil }

func (f *seventhCallFailClient) GetMetricsByPool(context.Context, market.GetMetricsByPoolRequest) (*market.GetMetricsByPoolResponse, error) {
	f.calls++
	if f.calls == 7 {
		return nil, errors.New("seventh call failure")
	}
	return &market.GetMetricsByPoolResponse{}, nil
}

func (f *seventhCallFailClient) LastRequestDebug() map[string]any { return nil }

func (f *eighthCallFailClient) GetMetricsByPool(context.Context, market.GetMetricsByPoolRequest) (*market.GetMetricsByPoolResponse, error) {
	f.calls++
	if f.calls == 8 {
		return nil, errors.New("eighth call failure")
	}
	return &market.GetMetricsByPoolResponse{}, nil
}

func (f *eighthCallFailClient) LastRequestDebug() map[string]any { return nil }

func (f *ninthCallFailClient) GetMetricsByPool(context.Context, market.GetMetricsByPoolRequest) (*market.GetMetricsByPoolResponse, error) {
	f.calls++
	if f.calls == 9 {
		return nil, errors.New("ninth call failure")
	}
	return &market.GetMetricsByPoolResponse{}, nil
}

func (f *ninthCallFailClient) LastRequestDebug() map[string]any { return nil }

func (f *tenthCallFailClient) GetMetricsByPool(context.Context, market.GetMetricsByPoolRequest) (*market.GetMetricsByPoolResponse, error) {
	f.calls++
	if f.calls == 10 {
		return nil, errors.New("tenth call failure")
	}
	return &market.GetMetricsByPoolResponse{}, nil
}

func (f *tenthCallFailClient) LastRequestDebug() map[string]any { return nil }

func (f *eleventhCallFailClient) GetMetricsByPool(context.Context, market.GetMetricsByPoolRequest) (*market.GetMetricsByPoolResponse, error) {
	f.calls++
	if f.calls == 11 {
		return nil, errors.New("eleventh call failure")
	}
	return &market.GetMetricsByPoolResponse{}, nil
}

func (f *eleventhCallFailClient) LastRequestDebug() map[string]any { return nil }

func (f *fakeClient) GetMetricsByPool(context.Context, market.GetMetricsByPoolRequest) (*market.GetMetricsByPoolResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.resp != nil {
		return f.resp, nil
	}
	return &market.GetMetricsByPoolResponse{}, nil
}

func (f *fakeClient) LastRequestDebug() map[string]any {
	return f.debug
}

func TestRunNonInteractive(t *testing.T) {
	out := bytes.NewBuffer(nil)
	errOut := bytes.NewBuffer(nil)
	builder := func(string, bool) (metricsClient, error) {
		return &fakeClient{
			resp: &market.GetMetricsByPoolResponse{
				PriceOfAInB:       decimal.NewFromInt(1),
				PriceOfAInSOL:     decimal.NewFromInt(2),
				LiquidityInB:      decimal.NewFromInt(3),
				LiquidityInSOL:    decimal.NewFromInt(4),
				TotalSupply:       decimal.NewFromInt(5),
				CirculatingSupply: decimal.NewFromInt(6),
				MarketCapInSOL:    decimal.NewFromInt(7),
				FDVInSOL:          decimal.NewFromInt(8),
				SupplyMethod:      "method",
			},
			debug: map[string]any{"ok": true},
		}, nil
	}

	code := run([]string{"-interactive=false"}, strings.NewReader(""), out, errOut, builder)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Pumpfun Bonding Curve") ||
		!strings.Contains(out.String(), "Pumpfun PumpSwap AMM") ||
		!strings.Contains(out.String(), "Raydium Liquidity V4") ||
		!strings.Contains(out.String(), "Raydium CPMM") ||
		!strings.Contains(out.String(), "Raydium CLMM") ||
		!strings.Contains(out.String(), "Raydium Launchpad") ||
		!strings.Contains(out.String(), "Meteora DLMM") ||
		!strings.Contains(out.String(), "Meteora DBC") ||
		!strings.Contains(out.String(), "Meteora DAMM V1") ||
		!strings.Contains(out.String(), "Meteora DAMM V2") ||
		!strings.Contains(out.String(), "Orca Whirlpool") {
		t.Fatalf("expected all presets in output, got: %s", out.String())
	}
}

func TestRunErrorPaths(t *testing.T) {
	out := bytes.NewBuffer(nil)
	errOut := bytes.NewBuffer(nil)

	code := run([]string{"-unknown-flag"}, strings.NewReader(""), out, errOut, func(string, bool) (metricsClient, error) {
		return &fakeClient{}, nil
	})
	if code != 2 {
		t.Fatalf("expected flag parse exit code 2, got %d", code)
	}

	code = run([]string{}, strings.NewReader(""), out, errOut, func(string, bool) (metricsClient, error) {
		return nil, errors.New("build failed")
	})
	if code != 1 {
		t.Fatalf("expected client build exit code 1, got %d", code)
	}

	code = run([]string{"-interactive=false"}, strings.NewReader(""), out, errOut, func(string, bool) (metricsClient, error) {
		return &fakeClient{err: errors.New("request failed")}, nil
	})
	if code != 1 {
		t.Fatalf("expected request failure exit code 1, got %d", code)
	}

	code = run([]string{"-interactive=false"}, strings.NewReader(""), out, errOut, func(string, bool) (metricsClient, error) {
		return &flakyClient{}, nil
	})
	if code != 1 {
		t.Fatalf("expected second preset failure exit code 1, got %d", code)
	}

	code = run([]string{"-interactive=false"}, strings.NewReader(""), out, errOut, func(string, bool) (metricsClient, error) {
		return &thirdCallFailClient{}, nil
	})
	if code != 1 {
		t.Fatalf("expected third preset failure exit code 1, got %d", code)
	}

	code = run([]string{"-interactive=false"}, strings.NewReader(""), out, errOut, func(string, bool) (metricsClient, error) {
		return &fourthCallFailClient{}, nil
	})
	if code != 1 {
		t.Fatalf("expected fourth preset failure exit code 1, got %d", code)
	}

	code = run([]string{"-interactive=false"}, strings.NewReader(""), out, errOut, func(string, bool) (metricsClient, error) {
		return &fifthCallFailClient{}, nil
	})
	if code != 1 {
		t.Fatalf("expected fifth preset failure exit code 1, got %d", code)
	}

	code = run([]string{"-interactive=false"}, strings.NewReader(""), out, errOut, func(string, bool) (metricsClient, error) {
		return &sixthCallFailClient{}, nil
	})
	if code != 1 {
		t.Fatalf("expected sixth preset failure exit code 1, got %d", code)
	}

	code = run([]string{"-interactive=false"}, strings.NewReader(""), out, errOut, func(string, bool) (metricsClient, error) {
		return &seventhCallFailClient{}, nil
	})
	if code != 1 {
		t.Fatalf("expected seventh preset failure exit code 1, got %d", code)
	}

	code = run([]string{"-interactive=false"}, strings.NewReader(""), out, errOut, func(string, bool) (metricsClient, error) {
		return &eighthCallFailClient{}, nil
	})
	if code != 1 {
		t.Fatalf("expected eighth preset failure exit code 1, got %d", code)
	}

	code = run([]string{"-interactive=false"}, strings.NewReader(""), out, errOut, func(string, bool) (metricsClient, error) {
		return &ninthCallFailClient{}, nil
	})
	if code != 1 {
		t.Fatalf("expected ninth preset failure exit code 1, got %d", code)
	}

	code = run([]string{"-interactive=false"}, strings.NewReader(""), out, errOut, func(string, bool) (metricsClient, error) {
		return &tenthCallFailClient{}, nil
	})
	if code != 1 {
		t.Fatalf("expected tenth preset failure exit code 1, got %d", code)
	}

	code = run([]string{"-interactive=false"}, strings.NewReader(""), out, errOut, func(string, bool) (metricsClient, error) {
		return &eleventhCallFailClient{}, nil
	})
	if code != 1 {
		t.Fatalf("expected eleventh preset failure exit code 1, got %d", code)
	}
}

func TestRunInteractiveFromRunFunction(t *testing.T) {
	out := bytes.NewBuffer(nil)
	errOut := bytes.NewBuffer(nil)
	builder := func(string, bool) (metricsClient, error) {
		return &fakeClient{
			resp:  &market.GetMetricsByPoolResponse{},
			debug: map[string]any{},
		}, nil
	}
	code := run([]string{}, strings.NewReader("13\n"), out, errOut, builder)
	if code != 0 {
		t.Fatalf("expected interactive exit code 0, got %d stderr=%s", code, errOut.String())
	}
}

func TestInteractiveExitAndUnknownOption(t *testing.T) {
	out := bytes.NewBuffer(nil)
	client := &fakeClient{resp: &market.GetMetricsByPoolResponse{}}
	runInteractive(strings.NewReader("x\n1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n"), out, client, time.Second, false)
	if !strings.Contains(out.String(), "Unknown option.") {
		t.Fatalf("expected unknown option message, got: %s", out.String())
	}
}

func TestInteractiveShowsPresetErrors(t *testing.T) {
	out := bytes.NewBuffer(nil)
	client := &fakeClient{err: errors.New("request failed")}
	runInteractive(strings.NewReader("1\n12\n13\n"), out, client, time.Second, false)
	if !strings.Contains(out.String(), "error: request failed") {
		t.Fatalf("expected interactive error output, got: %s", out.String())
	}
}

func TestInteractiveInvalidNumericOption(t *testing.T) {
	out := bytes.NewBuffer(nil)
	client := &fakeClient{resp: &market.GetMetricsByPoolResponse{}}
	runInteractive(strings.NewReader("999\n13\n"), out, client, time.Second, false)
	if !strings.Contains(out.String(), "Unknown option.") {
		t.Fatalf("expected unknown numeric option message, got: %s", out.String())
	}
}

func TestRunWithPresetAndBuildRequest(t *testing.T) {
	client := &fakeClient{
		resp: &market.GetMetricsByPoolResponse{
			PriceOfAInB:       decimal.NewFromInt(1),
			PriceOfAInSOL:     decimal.NewFromInt(1),
			LiquidityInB:      decimal.NewFromInt(1),
			LiquidityInSOL:    decimal.NewFromInt(1),
			TotalSupply:       decimal.NewFromInt(1),
			CirculatingSupply: decimal.NewFromInt(1),
			MarketCapInSOL:    decimal.NewFromInt(1),
			FDVInSOL:          decimal.NewFromInt(1),
		},
	}
	out := bytes.NewBuffer(nil)

	if err := runWithPreset(client, time.Second, bondingCurvePreset, false, out); err != nil {
		t.Fatalf("expected successful preset run, got %v", err)
	}
	if err := runWithPreset(client, time.Second, poolPreset{Name: "bad", MintA: "bad"}, false, out); err == nil {
		t.Fatal("expected invalid preset error")
	}

	_, err := buildBondingCurveRequest(poolPreset{
		Name:        "bad",
		MintA:       "bad",
		MintB:       solana.SolMint.String(),
	})
	if err == nil {
		t.Fatal("expected invalid mintA error")
	}
	_, err = buildBondingCurveRequest(poolPreset{
		Name:        "bad",
		MintA:       solana.SolMint.String(),
		MintB:       "bad",
	})
	if err == nil {
		t.Fatal("expected invalid mintB error")
	}
	_, err = buildRequest(poolPreset{
		Name:        "bad",
		PoolVersion: market.PoolVersionPumpfunAmm,
		MintA:       solana.SolMint.String(),
		MintB:       solana.SolMint.String(),
		PoolAddress: "bad",
	})
	if err == nil {
		t.Fatal("expected invalid pool address error")
	}
}

func TestRunRequestDebugModes(t *testing.T) {
	out := bytes.NewBuffer(nil)
	req := market.GetMetricsByPoolRequest{
		Pool: market.PoolIdentifier{
			Dex:         market.DexPumpfun,
			PoolVersion: market.PoolVersionPumpfunAmm,
			PoolAddress: solana.SolMint,
		},
	}

	if err := runRequest(&fakeClient{
		resp: &market.GetMetricsByPoolResponse{},
	}, time.Second, req, false, out); err != nil {
		t.Fatalf("expected successful request without debug, got %v", err)
	}

	if err := runRequest(&fakeClient{
		resp:  &market.GetMetricsByPoolResponse{},
		debug: map[string]any{},
	}, time.Second, req, true, out); err != nil {
		t.Fatalf("expected successful request with empty debug, got %v", err)
	}

	err := runRequest(&fakeClient{
		resp:  &market.GetMetricsByPoolResponse{},
		debug: map[string]any{"bad": make(chan int)},
	}, time.Second, req, true, out)
	if err == nil {
		t.Fatal("expected debug marshal error")
	}
}

func TestPrompt(t *testing.T) {
	out := bytes.NewBuffer(nil)
	result := prompt(bufio.NewReader(strings.NewReader("value\n")), out, "Label", "default")
	if result != "value" {
		t.Fatalf("expected provided value, got %s", result)
	}

	result = prompt(bufio.NewReader(strings.NewReader("\n")), out, "Label", "default")
	if result != "default" {
		t.Fatalf("expected default for empty input, got %s", result)
	}

	result = prompt(bufio.NewReader(strings.NewReader("value\r")), out, "Label", "default")
	if result != "value" {
		t.Fatalf("expected CR-terminated value, got %s", result)
	}

	result = prompt(bufio.NewReader(strings.NewReader("value\r\n")), out, "Label", "default")
	if result != "value" {
		t.Fatalf("expected CRLF-terminated value, got %s", result)
	}

	result = prompt(bufio.NewReader(strings.NewReader("value")), out, "Label", "default")
	if result != "value" {
		t.Fatalf("expected EOF-terminated value, got %s", result)
	}

	result = prompt(bufio.NewReader(strings.NewReader("7\x1bOM")), out, "Label", "default")
	if result != "7" {
		t.Fatalf("expected ESC O M terminated value, got %s", result)
	}

	result = prompt(bufio.NewReader(strings.NewReader("8\x1b[M")), out, "Label", "default")
	if result != "8" {
		t.Fatalf("expected ESC [ M terminated value, got %s", result)
	}

	result = prompt(bufio.NewReader(errReader{}), out, "Label", "default")
	if result != "default" {
		t.Fatalf("expected default on reader error, got %s", result)
	}
}

func TestWithCRLF(t *testing.T) {
	out := bytes.NewBuffer(nil)
	writer := withCRLF(out)
	if _, err := writer.Write([]byte("a\nb\r\nc")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if got := out.String(); got != "a\r\nb\r\nc" {
		t.Fatalf("unexpected normalized output: %q", got)
	}
}

func TestReadPromptLineEscapeBranches(t *testing.T) {
	line, err := readPromptLine(bufio.NewReader(strings.NewReader("abc\x1bXYtail\n")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line != "abc" {
		t.Fatalf("expected ESC to terminate buffered text, got %q", line)
	}

	line, err = readPromptLine(bufio.NewReader(strings.NewReader("\x1bXYtail\n")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line != "XYtail" {
		t.Fatalf("expected leading ESC to be ignored, got %q", line)
	}
}

func TestConsumeEscapeAsEnterBranches(t *testing.T) {
	if consumeEscapeAsEnter(bufio.NewReader(strings.NewReader("A"))) {
		t.Fatal("expected false when escape sequence is too short")
	}
	if consumeEscapeAsEnter(bufio.NewReader(strings.NewReader("AB"))) {
		t.Fatal("expected false for non-enter escape sequence")
	}
}

func TestCRLFWriterEdgeCases(t *testing.T) {
	writer := &crlfWriter{w: bytes.NewBuffer(nil)}
	n, err := writer.Write(nil)
	if err != nil {
		t.Fatalf("unexpected error for empty write: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected zero bytes written for empty input, got %d", n)
	}

	writer = &crlfWriter{w: errWriter{}}
	n, err = writer.Write([]byte("x"))
	if err == nil {
		t.Fatal("expected downstream writer error")
	}
	if n != 0 {
		t.Fatalf("expected zero bytes written on downstream error, got %d", n)
	}
}

func TestMain(t *testing.T) {
	originalArgs := os.Args
	originalExit := exitFunc
	originalRun := runFunc
	originalBuilder := defaultClientFunc
	defer func() {
		os.Args = originalArgs
		exitFunc = originalExit
		runFunc = originalRun
		defaultClientFunc = originalBuilder
	}()

	os.Args = []string{"examples", "-interactive=false"}
	var code int
	exitFunc = func(c int) { code = c }
	defaultClientFunc = func(string, bool) (metricsClient, error) { return &fakeClient{}, nil }
	runFunc = func([]string, io.Reader, io.Writer, io.Writer, clientBuilder) int { return 7 }

	main()
	if code != 7 {
		t.Fatalf("expected captured exit code 7, got %d", code)
	}
}

func TestNewClient(t *testing.T) {
	client, err := newClient(defaultRPCURL, false)
	if err != nil {
		t.Fatalf("newClient failed: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestCirculatingSupplyPct(t *testing.T) {
	if got := circulatingSupplyPct(decimal.Zero, decimal.NewFromInt(10)); !got.Equal(decimal.Zero) {
		t.Fatalf("expected zero pct on zero total supply, got %s", got)
	}
	got := circulatingSupplyPct(decimal.NewFromInt(200), decimal.NewFromInt(50))
	if !got.Equal(decimal.NewFromInt(25)) {
		t.Fatalf("expected 25, got %s", got)
	}
}

func TestPooledSOLAndMint_BondingCurveMetadata(t *testing.T) {
	token := mustPubkey(t, "3z2tRjNuQjoq6UDcw4zyEPD1Eb5KXMPYb4GWFzVT1DPg")
	pooledSOL, pooledMint := pooledSOLAndMint(solana.SolMint, token, map[string]any{
		"real_sol_reserve":   "1.25",
		"real_token_reserve": "9900.5",
	})
	if !pooledSOL.Equal(decimal.RequireFromString("1.25")) {
		t.Fatalf("unexpected pooled sol: %s", pooledSOL)
	}
	if !pooledMint.Equal(decimal.RequireFromString("9900.5")) {
		t.Fatalf("unexpected pooled mint: %s", pooledMint)
	}
}

func TestPooledSOLAndMint_BaseQuoteAndCPMMMetadata(t *testing.T) {
	tokenA := mustPubkey(t, "3z2tRjNuQjoq6UDcw4zyEPD1Eb5KXMPYb4GWFzVT1DPg")
	tokenB := mustPubkey(t, "2bpT3ksMdwdZ6DuHyq3FDUr7HDwvZ5DRZoT1fUPALJaH")

	pooledSOL, pooledMint := pooledSOLAndMint(tokenA, solana.SolMint, map[string]any{
		"pool_base_mint":     wrappedSOLMint,
		"pool_quote_mint":    tokenA.String(),
		"pool_base_reserve":  "50",
		"pool_quote_reserve": "100000",
	})
	if !pooledSOL.Equal(decimal.NewFromInt(50)) {
		t.Fatalf("unexpected pooled sol: %s", pooledSOL)
	}
	if !pooledMint.Equal(decimal.NewFromInt(100000)) {
		t.Fatalf("unexpected pooled mint: %s", pooledMint)
	}

	pooledSOL, pooledMint = pooledSOLAndMint(tokenA, tokenB, map[string]any{
		"pool_token0_mint":    tokenA.String(),
		"pool_token1_mint":    tokenB.String(),
		"pool_token0_reserve": "42",
		"pool_token1_reserve": "84",
	})
	if !pooledSOL.Equal(decimal.Zero) {
		t.Fatalf("expected zero pooled sol for non-SOL pair, got %s", pooledSOL)
	}
	if !pooledMint.Equal(decimal.NewFromInt(42)) {
		t.Fatalf("unexpected pooled mint for non-SOL pair: %s", pooledMint)
	}

	pooledSOL, pooledMint = pooledSOLAndMint(solana.SolMint, tokenA, map[string]any{
		"pool_base_mint":     wrappedSOLMint,
		"pool_quote_mint":    tokenA.String(),
		"pool_base_reserve":  "70",
		"pool_quote_reserve": "140000",
	})
	if !pooledSOL.Equal(decimal.NewFromInt(70)) {
		t.Fatalf("unexpected pooled sol for SOL-as-mintA branch: %s", pooledSOL)
	}
	if !pooledMint.Equal(decimal.NewFromInt(140000)) {
		t.Fatalf("unexpected pooled mint for SOL-as-mintA branch: %s", pooledMint)
	}
}

func TestPooledSOLAndMint_InvalidMetadataPaths(t *testing.T) {
	token := mustPubkey(t, "3z2tRjNuQjoq6UDcw4zyEPD1Eb5KXMPYb4GWFzVT1DPg")

	pooledSOL, pooledMint := pooledSOLAndMint(solana.SolMint, token, nil)
	if !pooledSOL.IsZero() || !pooledMint.IsZero() {
		t.Fatalf("expected zero values on empty metadata, got sol=%s mint=%s", pooledSOL, pooledMint)
	}

	pooledSOL, pooledMint = pooledSOLAndMint(solana.SolMint, token, map[string]any{
		"pool_base_mint":     "invalid",
		"pool_quote_mint":    token.String(),
		"pool_base_reserve":  "1",
		"pool_quote_reserve": "2",
	})
	if !pooledSOL.IsZero() || !pooledMint.IsZero() {
		t.Fatalf("expected zero values on invalid mint metadata, got sol=%s mint=%s", pooledSOL, pooledMint)
	}
}

func TestPooledByMintMetadataErrorCases(t *testing.T) {
	if _, _, ok := pooledByMintMetadata(solana.SolMint, solana.SolMint, map[string]any{}, "a", "b", "c", "d"); ok {
		t.Fatal("expected false for missing keys")
	}
	if _, _, ok := pooledByMintMetadata(solana.SolMint, solana.SolMint, map[string]any{
		"a": 123,
		"b": solana.SolMint.String(),
		"c": "1",
		"d": "1",
	}, "a", "b", "c", "d"); ok {
		t.Fatal("expected false for non-string first mint type")
	}
	if _, _, ok := pooledByMintMetadata(solana.SolMint, solana.SolMint, map[string]any{
		"a": " ",
		"b": solana.SolMint.String(),
		"c": "1",
		"d": "1",
	}, "a", "b", "c", "d"); ok {
		t.Fatal("expected false for blank mint string")
	}
	if _, _, ok := pooledByMintMetadata(solana.SolMint, solana.SolMint, map[string]any{
		"a": solana.SolMint.String(),
		"b": " ",
		"c": "1",
		"d": "1",
	}, "a", "b", "c", "d"); ok {
		t.Fatal("expected false for blank second mint string")
	}
	if _, _, ok := pooledByMintMetadata(solana.SolMint, solana.SolMint, map[string]any{
		"a": solana.SolMint.String(),
		"b": solana.SolMint.String(),
		"c": "bad",
		"d": "1",
	}, "a", "b", "c", "d"); ok {
		t.Fatal("expected false for bad first reserve")
	}
	if _, _, ok := pooledByMintMetadata(solana.SolMint, solana.SolMint, map[string]any{
		"a": solana.SolMint.String(),
		"b": solana.SolMint.String(),
		"c": "1",
		"d": "bad",
	}, "a", "b", "c", "d"); ok {
		t.Fatal("expected false for bad second reserve")
	}
	if _, _, ok := pooledByMintMetadata(
		mustPubkey(t, "3z2tRjNuQjoq6UDcw4zyEPD1Eb5KXMPYb4GWFzVT1DPg"),
		solana.SolMint,
		map[string]any{
		"a": solana.SolMint.String(),
		"b": mustPubkey(t, "2bpT3ksMdwdZ6DuHyq3FDUr7HDwvZ5DRZoT1fUPALJaH").String(),
		"c": "1",
		"d": "2",
	}, "a", "b", "c", "d"); ok {
		t.Fatal("expected false when pool mintA does not match either side")
	}
	if _, _, ok := pooledByMintMetadata(
		solana.SolMint,
		mustPubkey(t, "1zJX5gRnjLgmTpq5sVwkq69mNDQkCemqoasyjaPW6jm"),
		map[string]any{
		"a": solana.SolMint.String(),
		"b": mustPubkey(t, "2bpT3ksMdwdZ6DuHyq3FDUr7HDwvZ5DRZoT1fUPALJaH").String(),
		"c": "1",
		"d": "2",
	}, "a", "b", "c", "d"); ok {
		t.Fatal("expected false when pool mintB does not match either side")
	}
}

func TestReserveAndMintMatchingHelpers(t *testing.T) {
	token := mustPubkey(t, "3z2tRjNuQjoq6UDcw4zyEPD1Eb5KXMPYb4GWFzVT1DPg")
	rA := decimal.NewFromInt(10)
	rB := decimal.NewFromInt(20)

	if got, ok := reserveForMint(solana.SolMint, wrappedSOLMint, rA, token.String(), rB); !ok || !got.Equal(rA) {
		t.Fatalf("expected wrapped SOL match, got=%s ok=%v", got, ok)
	}
	if got, ok := reserveForMint(token, wrappedSOLMint, rA, token.String(), rB); !ok || !got.Equal(rB) {
		t.Fatalf("expected token match on second mint, got=%s ok=%v", got, ok)
	}
	if _, ok := reserveForMint(mustPubkey(t, "2bpT3ksMdwdZ6DuHyq3FDUr7HDwvZ5DRZoT1fUPALJaH"), wrappedSOLMint, rA, token.String(), rB); ok {
		t.Fatal("expected no match for unrelated mint")
	}

	if publicKeyMatchesString(token, "bad") {
		t.Fatal("expected invalid candidate mint to fail matching")
	}
	if !mintsEquivalent(solana.SolMint, mustPubkey(t, wrappedSOLMint)) {
		t.Fatal("expected native SOL and wrapped SOL to be equivalent")
	}
	if mintsEquivalent(solana.SolMint, token) {
		t.Fatal("did not expect unrelated mint equivalence")
	}
	if !isSOLMint(solana.SolMint) {
		t.Fatal("expected sol mint detection")
	}
	if isSOLMint(token) {
		t.Fatal("did not expect token mint to be SOL")
	}
	if !isWrappedSOLMint(mustPubkey(t, wrappedSOLMint)) {
		t.Fatal("expected wrapped SOL detection")
	}
	if isWrappedSOLMint(token) {
		t.Fatal("did not expect token mint to be wrapped SOL")
	}
}

func TestParseDecimalVariants(t *testing.T) {
	testCases := []struct {
		name   string
		value  any
		wantOK bool
		want   decimal.Decimal
	}{
		{name: "decimal", value: decimal.NewFromInt(1), wantOK: true, want: decimal.NewFromInt(1)},
		{name: "string", value: "2.5", wantOK: true, want: decimal.RequireFromString("2.5")},
		{name: "float64", value: float64(3.5), wantOK: true, want: decimal.NewFromFloat(3.5)},
		{name: "float32", value: float32(4.5), wantOK: true, want: decimal.NewFromFloat32(4.5)},
		{name: "int", value: int(5), wantOK: true, want: decimal.NewFromInt(5)},
		{name: "int64", value: int64(6), wantOK: true, want: decimal.NewFromInt(6)},
		{name: "uint64", value: uint64(7), wantOK: true, want: decimal.NewFromInt(7)},
		{name: "invalid-string", value: "bad", wantOK: false, want: decimal.Zero},
		{name: "unsupported", value: true, wantOK: false, want: decimal.Zero},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseDecimal(tc.value)
			if ok != tc.wantOK {
				t.Fatalf("expected ok=%v, got %v", tc.wantOK, ok)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("expected value %s, got %s", tc.want, got)
			}
		})
	}
}

func TestRunRequestWithExtendedFieldsAndError(t *testing.T) {
	out := bytes.NewBuffer(nil)
	req := market.GetMetricsByPoolRequest{
		Pool: market.PoolIdentifier{
			Dex:         market.DexRaydium,
			PoolVersion: market.PoolVersionRaydiumCPMM,
			PoolAddress: mustPubkey(t, "BScfGKZf9YDfpL11hZQnCQPskPrdeyFcvCjSA5qupEH5"),
		},
	}

	err := runRequest(&fakeClient{
		resp: &market.GetMetricsByPoolResponse{
			MintA:             mustPubkey(t, "3z2tRjNuQjoq6UDcw4zyEPD1Eb5KXMPYb4GWFzVT1DPg"),
			MintB:             solana.SolMint,
			TotalSupply:       decimal.NewFromInt(100),
			CirculatingSupply: decimal.NewFromInt(50),
			Metadata: map[string]any{
				"pool_token0_mint":    wrappedSOLMint,
				"pool_token1_mint":    "3z2tRjNuQjoq6UDcw4zyEPD1Eb5KXMPYb4GWFzVT1DPg",
				"pool_token0_reserve": "25",
				"pool_token1_reserve": "1000",
				"fdv_method":          "mint_total_supply_default",
				"fdv_supply":          "100",
			},
		},
		debug: map[string]any{"ok": true},
	}, time.Second, req, true, out)
	if err != nil {
		t.Fatalf("expected successful run request, got %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "circ_supply_pct=50") {
		t.Fatalf("expected circulating pct in output, got: %s", got)
	}
	if !strings.Contains(got, "pooled_sol=25") || !strings.Contains(got, "pooled_mint=1000") {
		t.Fatalf("expected pooled values in output, got: %s", got)
	}
	if !strings.Contains(got, "fdv_method=mint_total_supply_default") || !strings.Contains(got, "fdv_supply=100") {
		t.Fatalf("expected fdv metadata in output, got: %s", got)
	}

	if err := runRequest(&fakeClient{err: errors.New("boom")}, time.Second, req, false, out); err == nil {
		t.Fatal("expected request error to be returned")
	}
}

func mustPubkey(t *testing.T, value string) solana.PublicKey {
	t.Helper()
	pk, err := solana.PublicKeyFromBase58(value)
	if err != nil {
		t.Fatalf("invalid pubkey %q: %v", value, err)
	}
	return pk
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("read error")
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, errors.New("write error")
}
