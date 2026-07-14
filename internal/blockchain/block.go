package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Block is the append-only unit of the chain.
type Block struct {
	Height       int           `json:"height"`
	Timestamp    int64         `json:"timestamp"`
	Difficulty   int           `json:"difficulty"`
	Transactions []Transaction `json:"transactions"`
	MerkleRoot   string        `json:"merkle_root"`
	PrevHash     string        `json:"prev_hash"`
	Nonce        uint64        `json:"nonce"`
	Hash         string        `json:"hash"`
}

// NewGenesisBlock returns the deterministic, canonical genesis block.
func NewGenesisBlock() Block {
	return Block{
		Height:       0,
		Timestamp:    0,
		Difficulty:   MaxDifficulty,
		Transactions: []Transaction{},
		MerkleRoot:   EmptyMerkleRoot,
		PrevHash:     GenesisPreviousHash,
		Nonce:        GenesisNonce,
		Hash:         GenesisHash,
	}
}

// NewCandidateBlock creates a not-yet-mined block linked to prev.
func NewCandidateBlock(prev Block, transactions []Transaction, now time.Time, difficulty int) Block {
	copied := make([]Transaction, len(transactions))
	copy(copied, transactions)
	return Block{
		Height:       prev.Height + 1,
		Timestamp:    now.Unix(),
		Difficulty:   difficulty,
		Transactions: copied,
		MerkleRoot:   ComputeMerkleRoot(copied),
		PrevHash:     prev.Hash,
	}
}

// ComputeHash calculates SHA-256 over a stable serialisation of the block
// header fields except Hash. The Merkle root commits to the transaction list,
// so the block hash does not directly serialise every transaction.
func (b Block) ComputeHash() string {
	payload := b.canonicalPayload()
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (b Block) canonicalPayload() []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "height=%d\n", b.Height)
	fmt.Fprintf(&buf, "timestamp=%d\n", b.Timestamp)
	fmt.Fprintf(&buf, "difficulty=%d\n", b.Difficulty)
	writeCanonicalString(&buf, "prev_hash", b.PrevHash)
	writeCanonicalString(&buf, "merkle_root", b.MerkleRoot)
	fmt.Fprintf(&buf, "nonce=%d\n", b.Nonce)
	return buf.Bytes()
}

func writeCanonicalString(buf *bytes.Buffer, key, value string) {
	buf.WriteString(key)
	buf.WriteByte('=')
	buf.WriteString(strconv.Itoa(len(value)))
	buf.WriteByte(':')
	buf.WriteString(value)
	buf.WriteByte('\n')
}

// MeetsDifficulty checks whether hash begins with the requested number of zero hex digits.
func MeetsDifficulty(hash string, difficulty int) bool {
	if difficulty <= 0 {
		return true
	}
	return strings.HasPrefix(hash, strings.Repeat("0", difficulty))
}
