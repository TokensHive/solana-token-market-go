package pubkeyx

import (
	"testing"

	"github.com/gagliardetto/solana-go"
)

func TestMustAndSOLHelpers(t *testing.T) {
	if got := Must(solana.SolMint.String()); !got.Equals(solana.SolMint) {
		t.Fatalf("expected parsed SOL mint, got %s", got.String())
	}

	if !IsSOLMintString(solana.SolMint.String()) {
		t.Fatal("expected SOL mint string to be recognized")
	}
	if !IsSOLMintString(WrappedSOLMintStr) {
		t.Fatal("expected wrapped SOL mint string to be recognized")
	}
	if IsSOLMintString("11111111111111111111111111111111") {
		t.Fatal("expected non-SOL mint string to be rejected")
	}

	if !IsSOLMint(solana.SolMint) {
		t.Fatal("expected native SOL mint to be recognized")
	}
	if !IsSOLMint(WrappedSOLMint) {
		t.Fatal("expected wrapped SOL mint to be recognized")
	}
	if IsSOLMint(solana.MustPublicKeyFromBase58("11111111111111111111111111111111")) {
		t.Fatal("expected non-SOL mint to be rejected")
	}
}
