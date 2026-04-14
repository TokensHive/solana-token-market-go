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

func (f *flakyClient) GetMetricsByPool(context.Context, market.GetMetricsByPoolRequest) (*market.GetMetricsByPoolResponse, error) {
	f.calls++
	if f.calls == 2 {
		return nil, errors.New("second call failure")
	}
	return &market.GetMetricsByPoolResponse{}, nil
}

func (f *flakyClient) LastRequestDebug() map[string]any { return nil }

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
				SupplyMethod:      "method",
			},
			debug: map[string]any{"ok": true},
		}, nil
	}

	code := run([]string{"-interactive=false"}, strings.NewReader(""), out, errOut, builder)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Pumpfun Bonding Curve") || !strings.Contains(out.String(), "Pumpfun PumpSwap AMM") {
		t.Fatalf("expected both presets in output, got: %s", out.String())
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
	code := run([]string{}, strings.NewReader("4\n"), out, errOut, builder)
	if code != 0 {
		t.Fatalf("expected interactive exit code 0, got %d stderr=%s", code, errOut.String())
	}
}

func TestInteractiveExitAndUnknownOption(t *testing.T) {
	out := bytes.NewBuffer(nil)
	client := &fakeClient{resp: &market.GetMetricsByPoolResponse{}}
	runInteractive(strings.NewReader("x\n1\n2\n3\n4\n"), out, client, time.Second, false)
	if !strings.Contains(out.String(), "Unknown option.") {
		t.Fatalf("expected unknown option message, got: %s", out.String())
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
		},
	}
	out := bytes.NewBuffer(nil)

	if err := runWithPreset(client, time.Second, bondingCurvePreset, false, out); err != nil {
		t.Fatalf("expected successful preset run, got %v", err)
	}
	if err := runWithPreset(client, time.Second, poolPreset{Name: "bad", MintA: "bad"}, false, out); err == nil {
		t.Fatal("expected invalid preset error")
	}

	_, err := buildRequest(poolPreset{
		Name:        "bad",
		PoolVersion: market.PoolVersionPumpfunAmm,
		MintA:       "bad",
		MintB:       solana.SolMint.String(),
		PoolAddress: solana.SolMint.String(),
	})
	if err == nil {
		t.Fatal("expected invalid mintA error")
	}
	_, err = buildRequest(poolPreset{
		Name:        "bad",
		PoolVersion: market.PoolVersionPumpfunAmm,
		MintA:       solana.SolMint.String(),
		MintB:       "bad",
		PoolAddress: solana.SolMint.String(),
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
			MintA:       solana.SolMint,
			MintB:       solana.SolMint,
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

	result = prompt(bufio.NewReader(errReader{}), out, "Label", "default")
	if result != "default" {
		t.Fatalf("expected default on reader error, got %s", result)
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

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("read error")
}
