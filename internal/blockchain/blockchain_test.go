package blockchain

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGenesisIsDeterministic(t *testing.T) {
	state := NewState()
	if got, want := len(state.Chain), 1; got != want {
		t.Fatalf("chain length = %d, want %d", got, want)
	}
	genesis := state.Chain[0]
	if genesis.Height != 0 {
		t.Fatalf("genesis height = %d, want 0", genesis.Height)
	}
	if genesis.PrevHash != GenesisPreviousHash {
		t.Fatalf("genesis previous hash = %s, want %s", genesis.PrevHash, GenesisPreviousHash)
	}
	if got := genesis.ComputeHash(); got != genesis.Hash {
		t.Fatalf("genesis hash recompute = %s, stored = %s", got, genesis.Hash)
	}
	if !MeetsDifficulty(genesis.Hash, MaxDifficulty) {
		t.Fatalf("genesis hash %s does not satisfy max supported difficulty %d", genesis.Hash, MaxDifficulty)
	}
}

func TestBlockHashingIsDeterministic(t *testing.T) {
	tx, err := NewFaucet("alice", 100, "seed", time.Unix(100, 0))
	if err != nil {
		t.Fatalf("create faucet tx: %v", err)
	}
	block := Block{Height: 1, Timestamp: 200, Transactions: []Transaction{tx}, PrevHash: GenesisHash, Nonce: 42}
	first := block.ComputeHash()
	second := block.ComputeHash()
	if first != second {
		t.Fatalf("hashes differ: %s != %s", first, second)
	}
}

func TestMinedBlockSatisfiesDifficulty(t *testing.T) {
	tx, err := NewFaucet("alice", 100, "seed", time.Unix(100, 0))
	if err != nil {
		t.Fatalf("create faucet tx: %v", err)
	}
	candidate := NewCandidateBlock(NewGenesisBlock(), []Transaction{tx}, time.Unix(200, 0))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mined, _, err := Mine(ctx, candidate, 2, 2)
	if err != nil {
		t.Fatalf("mine: %v", err)
	}
	if !MeetsDifficulty(mined.Hash, 2) {
		t.Fatalf("mined hash %s does not satisfy difficulty", mined.Hash)
	}
	if recomputed := mined.ComputeHash(); recomputed != mined.Hash {
		t.Fatalf("nonce does not reproduce hash: recomputed %s stored %s", recomputed, mined.Hash)
	}
}

func TestHonestChainValidates(t *testing.T) {
	state := NewState()
	cfg := Config{Difficulty: 2, MaxBlockTx: 5, Workers: 2}

	faucetTx, err := NewFaucet("alice", 100, "seed", time.Unix(100, 0))
	if err != nil {
		t.Fatalf("create faucet tx: %v", err)
	}
	if err := state.AddPending(faucetTx); err != nil {
		t.Fatalf("add faucet pending: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := state.MinePending(ctx, cfg, time.Unix(200, 0)); err != nil {
		t.Fatalf("mine faucet block: %v", err)
	}

	transferTx, err := NewTransfer("alice", "bob", 40, "pay", time.Unix(300, 0))
	if err != nil {
		t.Fatalf("create transfer tx: %v", err)
	}
	if err := state.AddPending(transferTx); err != nil {
		t.Fatalf("add transfer pending: %v", err)
	}
	if _, _, err := state.MinePending(ctx, cfg, time.Unix(400, 0)); err != nil {
		t.Fatalf("mine transfer block: %v", err)
	}

	if err := ValidateChain(state.Chain, cfg.Difficulty); err != nil {
		t.Fatalf("valid chain rejected: %v", err)
	}
}

func TestTamperingIsDetectedAtFirstBadBlock(t *testing.T) {
	state := NewState()
	cfg := Config{Difficulty: 2, MaxBlockTx: 5, Workers: 2}
	tx, err := NewFaucet("alice", 100, "seed", time.Unix(100, 0))
	if err != nil {
		t.Fatalf("create faucet tx: %v", err)
	}
	if err := state.AddPending(tx); err != nil {
		t.Fatalf("add pending: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := state.MinePending(ctx, cfg, time.Unix(200, 0)); err != nil {
		t.Fatalf("mine: %v", err)
	}

	state.Chain[1].Transactions[0].Amount = 999
	err = ValidateChain(state.Chain, cfg.Difficulty)
	if err == nil {
		t.Fatal("tampered chain unexpectedly validated")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *ValidationError", err)
	}
	if validationErr.Height != 1 || validationErr.Check != "hash" {
		t.Fatalf("validation error = height %d check %q, want height 1 hash", validationErr.Height, validationErr.Check)
	}
}

func TestOverspendingTransactionIsRejectedAndBalanceUnchanged(t *testing.T) {
	state := NewState()
	cfg := Config{Difficulty: 2, MaxBlockTx: 5, Workers: 2}

	seed, err := NewFaucet("alice", 100, "seed", time.Unix(100, 0))
	if err != nil {
		t.Fatalf("create faucet tx: %v", err)
	}
	if err := state.AddPending(seed); err != nil {
		t.Fatalf("add seed pending: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := state.MinePending(ctx, cfg, time.Unix(200, 0)); err != nil {
		t.Fatalf("mine seed: %v", err)
	}

	before, err := state.Balances()
	if err != nil {
		t.Fatalf("balances before: %v", err)
	}
	bad, err := NewTransfer("alice", "bob", 150, "overspend", time.Unix(300, 0))
	if err != nil {
		t.Fatalf("create bad tx: %v", err)
	}
	if err := state.AddPending(bad); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("overspend error = %v, want ErrInsufficientFunds", err)
	}
	after, err := state.Balances()
	if err != nil {
		t.Fatalf("balances after: %v", err)
	}
	if before["alice"] != 100 || after["alice"] != 100 {
		t.Fatalf("balance changed: before=%d after=%d", before["alice"], after["alice"])
	}
	if len(state.Pending) != 0 {
		t.Fatalf("pending length = %d, want 0", len(state.Pending))
	}
}
