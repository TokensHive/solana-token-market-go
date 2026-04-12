package pumpswap

import "github.com/gagliardetto/solana-go"

func DerivePoolPDA(baseMint, quoteMint, config solana.PublicKey, programID solana.PublicKey) solana.PublicKey {
if baseMint.IsZero() || quoteMint.IsZero() || programID.IsZero() {
return solana.PublicKey{}
}
p, _, err := solana.FindProgramAddress([][]byte{[]byte("pool"), baseMint.Bytes(), quoteMint.Bytes(), config.Bytes()}, programID)
if err != nil {
return solana.PublicKey{}
}
return p
}
