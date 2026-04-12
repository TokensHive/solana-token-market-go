package rpc

import "github.com/gagliardetto/solana-go"

func ChunkPubkeys(keys []solana.PublicKey, size int) [][]solana.PublicKey {
if size <= 0 {
size = 100
}
out := make([][]solana.PublicKey, 0, (len(keys)+size-1)/size)
for i := 0; i < len(keys); i += size {
j := i + size
if j > len(keys) {
j = len(keys)
}
out = append(out, keys[i:j])
}
return out
}
