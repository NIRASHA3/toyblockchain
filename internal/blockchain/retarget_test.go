package blockchain

import (
	"context"
	"errors"
	"testing"
	"time"
)

func mineFaucetBlockAt(t *testing.T, state *State, cfg Config, recipient string, now time.Time) Block {
	t.Helper()
	tx, err := NewFaucet(recipient, 1, "retarget test", now)
	if err != nil {
		t.Fatalf("create faucet: %v", err)
	}
	if err := state.AddPending(tx); err != nil {
		t.Fatalf("add pending: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	block, _, err := state.MinePending(ctx, cfg, now)
	if err != nil {
		t.Fatalf("mine block: %v", err)
	}
	return block
}

func TestDifficultyRetargetIncreasesAfterFastInterval(t *testing.T) {
	state := NewState()
	cfg := Config{Difficulty: 1, MaxBlockTx: 1, Workers: 1, RetargetInterval: 3, TargetBlockTime: 10 * time.Second}

	mineFaucetBlockAt(t, &state, cfg, "alice", time.Unix(100, 0))
	mineFaucetBlockAt(t, &state, cfg, "alice", time.Unix(101, 0))
	mineFaucetBlockAt(t, &state, cfg, "alice", time.Unix(102, 0))

	next, err := cfg.NextDifficulty(state.Chain)
	if err != nil {
		t.Fatalf("next difficulty: %v", err)
	}
	if next != 2 {
		t.Fatalf("next difficulty = %d, want 2", next)
	}

	block := mineFaucetBlockAt(t, &state, cfg, "alice", time.Unix(103, 0))
	if block.Difficulty != 2 {
		t.Fatalf("mined difficulty = %d, want 2", block.Difficulty)
	}
	if err := ValidateChainWithConfig(state.Chain, cfg); err != nil {
		t.Fatalf("validate retargeted chain: %v", err)
	}
}

func TestDifficultyRetargetDecreasesAfterSlowInterval(t *testing.T) {
	state := NewState()
	cfg := Config{Difficulty: 3, MaxBlockTx: 1, Workers: 1, RetargetInterval: 3, TargetBlockTime: 10 * time.Second}

	mineFaucetBlockAt(t, &state, cfg, "alice", time.Unix(100, 0))
	mineFaucetBlockAt(t, &state, cfg, "alice", time.Unix(200, 0))
	mineFaucetBlockAt(t, &state, cfg, "alice", time.Unix(300, 0))

	next, err := cfg.NextDifficulty(state.Chain)
	if err != nil {
		t.Fatalf("next difficulty: %v", err)
	}
	if next != 2 {
		t.Fatalf("next difficulty = %d, want 2", next)
	}
}

func TestValidateChainDetectsUnexpectedRetargetDifficulty(t *testing.T) {
	state := NewState()
	retargetCfg := Config{Difficulty: 1, MaxBlockTx: 1, Workers: 1, RetargetInterval: 3, TargetBlockTime: 10 * time.Second}
	disabledCfg := Config{Difficulty: 1, MaxBlockTx: 1, Workers: 1, RetargetInterval: 0}

	mineFaucetBlockAt(t, &state, retargetCfg, "alice", time.Unix(100, 0))
	mineFaucetBlockAt(t, &state, retargetCfg, "alice", time.Unix(101, 0))
	mineFaucetBlockAt(t, &state, retargetCfg, "alice", time.Unix(102, 0))

	// Mine the next block with retargeting disabled, so it incorrectly keeps difficulty 1
	// even though the configured retargeting rules expect difficulty 2.
	mineFaucetBlockAt(t, &state, disabledCfg, "alice", time.Unix(103, 0))

	err := ValidateChainWithConfig(state.Chain, retargetCfg)
	if err == nil {
		t.Fatal("chain with wrong retargeted difficulty unexpectedly valid")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || validationErr.Check != "difficulty retarget" || validationErr.Height != 4 {
		t.Fatalf("error = %v, want difficulty retarget validation error at block 4", err)
	}
}
