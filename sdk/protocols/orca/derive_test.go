package orca

import (
	"testing"

	"github.com/gagliardetto/solana-go"
)

func TestDeriveWhirlpoolAddressDeterministic(t *testing.T) {
	config := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	mintA := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	mintB := solana.MustPublicKeyFromBase58("Es9vMFrzaCERmJfrF4H2ZQ8QYfV7kRXKuX3sX5Yucs5b")
	program := solana.MustPublicKeyFromBase58("whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc")
	p1 := DeriveWhirlpoolAddress(config, mintA, mintB, 64, program)
	p2 := DeriveWhirlpoolAddress(config, mintA, mintB, 64, program)
	if p1.IsZero() || p1.String() != p2.String() {
		t.Fatal("expected deterministic non-zero whirlpool pda")
	}
}
