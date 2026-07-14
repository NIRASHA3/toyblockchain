package blockchain

import (
	"testing"
	"time"
)

func TestComputeMerkleRootDeterministic(t *testing.T) {
	alice := testWallet(t)
	bob := testWallet(t)
	first := signedTestTransfer(t, alice, bob.Address, 10, 1, "first", time.Unix(100, 0))
	second, err := NewFaucet(alice.Address, 25, "seed", time.Unix(101, 0))
	if err != nil {
		t.Fatalf("create faucet: %v", err)
	}

	txs := []Transaction{first, second}
	rootA := ComputeMerkleRoot(txs)
	rootB := ComputeMerkleRoot(txs)
	if rootA != rootB {
		t.Fatalf("merkle root not deterministic: %s != %s", rootA, rootB)
	}
	if rootA == EmptyMerkleRoot {
		t.Fatal("non-empty transaction list returned empty merkle root")
	}
}

func TestComputeMerkleRootChangesWhenTransactionChanges(t *testing.T) {
	alice := testWallet(t)
	bob := testWallet(t)
	tx := signedTestTransfer(t, alice, bob.Address, 10, 1, "pay", time.Unix(100, 0))
	root := ComputeMerkleRoot([]Transaction{tx})
	tx.Amount = 11
	changed := ComputeMerkleRoot([]Transaction{tx})
	if root == changed {
		t.Fatal("merkle root did not change after transaction mutation")
	}
}

func TestEmptyMerkleRoot(t *testing.T) {
	if ComputeMerkleRoot(nil) != EmptyMerkleRoot {
		t.Fatalf("nil transactions root = %s, want %s", ComputeMerkleRoot(nil), EmptyMerkleRoot)
	}
	if ComputeMerkleRoot([]Transaction{}) != EmptyMerkleRoot {
		t.Fatalf("empty transactions root = %s, want %s", ComputeMerkleRoot([]Transaction{}), EmptyMerkleRoot)
	}
}
