package blockchain

import (
	"context"
	"fmt"
	"time"
)

// State is the persisted local node state: confirmed chain plus pending transactions.
type State struct {
	Chain   []Block       `json:"chain"`
	Pending []Transaction `json:"pending"`
}

// NewState creates a fresh blockchain containing only the deterministic genesis block.
func NewState() State {
	return State{Chain: []Block{NewGenesisBlock()}, Pending: []Transaction{}}
}

// NextNonce returns the next valid nonce for an account after confirmed and pending transactions.
func (s State) NextNonce(account string) (uint64, error) {
	ledger, err := LedgerFromChain(s.Chain)
	if err != nil {
		return 0, fmt.Errorf("derive ledger: %w", err)
	}
	for _, pendingTx := range s.Pending {
		if err := ledger.ApplyTransaction(pendingTx); err != nil {
			return 0, fmt.Errorf("existing pending transaction is invalid: %w", err)
		}
	}
	return ledger.Nonces[account] + 1, nil
}

// AddPending validates a transaction against confirmed and already-pending ledger state.
func (s *State) AddPending(tx Transaction) error {
	ledger, err := LedgerFromChain(s.Chain)
	if err != nil {
		return fmt.Errorf("derive ledger: %w", err)
	}
	for _, pendingTx := range s.Pending {
		if err := ledger.ApplyTransaction(pendingTx); err != nil {
			return fmt.Errorf("existing pending transaction is invalid: %w", err)
		}
	}
	if err := ledger.ApplyTransaction(tx); err != nil {
		return fmt.Errorf("reject transaction: %w", err)
	}
	s.Pending = append(s.Pending, tx)
	return nil
}

// MinePending mines up to cfg.MaxBlockTx pending transactions into a new block.
func (s *State) MinePending(ctx context.Context, cfg Config, now time.Time) (Block, MiningStats, error) {
	if err := cfg.Validate(); err != nil {
		return Block{}, MiningStats{}, err
	}
	if len(s.Pending) == 0 {
		return Block{}, MiningStats{}, ErrNoPendingTx
	}

	limit := cfg.MaxBlockTx
	if len(s.Pending) < limit {
		limit = len(s.Pending)
	}
	batch := make([]Transaction, limit)
	copy(batch, s.Pending[:limit])

	ledger, err := LedgerFromChain(s.Chain)
	if err != nil {
		return Block{}, MiningStats{}, fmt.Errorf("derive ledger: %w", err)
	}
	for _, tx := range batch {
		if err := ledger.ApplyTransaction(tx); err != nil {
			return Block{}, MiningStats{}, fmt.Errorf("pending batch is invalid: %w", err)
		}
	}

	nextDifficulty, err := cfg.NextDifficulty(s.Chain)
	if err != nil {
		return Block{}, MiningStats{}, err
	}
	candidate := NewCandidateBlock(s.Chain[len(s.Chain)-1], batch, now, nextDifficulty)
	mined, stats, err := Mine(ctx, candidate, nextDifficulty, cfg.Workers)
	if err != nil {
		return Block{}, stats, err
	}

	s.Chain = append(s.Chain, mined)
	s.Pending = append([]Transaction{}, s.Pending[limit:]...)
	return mined, stats, nil
}

// Balances returns confirmed balances, excluding pending transactions.
func (s State) Balances() (Balances, error) {
	return BalancesFromChain(s.Chain)
}

// BalancesIncludingPending returns balances after confirmed and pending transactions.
func (s State) BalancesIncludingPending() (Balances, error) {
	ledger, err := LedgerFromChain(s.Chain)
	if err != nil {
		return nil, err
	}
	for _, tx := range s.Pending {
		if err := ledger.ApplyTransaction(tx); err != nil {
			return nil, fmt.Errorf("apply pending: %w", err)
		}
	}
	return ledger.Balances, nil
}
