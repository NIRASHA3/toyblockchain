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

// MerkleProofStep is one sibling hash needed to prove that a transaction leaf
// belongs to a Merkle root. Position describes where the sibling is relative to
// the running hash being verified.
type MerkleProofStep struct {
	Position string `json:"position"`
	Hash     string `json:"hash"`
}

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
		level = nextMerkleLevel(level)
	}

	return level[0]
}

// BuildMerkleProof returns the sibling hashes required to prove that the
// transaction at index belongs to the Merkle root of transactions.
func BuildMerkleProof(transactions []Transaction, index int) ([]MerkleProofStep, error) {
	if len(transactions) == 0 {
		return nil, fmt.Errorf("cannot build merkle proof for empty transaction list")
	}
	if index < 0 || index >= len(transactions) {
		return nil, fmt.Errorf("transaction index %d out of range", index)
	}

	level := make([]string, len(transactions))
	for i, tx := range transactions {
		level[i] = TransactionHash(tx)
	}

	proof := make([]MerkleProofStep, 0)
	currentIndex := index
	for len(level) > 1 {
		if currentIndex%2 == 0 {
			siblingIndex := currentIndex + 1
			if siblingIndex >= len(level) {
				siblingIndex = currentIndex
			}
			proof = append(proof, MerkleProofStep{Position: "right", Hash: level[siblingIndex]})
		} else {
			siblingIndex := currentIndex - 1
			proof = append(proof, MerkleProofStep{Position: "left", Hash: level[siblingIndex]})
		}

		level = nextMerkleLevel(level)
		currentIndex /= 2
	}

	return proof, nil
}

// VerifyMerkleProof reconstructs the Merkle root from a transaction leaf hash
// and a proof path.
func VerifyMerkleProof(transactionHash string, proof []MerkleProofStep, root string) bool {
	current := transactionHash
	for _, step := range proof {
		switch step.Position {
		case "left":
			current = hashMerklePair(step.Hash, current)
		case "right":
			current = hashMerklePair(current, step.Hash)
		default:
			return false
		}
	}
	return current == root
}

func nextMerkleLevel(level []string) []string {
	next := make([]string, 0, (len(level)+1)/2)
	for i := 0; i < len(level); i += 2 {
		left := level[i]
		right := left
		if i+1 < len(level) {
			right = level[i+1]
		}
		next = append(next, hashMerklePair(left, right))
	}
	return next
}

func hashMerklePair(left, right string) string {
	payload := []byte("left=" + left + "\nright=" + right + "\n")
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
