package pubkeyx

import "github.com/gagliardetto/solana-go"

const WrappedSOLMintStr = "So11111111111111111111111111111111111111112"

var WrappedSOLMint = solana.MustPublicKeyFromBase58(WrappedSOLMintStr)

func IsSOLMintString(m string) bool {
	return m == solana.SolMint.String() || m == WrappedSOLMintStr
}

func IsSOLMint(pk solana.PublicKey) bool {
	return pk.Equals(solana.SolMint) || pk.Equals(WrappedSOLMint)
}
