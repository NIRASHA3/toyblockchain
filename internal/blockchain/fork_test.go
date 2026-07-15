package blockchain

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestResolveForkAdoptsLongerValidCandidate(t *testing.T) {
	cfg := Config{Difficulty: 1, MaxBlockTx: 5, RetargetInterval: 0}
	local := NewState()
	candidate := NewState()
	mineFaucetBlock(t, &local, cfg, "alice", 10)
	mineFaucetBlock(t, &candidate, cfg, "bob", 10)
	mineFaucetBlock(t, &candidate, cfg, "bob", 5)

	resolved, result, err := ResolveFork(local, candidate, cfg)
	if err != nil {
		t.Fatalf("ResolveFork returned error: %v", err)
	}
	if !result.Adopted || result.Decision != ForkDecisionAdoptCandidate {
		t.Fatalf("expected candidate adoption, got %+v", result)
	}
	if got, want := len(resolved.Chain), len(candidate.Chain); got != want {
		t.Fatalf("resolved chain length = %d, want %d", got, want)
	}
	if resolved.Chain[len(resolved.Chain)-1].Hash != candidate.Chain[len(candidate.Chain)-1].Hash {
		t.Fatalf("resolved chain tip does not match candidate tip")
	}
}

func TestResolveForkKeepsLocalWhenCandidateIsNotLonger(t *testing.T) {
	cfg := Config{Difficulty: 1, MaxBlockTx: 5, RetargetInterval: 0}
	local := NewState()
	candidate := NewState()
	mineFaucetBlock(t, &local, cfg, "alice", 10)
	mineFaucetBlock(t, &candidate, cfg, "bob", 10)

	resolved, result, err := ResolveFork(local, candidate, cfg)
	if err != nil {
		t.Fatalf("ResolveFork returned error: %v", err)
	}
	if result.Adopted || result.Decision != ForkDecisionKeepLocal {
		t.Fatalf("expected local chain to be kept, got %+v", result)
	}
	if resolved.Chain[len(resolved.Chain)-1].Hash != local.Chain[len(local.Chain)-1].Hash {
		t.Fatalf("resolved chain tip changed unexpectedly")
	}
}

func TestResolveForkRejectsInvalidCandidate(t *testing.T) {
	cfg := Config{Difficulty: 1, MaxBlockTx: 5, RetargetInterval: 0}
	local := NewState()
	candidate := NewState()
	mineFaucetBlock(t, &candidate, cfg, "bob", 10)
	mineFaucetBlock(t, &candidate, cfg, "bob", 5)
	candidate.Chain[1].Transactions[0].Amount = 999

	_, _, err := ResolveFork(local, candidate, cfg)
	if err == nil {
		t.Fatalf("expected invalid candidate to be rejected")
	}
	if !strings.Contains(err.Error(), "candidate chain is invalid") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveForkFiltersConflictingPendingTransactions(t *testing.T) {
	cfg := Config{Difficulty: 1, MaxBlockTx: 5, RetargetInterval: 0}
	wallet, err := NewWallet()
	if err != nil {
		t.Fatalf("NewWallet: %v", err)
	}
	local := NewState()
	candidate := NewState()

	mineFaucetBlock(t, &local, cfg, wallet.Address, 10)
	mineFaucetBlock(t, &candidate, cfg, wallet.Address, 10)
	localTx, err := NewSignedTransfer(wallet, "local-recipient", 3, 1, "local pending", time.Unix(10, 0))
	if err != nil {
		t.Fatalf("NewSignedTransfer local: %v", err)
	}
	if err := local.AddPending(localTx); err != nil {
		t.Fatalf("AddPending local: %v", err)
	}

	confirmedTx, err := NewSignedTransfer(wallet, "candidate-recipient", 4, 1, "candidate confirmed", time.Unix(11, 0))
	if err != nil {
		t.Fatalf("NewSignedTransfer candidate: %v", err)
	}
	if err := candidate.AddPending(confirmedTx); err != nil {
		t.Fatalf("AddPending candidate: %v", err)
	}
	minePendingAt(t, &candidate, cfg, time.Unix(12, 0))

	resolved, result, err := ResolveFork(local, candidate, cfg)
	if err != nil {
		t.Fatalf("ResolveFork returned error: %v", err)
	}
	if !result.Adopted {
		t.Fatalf("expected candidate adoption, got %+v", result)
	}
	if result.KeptPending != 0 || result.DroppedPending != 1 || len(resolved.Pending) != 0 {
		t.Fatalf("expected conflicting local pending tx to be dropped, result=%+v pending=%d", result, len(resolved.Pending))
	}
}

func mineFaucetBlock(t *testing.T, state *State, cfg Config, to string, amount int64) {
	t.Helper()
	tx, err := NewFaucet(to, amount, "test funding", time.Unix(int64(len(state.Chain)), 0))
	if err != nil {
		t.Fatalf("NewFaucet: %v", err)
	}
	if err := state.AddPending(tx); err != nil {
		t.Fatalf("AddPending: %v", err)
	}
	minePendingAt(t, state, cfg, time.Unix(int64(len(state.Chain)), 0))
}

func minePendingAt(t *testing.T, state *State, cfg Config, now time.Time) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, _, err := state.MinePending(ctx, cfg, now); err != nil {
		t.Fatalf("MinePending: %v", err)
	}
}
