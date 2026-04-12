package quote

import "github.com/TokensHive/solana-token-market-go/sdk/internal/pubkeyx"

func IsSOL(m string) bool { return pubkeyx.IsSOLMintString(m) }
