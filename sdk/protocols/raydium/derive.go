package raydium

import "github.com/gagliardetto/solana-go"

func DeriveCLMMPool(config, mintA, mintB, programID solana.PublicKey) solana.PublicKey {
if config.IsZero() || mintA.IsZero() || mintB.IsZero() || programID.IsZero() {
return solana.PublicKey{}
}
p, _, err := solana.FindProgramAddress([][]byte{[]byte("pool"), config.Bytes(), mintA.Bytes(), mintB.Bytes()}, programID)
if err != nil {
return solana.PublicKey{}
}
return p
}
