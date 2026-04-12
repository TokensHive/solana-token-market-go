package pumpfun

import "github.com/gagliardetto/solana-go"

func DeriveBondingCurvePDA(mint solana.PublicKey) solana.PublicKey {
if defaultProgramID.IsZero() || mint.IsZero() {
return solana.PublicKey{}
}
pda, _, err := solana.FindProgramAddress([][]byte{[]byte("bonding-curve"), mint.Bytes()}, defaultProgramID)
if err != nil {
return solana.PublicKey{}
}
return pda
}
