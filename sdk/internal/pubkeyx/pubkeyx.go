package pubkeyx

import "github.com/gagliardetto/solana-go"

func Must(s string) solana.PublicKey {
return solana.MustPublicKeyFromBase58(s)
}
