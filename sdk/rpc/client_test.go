package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TokensHive/solana-token-market-go/sdk/internal/reqdebug"
	"github.com/gagliardetto/solana-go"
	rpcclient "github.com/gagliardetto/solana-go/rpc"
)

type rpcRequest struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
}

type mockBackend struct {
	getAccountInfoWithOptsFn      func(context.Context, solana.PublicKey, *rpcclient.GetAccountInfoOpts) (*rpcclient.GetAccountInfoResult, error)
	getMultipleAccountsWithOptsFn func(context.Context, []solana.PublicKey, *rpcclient.GetMultipleAccountsOpts) (*rpcclient.GetMultipleAccountsResult, error)
	getTokenSupplyFn              func(context.Context, solana.PublicKey, rpcclient.CommitmentType) (*rpcclient.GetTokenSupplyResult, error)
	getSignaturesWithOptsFn       func(context.Context, solana.PublicKey, *rpcclient.GetSignaturesForAddressOpts) ([]*rpcclient.TransactionSignature, error)
	getTransactionFn              func(context.Context, solana.Signature, *rpcclient.GetTransactionOpts) (*rpcclient.GetTransactionResult, error)
	rpcCallForIntoFn              func(context.Context, interface{}, string, []interface{}) error
}

func (m *mockBackend) GetAccountInfoWithOpts(ctx context.Context, account solana.PublicKey, opts *rpcclient.GetAccountInfoOpts) (*rpcclient.GetAccountInfoResult, error) {
	return m.getAccountInfoWithOptsFn(ctx, account, opts)
}

func (m *mockBackend) GetMultipleAccountsWithOpts(ctx context.Context, accounts []solana.PublicKey, opts *rpcclient.GetMultipleAccountsOpts) (*rpcclient.GetMultipleAccountsResult, error) {
	return m.getMultipleAccountsWithOptsFn(ctx, accounts, opts)
}

func (m *mockBackend) GetTokenSupply(ctx context.Context, tokenMint solana.PublicKey, commitment rpcclient.CommitmentType) (*rpcclient.GetTokenSupplyResult, error) {
	return m.getTokenSupplyFn(ctx, tokenMint, commitment)
}

func (m *mockBackend) GetSignaturesForAddressWithOpts(ctx context.Context, account solana.PublicKey, opts *rpcclient.GetSignaturesForAddressOpts) ([]*rpcclient.TransactionSignature, error) {
	return m.getSignaturesWithOptsFn(ctx, account, opts)
}

func (m *mockBackend) GetTransaction(ctx context.Context, txSig solana.Signature, opts *rpcclient.GetTransactionOpts) (*rpcclient.GetTransactionResult, error) {
	return m.getTransactionFn(ctx, txSig, opts)
}

func (m *mockBackend) RPCCallForInto(ctx context.Context, out interface{}, method string, params []interface{}) error {
	return m.rpcCallForIntoFn(ctx, out, method, params)
}

func newRPCServer(t *testing.T, handler func(method string, id json.RawMessage) string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(req.ID) == 0 {
			req.ID = json.RawMessage("1")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(handler(req.Method, req.ID)))
	}))
}

func TestSolanaRPCClient_GetAccount(t *testing.T) {
	server := newRPCServer(t, func(method string, id json.RawMessage) string {
		if method == "getAccountInfo" {
			return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"context":{"slot":12},"value":{"data":["AA==","base64"],"lamports":100,"owner":"11111111111111111111111111111111","executable":false,"rentEpoch":0}}}`
		}
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"error":{"code":-32601,"message":"unknown"}}`
	})
	defer server.Close()

	client := NewSolanaRPCClient(server.URL)
	rec := reqdebug.NewRecorder("rpc")
	ctx := reqdebug.WithRecorder(context.Background(), rec)
	info, err := client.GetAccount(ctx, solana.SolMint)
	if err != nil {
		t.Fatalf("get account failed: %v", err)
	}
	if info == nil || !info.Exists || info.Slot != 12 {
		t.Fatalf("unexpected account info: %#v", info)
	}

	snapshot := rec.SnapshotMap()
	rpcBlock := snapshot["rpc"].(map[string]any)
	if rpcBlock["total"].(int) != 1 {
		t.Fatalf("expected one rpc call, got %v", rpcBlock["total"])
	}
}

func TestSolanaRPCClient_GetAccountNotFoundAndError(t *testing.T) {
	notFoundServer := newRPCServer(t, func(method string, id json.RawMessage) string {
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"context":{"slot":1},"value":null}}`
	})
	defer notFoundServer.Close()

	client := NewSolanaRPCClient(notFoundServer.URL)
	info, err := client.GetAccount(context.Background(), solana.SolMint)
	if err != nil {
		t.Fatalf("unexpected error for null account: %v", err)
	}
	if info == nil || info.Exists {
		t.Fatalf("expected non-existing account, got %#v", info)
	}

	nilValueServer := newRPCServer(t, func(method string, id json.RawMessage) string {
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"context":{"slot":1}}}`
	})
	defer nilValueServer.Close()
	client = NewSolanaRPCClient(nilValueServer.URL)
	info, err = client.GetAccount(context.Background(), solana.SolMint)
	if err != nil {
		t.Fatalf("unexpected error for missing value field: %v", err)
	}
	if info == nil || info.Exists {
		t.Fatalf("expected non-existing account for missing value field, got %#v", info)
	}

	errServer := newRPCServer(t, func(method string, id json.RawMessage) string {
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"error":{"code":-32000,"message":"boom"}}`
	})
	defer errServer.Close()

	client = NewSolanaRPCClient(errServer.URL)
	if _, err = client.GetAccount(context.Background(), solana.SolMint); err == nil {
		t.Fatal("expected rpc error")
	}
}

func TestSolanaRPCClient_GetMultipleAccounts(t *testing.T) {
	server := newRPCServer(t, func(method string, id json.RawMessage) string {
		if method == "getMultipleAccounts" {
			return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"context":{"slot":22},"value":[{"data":["AA==","base64"],"lamports":2,"owner":"11111111111111111111111111111111","executable":false,"rentEpoch":0},null]}}`
		}
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"error":{"code":-32601,"message":"unknown"}}`
	})
	defer server.Close()

	client := NewSolanaRPCClient(server.URL)
	keys := []solana.PublicKey{solana.SolMint, solana.MustPublicKeyFromBase58("11111111111111111111111111111111")}
	info, err := client.GetMultipleAccounts(context.Background(), keys)
	if err != nil {
		t.Fatalf("get multiple failed: %v", err)
	}
	if len(info) != 2 || !info[0].Exists || info[1].Exists {
		t.Fatalf("unexpected multiple accounts output: %#v", info)
	}

	errServer := newRPCServer(t, func(method string, id json.RawMessage) string {
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"error":{"code":-32000,"message":"boom"}}`
	})
	defer errServer.Close()

	client = NewSolanaRPCClient(errServer.URL)
	if _, err = client.GetMultipleAccounts(context.Background(), keys); err == nil {
		t.Fatal("expected rpc error for get multiple accounts")
	}
}

func TestSolanaRPCClient_GetTokenSupply(t *testing.T) {
	server := newRPCServer(t, func(method string, id json.RawMessage) string {
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"context":{"slot":33},"value":{"amount":"1234500","decimals":4,"uiAmount":123.45,"uiAmountString":"123.45"}}}`
	})
	defer server.Close()

	client := NewSolanaRPCClient(server.URL)
	amount, decimals, err := client.GetTokenSupply(context.Background(), solana.SolMint)
	if err != nil {
		t.Fatalf("get token supply failed: %v", err)
	}
	if amount.String() != "123.45" || decimals != 4 {
		t.Fatalf("unexpected token supply amount=%s decimals=%d", amount, decimals)
	}

	badAmountServer := newRPCServer(t, func(method string, id json.RawMessage) string {
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"context":{"slot":33},"value":{"amount":"NaN","decimals":4,"uiAmount":0,"uiAmountString":"0"}}}`
	})
	defer badAmountServer.Close()

	client = NewSolanaRPCClient(badAmountServer.URL)
	if _, _, err = client.GetTokenSupply(context.Background(), solana.SolMint); err == nil {
		t.Fatal("expected parse error for invalid amount")
	}

	errServer := newRPCServer(t, func(method string, id json.RawMessage) string {
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"error":{"code":-32000,"message":"boom"}}`
	})
	defer errServer.Close()
	client = NewSolanaRPCClient(errServer.URL)
	if _, _, err = client.GetTokenSupply(context.Background(), solana.SolMint); err == nil {
		t.Fatal("expected rpc error for token supply")
	}
}

func TestSolanaRPCClient_GetSignaturesForAddress(t *testing.T) {
	server := newRPCServer(t, func(method string, id json.RawMessage) string {
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":[{"signature":"1111111111111111111111111111111111111111111111111111111111111111","slot":1,"err":null,"memo":null,"blockTime":1,"confirmationStatus":"finalized"}]}`
	})
	defer server.Close()

	client := NewSolanaRPCClient(server.URL)
	limit := 5
	before := solana.Signature{}
	until := solana.Signature{}
	out, err := client.GetSignaturesForAddress(context.Background(), solana.SolMint, &SignaturesForAddressOptions{
		Limit:  limit,
		Before: &before,
		Until:  &until,
	})
	if err != nil {
		t.Fatalf("get signatures failed: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one signature, got %d", len(out))
	}

	out, err = client.GetSignaturesForAddress(context.Background(), solana.SolMint, nil)
	if err != nil || len(out) != 1 {
		t.Fatalf("expected signatures with nil options, out=%#v err=%v", out, err)
	}

	errServer := newRPCServer(t, func(method string, id json.RawMessage) string {
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"error":{"code":-32000,"message":"boom"}}`
	})
	defer errServer.Close()
	client = NewSolanaRPCClient(errServer.URL)
	if _, err = client.GetSignaturesForAddress(context.Background(), solana.SolMint, nil); err == nil {
		t.Fatal("expected signatures rpc error")
	}
}

func TestSolanaRPCClient_GetTransactionAndRaw(t *testing.T) {
	txServer := newRPCServer(t, func(method string, id json.RawMessage) string {
		switch method {
		case "getTransaction":
			return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"slot":1,"blockTime":1,"meta":{"err":null,"fee":0,"preBalances":[],"postBalances":[]},"transaction":["AQ==","base64"],"version":"legacy"}}`
		default:
			return `{"jsonrpc":"2.0","id":` + string(id) + `,"error":{"code":-32601,"message":"unknown"}}`
		}
	})
	defer txServer.Close()

	client := NewSolanaRPCClient(txServer.URL)
	if _, err := client.GetTransaction(context.Background(), solana.Signature{}); err != nil {
		t.Fatalf("get transaction failed: %v", err)
	}

	notFoundServer := newRPCServer(t, func(method string, id json.RawMessage) string {
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":null}`
	})
	defer notFoundServer.Close()

	client = NewSolanaRPCClient(notFoundServer.URL)
	if _, err := client.GetTransaction(context.Background(), solana.Signature{}); err == nil {
		t.Fatal("expected not found error for null result")
	}

	errServer := newRPCServer(t, func(method string, id json.RawMessage) string {
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"error":{"code":-32000,"message":"boom"}}`
	})
	defer errServer.Close()
	client = NewSolanaRPCClient(errServer.URL)
	if _, err := client.GetTransaction(context.Background(), solana.Signature{}); err == nil {
		t.Fatal("expected get transaction rpc error")
	}

	rawServer := newRPCServer(t, func(method string, id json.RawMessage) string {
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"slot":9}}`
	})
	defer rawServer.Close()

	client = NewSolanaRPCClient(rawServer.URL)
	raw, err := client.GetTransactionRaw(context.Background(), solana.Signature{})
	if err != nil {
		t.Fatalf("get transaction raw failed: %v", err)
	}
	if string(raw) != `{"slot":9}` {
		t.Fatalf("unexpected raw payload: %s", string(raw))
	}

	rawNullServer := newRPCServer(t, func(method string, id json.RawMessage) string {
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":null}`
	})
	defer rawNullServer.Close()
	client = NewSolanaRPCClient(rawNullServer.URL)
	if _, err := client.GetTransactionRaw(context.Background(), solana.Signature{}); err == nil {
		t.Fatal("expected not found for raw null result")
	}

	rawErrServer := newRPCServer(t, func(method string, id json.RawMessage) string {
		return `{"jsonrpc":"2.0","id":` + string(id) + `,"error":{"code":-32000,"message":"boom"}}`
	})
	defer rawErrServer.Close()
	client = NewSolanaRPCClient(rawErrServer.URL)
	if _, err := client.GetTransactionRaw(context.Background(), solana.Signature{}); err == nil {
		t.Fatal("expected get transaction raw rpc error")
	}
}

func TestNoopClient(t *testing.T) {
	client := NewNoopClient()

	account, err := client.GetAccount(context.Background(), solana.SolMint)
	if err != nil || account.Exists {
		t.Fatalf("unexpected noop get account result: account=%#v err=%v", account, err)
	}
	multi, err := client.GetMultipleAccounts(context.Background(), []solana.PublicKey{solana.SolMint})
	if err != nil || len(multi) != 1 || multi[0].Exists {
		t.Fatalf("unexpected noop get multiple result: %#v err=%v", multi, err)
	}
	if _, _, err = client.GetTokenSupply(context.Background(), solana.SolMint); err != nil {
		t.Fatalf("unexpected noop supply error: %v", err)
	}
	if sigs, err := client.GetSignaturesForAddress(context.Background(), solana.SolMint, nil); err != nil || sigs != nil {
		t.Fatalf("unexpected noop signatures result: %#v err=%v", sigs, err)
	}
	if tx, err := client.GetTransaction(context.Background(), solana.Signature{}); err != nil || tx != nil {
		t.Fatalf("unexpected noop tx result: %#v err=%v", tx, err)
	}
	if raw, err := client.GetTransactionRaw(context.Background(), solana.Signature{}); err != nil || raw != nil {
		t.Fatalf("unexpected noop raw result: %#v err=%v", raw, err)
	}
}

func TestClientInterfaceCompileAssertion(t *testing.T) {
	var _ Client = (*SolanaRPCClient)(nil)
	var _ Client = (*noopClient)(nil)
	var _ = rpcclient.CommitmentFinalized
}

func TestGetAccountNotFoundByErrorString(t *testing.T) {
	client := &SolanaRPCClient{
		inner: &mockBackend{
			getAccountInfoWithOptsFn: func(context.Context, solana.PublicKey, *rpcclient.GetAccountInfoOpts) (*rpcclient.GetAccountInfoResult, error) {
				return nil, errors.New("resource not found")
			},
		},
	}
	info, err := client.GetAccount(context.Background(), solana.SolMint)
	if err != nil {
		t.Fatalf("expected not-found error to be normalized, got %v", err)
	}
	if info == nil || info.Exists {
		t.Fatalf("expected normalized missing account, got %#v", info)
	}
}

func TestGetAccountNilResultBranch(t *testing.T) {
	client := &SolanaRPCClient{
		inner: &mockBackend{
			getAccountInfoWithOptsFn: func(context.Context, solana.PublicKey, *rpcclient.GetAccountInfoOpts) (*rpcclient.GetAccountInfoResult, error) {
				return nil, nil
			},
		},
	}
	info, err := client.GetAccount(context.Background(), solana.SolMint)
	if err != nil {
		t.Fatalf("expected nil-result account to map to not found, got %v", err)
	}
	if info == nil || info.Exists {
		t.Fatalf("expected non-existing account on nil result, got %#v", info)
	}
}

func TestGetTransactionNilResultBranch(t *testing.T) {
	client := &SolanaRPCClient{
		inner: &mockBackend{
			getTransactionFn: func(context.Context, solana.Signature, *rpcclient.GetTransactionOpts) (*rpcclient.GetTransactionResult, error) {
				return nil, nil
			},
		},
	}
	if _, err := client.GetTransaction(context.Background(), solana.Signature{}); err == nil {
		t.Fatal("expected nil-result branch error")
	}
}

func TestSolanaRPCClient_RecorderPaths(t *testing.T) {
	server := newRPCServer(t, func(method string, id json.RawMessage) string {
		switch method {
		case "getAccountInfo":
			return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"context":{"slot":1},"value":{"data":["AA==","base64"],"lamports":1,"owner":"11111111111111111111111111111111","executable":false,"rentEpoch":0}}}`
		case "getMultipleAccounts":
			return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"context":{"slot":1},"value":[{"data":["AA==","base64"],"lamports":1,"owner":"11111111111111111111111111111111","executable":false,"rentEpoch":0}]}}`
		case "getTokenSupply":
			return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"context":{"slot":1},"value":{"amount":"1","decimals":0,"uiAmount":1,"uiAmountString":"1"}}}`
		case "getSignaturesForAddress":
			return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":[]}`
		case "getTransaction":
			return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"slot":1,"blockTime":1,"meta":{"err":null,"fee":0,"preBalances":[],"postBalances":[]},"transaction":["AQ==","base64"],"version":"legacy"}}`
		default:
			return `{"jsonrpc":"2.0","id":` + string(id) + `,"result":{"slot":1}}`
		}
	})
	defer server.Close()

	client := NewSolanaRPCClient(server.URL)
	rec := reqdebug.NewRecorder("rpc")
	ctx := reqdebug.WithRecorder(context.Background(), rec)

	if _, err := client.GetAccount(ctx, solana.SolMint); err != nil {
		t.Fatalf("get account with recorder failed: %v", err)
	}
	if _, err := client.GetMultipleAccounts(ctx, []solana.PublicKey{solana.SolMint}); err != nil {
		t.Fatalf("get multiple accounts with recorder failed: %v", err)
	}
	if _, _, err := client.GetTokenSupply(ctx, solana.SolMint); err != nil {
		t.Fatalf("get token supply with recorder failed: %v", err)
	}
	if _, err := client.GetSignaturesForAddress(ctx, solana.SolMint, nil); err != nil {
		t.Fatalf("get signatures with recorder failed: %v", err)
	}
	if _, err := client.GetTransaction(ctx, solana.Signature{}); err != nil {
		t.Fatalf("get transaction with recorder failed: %v", err)
	}
	if _, err := client.GetTransactionRaw(ctx, solana.Signature{}); err != nil {
		t.Fatalf("get raw transaction with recorder failed: %v", err)
	}
}
