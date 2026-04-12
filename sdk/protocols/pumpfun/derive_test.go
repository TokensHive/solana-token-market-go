package pumpfun

import (
	"testing"

	"github.com/gagliardetto/solana-go"
)

func TestDeriveBondingCurvePDA(t *testing.T) {
	mint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	pda := DeriveBondingCurvePDA(mint)
	if pda.IsZero() {
		t.Fatal("expected non-zero bonding curve pda")
	}
	pda2 := DeriveBondingCurvePDA(mint)
	if pda.String() != pda2.String() {
		t.Fatal("expected deterministic derivation")
	}
}
