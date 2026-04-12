package raydium

import "github.com/gagliardetto/solana-go"

func DeriveCanonicalCPMMPool(baseMint, quoteMint, config, programID solana.PublicKey) solana.PublicKey {
	if baseMint.IsZero() || quoteMint.IsZero() || config.IsZero() || programID.IsZero() {
		return solana.PublicKey{}
	}
	pda, _, err := solana.FindProgramAddress([][]byte{[]byte("pool"), config.Bytes(), baseMint.Bytes(), quoteMint.Bytes()}, programID)
	if err != nil {
		return solana.PublicKey{}
	}
	return pda
}
