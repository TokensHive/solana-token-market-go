package meteora

import "github.com/gagliardetto/solana-go"

func DeriveDAMMV2Pool(baseMint, quoteMint, config, programID solana.PublicKey) solana.PublicKey {
	if baseMint.IsZero() || quoteMint.IsZero() || programID.IsZero() {
		return solana.PublicKey{}
	}
	p, _, err := solana.FindProgramAddress([][]byte{[]byte("pool"), config.Bytes(), baseMint.Bytes(), quoteMint.Bytes()}, programID)
	if err != nil {
		return solana.PublicKey{}
	}
	return p
}

func DeriveDLMMPool(baseMint, quoteMint, config, programID solana.PublicKey) solana.PublicKey {
	if baseMint.IsZero() || quoteMint.IsZero() || programID.IsZero() {
		return solana.PublicKey{}
	}
	p, _, err := solana.FindProgramAddress([][]byte{[]byte("lb_pair"), config.Bytes(), baseMint.Bytes(), quoteMint.Bytes()}, programID)
	if err != nil {
		return solana.PublicKey{}
	}
	return p
}
