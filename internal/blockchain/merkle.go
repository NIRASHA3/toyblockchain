package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// EmptyMerkleRoot is the deterministic Merkle root used for blocks without
// transactions, including the canonical genesis block.
const EmptyMerkleRoot = "0000000000000000000000000000000000000000000000000000000000000000"

// TransactionHash returns the canonical leaf hash for a transaction.
// The leaf commits to the full transaction, including its ID and signature.
func TransactionHash(tx Transaction) string {
	var buf bytes.Buffer
	writeCanonicalString(&buf, "id", tx.ID)
	writeCanonicalString(&buf, "from", tx.From)
	writeCanonicalString(&buf, "to", tx.To)
	fmt.Fprintf(&buf, "amount=%d\n", tx.Amount)
	fmt.Fprintf(&buf, "created_at=%d\n", tx.CreatedAt)
	writeCanonicalString(&buf, "memo", tx.Memo)
	fmt.Fprintf(&buf, "tx_nonce=%d\n", tx.Nonce)
	writeCanonicalString(&buf, "public_key", tx.PublicKey)
	writeCanonicalString(&buf, "signature", tx.Signature)

	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:])
}

// ComputeMerkleRoot builds a deterministic binary Merkle tree from the given
// transactions. If a level has an odd number of nodes, the final node is
// duplicated, matching the common blockchain approach for odd tree widths.
func ComputeMerkleRoot(transactions []Transaction) string {
	if len(transactions) == 0 {
		return EmptyMerkleRoot
	}

	level := make([]string, len(transactions))
	for i, tx := range transactions {
		level[i] = TransactionHash(tx)
	}

	for len(level) > 1 {
		next := make([]string, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			left := level[i]
			right := left
			if i+1 < len(level) {
				right = level[i+1]
			}
			next = append(next, hashMerklePair(left, right))
		}
		level = next
	}

	return level[0]
}

func hashMerklePair(left, right string) string {
	payload := []byte("left=" + left + "\nright=" + right + "\n")
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
