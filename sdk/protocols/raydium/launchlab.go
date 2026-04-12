package raydium

import "github.com/TokensHive/solana-token-market-go/sdk/market"

func IsLaunchPoolMigrated(p *market.Pool) bool {
if p == nil {
return false
}
return p.IsMigrated || p.Protocol != market.ProtocolRaydiumLaunchLab
}
