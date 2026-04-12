package quote

import "github.com/gagliardetto/solana-go"

func IsSOL(m string) bool { return m == solana.SolMint.String() }
