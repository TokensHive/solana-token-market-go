package supply

import (
	"context"
	"encoding/binary"

	"github.com/TokensHive/solana-token-market-go/sdk/rpc"
	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

const (
	pumpfunBondingCurveSeed        = "bonding-curve"
	pumpfunTokenTotalSupplyOffset  = 40
	pumpfunTokenTotalSupplyDataLen = 48
)

var (
	pumpfunProgramID         = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
	fdvMethodMintTotalSupply = "mint_total_supply"
	fdvMethodPumpCurveTotal  = "pumpfun_curve_token_total_supply"
)

func ResolveFDVSupply(ctx context.Context, rpcClient rpc.Client, mint solana.PublicKey, totalSupply decimal.Decimal) (decimal.Decimal, string) {
	if rpcClient == nil || mint.IsZero() {
		return totalSupply, fdvMethodMintTotalSupply
	}
	bondingCurve, _, _ := solana.FindProgramAddress([][]byte{
		[]byte(pumpfunBondingCurveSeed),
		mint.Bytes(),
	}, pumpfunProgramID)
	info, err := rpcClient.GetAccount(ctx, bondingCurve)
	if err != nil || info == nil || !info.Exists || !info.Owner.Equals(pumpfunProgramID) || len(info.Data) < pumpfunTokenTotalSupplyDataLen {
		return totalSupply, fdvMethodMintTotalSupply
	}
	raw := binary.LittleEndian.Uint64(info.Data[pumpfunTokenTotalSupplyOffset : pumpfunTokenTotalSupplyOffset+8])
	pumpCurveSupply := decimal.NewFromUint64(raw).Shift(-6)
	if pumpCurveSupply.GreaterThan(totalSupply) {
		return pumpCurveSupply, fdvMethodPumpCurveTotal
	}
	return totalSupply, fdvMethodMintTotalSupply
}
