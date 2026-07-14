package blockchain

import (
	"context"
	"errors"
	"testing"
	"time"
)

func buildTwoBlockTestChain(t *testing.T) State {
	t.Helper()
	state := NewState()
	cfg := Config{Difficulty: 1, MaxBlockTx: 5, Workers: 1}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	alice := testWallet(t)
	bob := testWallet(t)

	seed, err := NewFaucet(alice.Address, 100, "seed", time.Unix(100, 0))
	if err != nil {
		t.Fatalf("create seed tx: %v", err)
	}
	if err := state.AddPending(seed); err != nil {
		t.Fatalf("add seed tx: %v", err)
	}
	if _, _, err := state.MinePending(ctx, cfg, time.Unix(200, 0)); err != nil {
		t.Fatalf("mine seed block: %v", err)
	}

	spend := signedTestTransfer(t, alice, bob.Address, 40, 1, "pay", time.Unix(300, 0))
	if err := state.AddPending(spend); err != nil {
		t.Fatalf("add spend tx: %v", err)
	}
	if _, _, err := state.MinePending(ctx, cfg, time.Unix(400, 0)); err != nil {
		t.Fatalf("mine spend block: %v", err)
	}
	return state
}

func TestBlockHashIncludesPreviousHash(t *testing.T) {
	block := Block{Height: 1, Timestamp: 100, Difficulty: 1, PrevHash: GenesisHash, Nonce: 42}
	original := block.ComputeHash()
	block.PrevHash = GenesisPreviousHash
	changed := block.ComputeHash()
	if original == changed {
		t.Fatal("block hash did not change after previous hash changed")
	}
}

func TestValidateChainDetectsPreviousHashLinkMismatch(t *testing.T) {
	state := buildTwoBlockTestChain(t)
	state.Chain[2].PrevHash = state.Chain[0].Hash
	state.Chain[2].Difficulty = 1
	state.Chain[2].Nonce = 0
	for {
		state.Chain[2].Hash = state.Chain[2].ComputeHash()
		if MeetsDifficulty(state.Chain[2].Hash, state.Chain[2].Difficulty) {
			break
		}
		state.Chain[2].Nonce++
	}

	err := ValidateChain(state.Chain, 1)
	if err == nil {
		t.Fatal("chain with broken previous-hash link unexpectedly validated")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *ValidationError", err)
	}
	if validationErr.Height != 2 || validationErr.Check != "previous-hash link" {
		t.Fatalf("validation error = height %d check %q, want height 2 previous-hash link", validationErr.Height, validationErr.Check)
	}
}

func TestValidateChainDetectsStoredHashMismatch(t *testing.T) {
	state := buildTwoBlockTestChain(t)
	state.Chain[1].Nonce++
	err := ValidateChain(state.Chain, 1)
	if err == nil {
		t.Fatal("chain with stored hash mismatch unexpectedly validated")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *ValidationError", err)
	}
	if validationErr.Height != 1 || validationErr.Check != "hash" {
		t.Fatalf("validation error = height %d check %q, want height 1 hash", validationErr.Height, validationErr.Check)
	}
}

func TestSaveLoadStateJSONRoundTrip(t *testing.T) {
	state := buildTwoBlockTestChain(t)
	path := t.TempDir() + "/chain.json"
	if err := SaveState(path, state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(loaded.Chain) != len(state.Chain) {
		t.Fatalf("loaded chain length = %d, want %d", len(loaded.Chain), len(state.Chain))
	}
	if loaded.Chain[2].Hash != state.Chain[2].Hash {
		t.Fatalf("loaded tip hash = %s, want %s", loaded.Chain[2].Hash, state.Chain[2].Hash)
	}
	if err := ValidateChain(loaded.Chain, 1); err != nil {
		t.Fatalf("loaded chain failed validation: %v", err)
	}
}
