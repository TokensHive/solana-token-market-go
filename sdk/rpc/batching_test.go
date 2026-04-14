package rpc

import (
	"testing"

	"github.com/gagliardetto/solana-go"
)

func TestChunkPubkeys(t *testing.T) {
	keys := []solana.PublicKey{
		solana.SolMint,
		solana.MustPublicKeyFromBase58("11111111111111111111111111111111"),
		solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111"),
	}

	chunks := ChunkPubkeys(keys, 2)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 2 || len(chunks[1]) != 1 {
		t.Fatalf("unexpected chunk sizes: %d and %d", len(chunks[0]), len(chunks[1]))
	}

	defaultSized := ChunkPubkeys(keys, 0)
	if len(defaultSized) != 1 || len(defaultSized[0]) != len(keys) {
		t.Fatalf("expected single default-sized chunk, got %#v", defaultSized)
	}
}
