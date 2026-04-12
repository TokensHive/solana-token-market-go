package orca

import "github.com/gagliardetto/solana-go"

func DeriveWhirlpoolAddress(config, mintA, mintB solana.PublicKey, tickSpacing uint16, programID solana.PublicKey) solana.PublicKey {
	if config.IsZero() || mintA.IsZero() || mintB.IsZero() || programID.IsZero() {
		return solana.PublicKey{}
	}
	spacing := []byte{byte(tickSpacing), byte(tickSpacing >> 8)}
	pda, _, err := solana.FindProgramAddress([][]byte{[]byte("whirlpool"), config.Bytes(), mintA.Bytes(), mintB.Bytes(), spacing}, programID)
	if err != nil {
		return solana.PublicKey{}
	}
	return pda
}

func DeriveOraclePDA(whirlpool, programID solana.PublicKey) solana.PublicKey {
	if whirlpool.IsZero() || programID.IsZero() {
		return solana.PublicKey{}
	}
	pda, _, err := solana.FindProgramAddress([][]byte{[]byte("oracle"), whirlpool.Bytes()}, programID)
	if err != nil {
		return solana.PublicKey{}
	}
	return pda
}
