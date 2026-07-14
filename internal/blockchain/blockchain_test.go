package blockchain

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGenesisBlockDeterministic(t *testing.T) {
	first := NewGenesisBlock()
	second := NewGenesisBlock()
	if first.Hash != second.Hash || first.Nonce != second.Nonce {
		t.Fatalf("genesis not deterministic")
	}
	if first.PrevHash != GenesisPreviousHash || first.Height != 0 || len(first.Transactions) != 0 {
		t.Fatalf("invalid genesis block: %+v", first)
	}
}

func TestBlockHashDeterministic(t *testing.T) {
	alice := testWallet(t)
	bob := testWallet(t)
	tx := signedTestTransfer(t, alice, bob.Address, 10, 1, "pay", time.Unix(100, 0))
	block := Block{Height: 1, Timestamp: 200, Difficulty: 2, Transactions: []Transaction{tx}, PrevHash: GenesisHash, Nonce: 42}
	first := block.ComputeHash()
	second := block.ComputeHash()
	if first != second {
		t.Fatalf("hash not deterministic: %s != %s", first, second)
	}
}

func TestMineBlockMeetsDifficulty(t *testing.T) {
	tx, err := NewFaucet("alice", 100, "seed", time.Unix(100, 0))
	if err != nil {
		t.Fatalf("create faucet: %v", err)
	}
	candidate := NewCandidateBlock(NewGenesisBlock(), []Transaction{tx}, time.Unix(200, 0), 2)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mined, _, err := Mine(ctx, candidate, 2, 1)
	if err != nil {
		t.Fatalf("mine: %v", err)
	}
	if !MeetsDifficulty(mined.Hash, 2) {
		t.Fatalf("hash %s does not meet difficulty", mined.Hash)
	}
	if mined.ComputeHash() != mined.Hash {
		t.Fatalf("nonce does not reproduce hash")
	}
}

func TestHonestChainValidates(t *testing.T) {
	state := NewState()
	cfg := Config{Difficulty: 1, MaxBlockTx: 5, Workers: 1}
	alice := testWallet(t)
	bob := testWallet(t)

	seed, err := NewFaucet(alice.Address, 100, "seed", time.Unix(100, 0))
	if err != nil {
		t.Fatalf("create faucet: %v", err)
	}
	if err := state.AddPending(seed); err != nil {
		t.Fatalf("add faucet: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := state.MinePending(ctx, cfg, time.Unix(200, 0)); err != nil {
		t.Fatalf("mine faucet: %v", err)
	}

	transfer := signedTestTransfer(t, alice, bob.Address, 40, 1, "pay", time.Unix(300, 0))
	if err := state.AddPending(transfer); err != nil {
		t.Fatalf("add transfer: %v", err)
	}
	if _, _, err := state.MinePending(ctx, cfg, time.Unix(400, 0)); err != nil {
		t.Fatalf("mine transfer: %v", err)
	}

	if err := ValidateChain(state.Chain, cfg.Difficulty); err != nil {
		t.Fatalf("validate honest chain: %v", err)
	}
	balances, err := state.Balances()
	if err != nil {
		t.Fatalf("balances: %v", err)
	}
	if balances[alice.Address] != 60 || balances[bob.Address] != 40 {
		t.Fatalf("balances = %#v, want alice 60 bob 40", balances)
	}
}

func TestTamperingDetected(t *testing.T) {
	state := buildTwoBlockTestChain(t)
	state.Chain[1].Transactions[0].Amount = 999
	err := ValidateChain(state.Chain, 1)
	if err == nil {
		t.Fatal("tampered chain unexpectedly valid")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || validationErr.Check != "merkle root" || validationErr.Height != 1 {
		t.Fatalf("error = %v, want merkle root validation error at block 1", err)
	}
}

func TestOverspendingTransactionRejected(t *testing.T) {
	state := NewState()
	cfg := Config{Difficulty: 1, MaxBlockTx: 5, Workers: 1}
	alice := testWallet(t)
	bob := testWallet(t)
	seed, err := NewFaucet(alice.Address, 100, "seed", time.Unix(100, 0))
	if err != nil {
		t.Fatalf("create faucet: %v", err)
	}
	if err := state.AddPending(seed); err != nil {
		t.Fatalf("add seed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := state.MinePending(ctx, cfg, time.Unix(200, 0)); err != nil {
		t.Fatalf("mine seed: %v", err)
	}

	tx := signedTestTransfer(t, alice, bob.Address, 150, 1, "overspend", time.Unix(300, 0))
	if err := state.AddPending(tx); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("error = %v, want ErrInsufficientFunds", err)
	}
}
