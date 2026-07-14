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

func TestMerkleProofVerifies(t *testing.T) {
	alice := testWallet(t)
	bob := testWallet(t)
	charlie := testWallet(t)
	txs := []Transaction{
		signedTestTransfer(t, alice, bob.Address, 10, 1, "one", time.Unix(100, 0)),
		signedTestTransfer(t, bob, charlie.Address, 5, 1, "two", time.Unix(101, 0)),
		signedTestTransfer(t, charlie, alice.Address, 2, 1, "three", time.Unix(102, 0)),
	}

	root := ComputeMerkleRoot(txs)
	for i, tx := range txs {
		proof, err := BuildMerkleProof(txs, i)
		if err != nil {
			t.Fatalf("proof for tx %d: %v", i, err)
		}
		if !VerifyMerkleProof(TransactionHash(tx), proof, root) {
			t.Fatalf("proof for tx %d did not verify", i)
		}
	}
}

func TestMerkleProofRejectsWrongRoot(t *testing.T) {
	alice := testWallet(t)
	bob := testWallet(t)
	tx := signedTestTransfer(t, alice, bob.Address, 10, 1, "pay", time.Unix(100, 0))
	proof, err := BuildMerkleProof([]Transaction{tx}, 0)
	if err != nil {
		t.Fatalf("proof: %v", err)
	}
	if VerifyMerkleProof(TransactionHash(tx), proof, EmptyMerkleRoot) {
		t.Fatal("proof verified against wrong root")
	}
}

func TestMerkleProofRejectsOutOfRangeIndex(t *testing.T) {
	alice := testWallet(t)
	bob := testWallet(t)
	tx := signedTestTransfer(t, alice, bob.Address, 10, 1, "pay", time.Unix(100, 0))
	if _, err := BuildMerkleProof([]Transaction{tx}, 1); err == nil {
		t.Fatal("expected out-of-range proof error")
	}
}
