package meteora

import "github.com/gagliardetto/solana-go"

func DeriveDBCPool(baseMint, quoteMint, config, programID solana.PublicKey) solana.PublicKey {
	if baseMint.IsZero() || quoteMint.IsZero() || config.IsZero() || programID.IsZero() {
		return solana.PublicKey{}
	}
	pda, _, err := solana.FindProgramAddress([][]byte{[]byte("dbc_pool"), baseMint.Bytes(), quoteMint.Bytes(), config.Bytes()}, programID)
	if err != nil {
		return solana.PublicKey{}
	}
	return pda
}
