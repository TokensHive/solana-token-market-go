package quote

import (
	"testing"

	"github.com/TokensHive/solana-token-market-go/sdk/internal/pubkeyx"
	"github.com/gagliardetto/solana-go"
)

func TestIsSOL(t *testing.T) {
	if !IsSOL(solana.SolMint.String()) {
		t.Fatal("expected native sol mint to be recognized")
	}
	if !IsSOL(pubkeyx.WrappedSOLMintStr) {
		t.Fatal("expected wrapped sol mint to be recognized")
	}
	if IsSOL("11111111111111111111111111111111") {
		t.Fatal("expected non-sol mint to be rejected")
	}
}
