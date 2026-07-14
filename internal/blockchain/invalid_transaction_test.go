package blockchain

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

func TestRejectZeroAndNegativeTransactionAmounts(t *testing.T) {
	now := time.Unix(100, 0)
	wallet := testWallet(t)
	recipient := testWallet(t)
	tests := []struct {
		name string
		make func() (Transaction, error)
	}{
		{name: "transfer zero amount", make: func() (Transaction, error) { return NewSignedTransfer(wallet, recipient.Address, 0, 1, "", now) }},
		{name: "transfer negative amount", make: func() (Transaction, error) { return NewSignedTransfer(wallet, recipient.Address, -10, 1, "", now) }},
		{name: "faucet zero amount", make: func() (Transaction, error) { return NewFaucet(wallet.Address, 0, "", now) }},
		{name: "faucet negative amount", make: func() (Transaction, error) { return NewFaucet(wallet.Address, -10, "", now) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.make(); !errors.Is(err, ErrInvalidAmount) {
				t.Fatalf("error = %v, want ErrInvalidAmount", err)
			}
		})
	}
}

func TestRejectBlankTransactionAccounts(t *testing.T) {
	now := time.Unix(100, 0)
	wallet := testWallet(t)
	tests := []struct {
		name string
		make func() (Transaction, error)
	}{
		{name: "blank transfer recipient", make: func() (Transaction, error) { return NewSignedTransfer(wallet, "", 10, 1, "", now) }},
		{name: "blank faucet recipient", make: func() (Transaction, error) { return NewFaucet("", 10, "", now) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.make(); !errors.Is(err, ErrInvalidTransaction) {
				t.Fatalf("error = %v, want ErrInvalidTransaction", err)
			}
		})
	}
}

func TestRejectTransactionIDMismatch(t *testing.T) {
	tx, err := NewFaucet("alice", 100, "seed", time.Unix(100, 0))
	if err != nil {
		t.Fatalf("create faucet tx: %v", err)
	}
	tx.ID = "tampered-id"

	balances := make(Balances)
	if err := ApplyTransaction(balances, tx); !errors.Is(err, ErrInvalidTransaction) {
		t.Fatalf("error = %v, want ErrInvalidTransaction", err)
	}
	if balances["alice"] != 0 {
		t.Fatalf("balance changed after rejected transaction: got %d", balances["alice"])
	}
}

func TestRejectBalanceOverflowForFaucet(t *testing.T) {
	balances := Balances{"alice": math.MaxInt64}
	tx, err := NewFaucet("alice", 1, "overflow", time.Unix(100, 0))
	if err != nil {
		t.Fatalf("create faucet tx: %v", err)
	}

	if err := ApplyTransaction(balances, tx); !errors.Is(err, ErrBalanceOverflow) {
		t.Fatalf("error = %v, want ErrBalanceOverflow", err)
	}
	if balances["alice"] != math.MaxInt64 {
		t.Fatalf("balance changed after overflow rejection: got %d", balances["alice"])
	}
}

func TestRejectBalanceOverflowForTransferRecipientWithoutDebitingSender(t *testing.T) {
	alice := testWallet(t)
	bob := testWallet(t)
	balances := Balances{alice.Address: 100, bob.Address: math.MaxInt64 - 10}
	tx := signedTestTransfer(t, alice, bob.Address, 11, 1, "overflow", time.Unix(100, 0))

	if err := ApplyTransaction(balances, tx); !errors.Is(err, ErrBalanceOverflow) {
		t.Fatalf("error = %v, want ErrBalanceOverflow", err)
	}
	if balances[alice.Address] != 100 || balances[bob.Address] != math.MaxInt64-10 {
		t.Fatalf("balances changed after overflow rejection: alice=%d bob=%d", balances[alice.Address], balances[bob.Address])
	}
}

func TestPendingPoolPreventsOverspendAcrossPendingTransactions(t *testing.T) {
	state := NewState()
	cfg := Config{Difficulty: 1, MaxBlockTx: 5, Workers: 1}
	alice := testWallet(t)
	bob := testWallet(t)
	charlie := testWallet(t)

	seed, err := NewFaucet(alice.Address, 100, "seed", time.Unix(100, 0))
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

	firstSpend := signedTestTransfer(t, alice, bob.Address, 70, 1, "first", time.Unix(300, 0))
	if err := state.AddPending(firstSpend); err != nil {
		t.Fatalf("add first spend: %v", err)
	}

	secondSpend := signedTestTransfer(t, alice, charlie.Address, 40, 2, "second", time.Unix(400, 0))
	if err := state.AddPending(secondSpend); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("second pending spend error = %v, want ErrInsufficientFunds", err)
	}
	if len(state.Pending) != 1 {
		t.Fatalf("pending length = %d, want 1", len(state.Pending))
	}
}

func TestValidateChainDetectsInvalidAmountInsideStoredBlock(t *testing.T) {
	badTx := Transaction{From: FaucetAccount, To: "alice", Amount: 0, CreatedAt: time.Unix(100, 0).UnixNano()}
	badTx.ID = badTx.ComputeID()

	candidate := NewCandidateBlock(NewGenesisBlock(), []Transaction{badTx}, time.Unix(200, 0), 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mined, _, err := Mine(ctx, candidate, 1, 1)
	if err != nil {
		t.Fatalf("mine invalid block for validation test: %v", err)
	}

	chain := []Block{NewGenesisBlock(), mined}
	err = ValidateChain(chain, 1)
	if err == nil {
		t.Fatal("chain with invalid transaction amount unexpectedly validated")
	}

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *ValidationError", err)
	}
	if validationErr.Height != 1 || validationErr.Check != "ledger" {
		t.Fatalf("validation error = height %d check %q, want height 1 ledger", validationErr.Height, validationErr.Check)
	}
	if !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("error = %v, want wrapped ErrInvalidAmount", err)
	}
}

func TestRejectDuplicateTransactionID(t *testing.T) {
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
	spend := signedTestTransfer(t, alice, bob.Address, 40, 1, "pay", time.Unix(300, 0))
	if err := state.AddPending(spend); err != nil {
		t.Fatalf("add spend: %v", err)
	}
	if _, _, err := state.MinePending(ctx, cfg, time.Unix(400, 0)); err != nil {
		t.Fatalf("mine spend: %v", err)
	}

	state.Chain[2].Transactions = append(state.Chain[2].Transactions, spend)
	state.Chain[2].MerkleRoot = ComputeMerkleRoot(state.Chain[2].Transactions)
	state.Chain[2].Hash = state.Chain[2].ComputeHash()
	for !MeetsDifficulty(state.Chain[2].Hash, state.Chain[2].Difficulty) {
		state.Chain[2].Nonce++
		state.Chain[2].Hash = state.Chain[2].ComputeHash()
	}
	err = ValidateChain(state.Chain, 1)
	if !errors.Is(err, ErrDuplicateTx) {
		t.Fatalf("error = %v, want ErrDuplicateTx", err)
	}
}
