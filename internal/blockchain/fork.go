package blockchain

import "fmt"

// ForkDecision describes the outcome of comparing a local chain with a competing chain.
type ForkDecision string

const (
	ForkDecisionKeepLocal      ForkDecision = "keep_local"
	ForkDecisionAdoptCandidate ForkDecision = "adopt_candidate"
)

// ForkResolutionResult summarises a heaviest-valid-chain fork resolution attempt.
type ForkResolutionResult struct {
	Decision        ForkDecision `json:"decision"`
	Adopted         bool         `json:"adopted"`
	LocalHeight     int          `json:"local_height"`
	CandidateHeight int          `json:"candidate_height"`
	LocalWork       uint64       `json:"local_work"`
	CandidateWork   uint64       `json:"candidate_work"`
	KeptPending     int          `json:"kept_pending"`
	DroppedPending  int          `json:"dropped_pending"`
	Reason          string       `json:"reason"`
}

// ResolveFork validates the local and candidate states, then applies a heaviest-valid-chain rule.
//
// The candidate chain is adopted only when it has more cumulative proof-of-work than
// the local chain. The local pending pool is then replayed on top of the adopted
// chain, keeping only transactions that remain valid. Candidate pending transactions
// are intentionally not imported, because the fork-choice rule applies to confirmed
// blocks, not to a peer's mempool.
func ResolveFork(local State, candidate State, cfg Config) (State, ForkResolutionResult, error) {
	if err := cfg.Validate(); err != nil {
		return State{}, ForkResolutionResult{}, err
	}
	if err := ValidateChainWithConfig(local.Chain, cfg); err != nil {
		return State{}, ForkResolutionResult{}, fmt.Errorf("local chain is invalid: %w", err)
	}
	if err := ValidateChainWithConfig(candidate.Chain, cfg); err != nil {
		return State{}, ForkResolutionResult{}, fmt.Errorf("candidate chain is invalid: %w", err)
	}
	localHeight := len(local.Chain) - 1
	candidateHeight := len(candidate.Chain) - 1
	localWork := CumulativeChainWork(local.Chain)
	candidateWork := CumulativeChainWork(candidate.Chain)
	if candidateWork <= localWork {
		return local, ForkResolutionResult{
			Decision:        ForkDecisionKeepLocal,
			Adopted:         false,
			LocalHeight:     localHeight,
			CandidateHeight: candidateHeight,
			LocalWork:       localWork,
			CandidateWork:   candidateWork,
			KeptPending:     len(local.Pending),
			DroppedPending:  0,
			Reason:          "candidate chain does not have more cumulative proof-of-work than local chain",
		}, nil
	}
	pending, dropped, err := filterPendingForChain(candidate.Chain, local.Pending)
	if err != nil {
		return State{}, ForkResolutionResult{}, err
	}
	adopted := State{
		Chain:   copyChain(candidate.Chain),
		Pending: pending,
	}
	return adopted, ForkResolutionResult{
		Decision:        ForkDecisionAdoptCandidate,
		Adopted:         true,
		LocalHeight:     localHeight,
		CandidateHeight: candidateHeight,
		LocalWork:       localWork,
		CandidateWork:   candidateWork,
		KeptPending:     len(pending),
		DroppedPending:  dropped,
		Reason:          "candidate chain has more cumulative proof-of-work and is valid",
	}, nil
}

// CumulativeChainWork returns the total estimated proof-of-work represented by a chain.
//
// This toy blockchain defines difficulty as the number of leading zero hexadecimal
// digits required in a block hash. Each additional leading hex zero makes the expected
// mining work about 16 times harder, so a block's estimated work is 16^difficulty.
func CumulativeChainWork(chain []Block) uint64 {
	var total uint64
	for _, block := range chain {
		work := blockWork(block)
		if total > maxUint64()-work {
			return maxUint64()
		}
		total += work
	}
	return total
}
func blockWork(block Block) uint64 {
	if block.Difficulty <= 0 {
		return 1
	}
	shift := uint(block.Difficulty * 4)
	if shift >= 63 {
		return maxUint64()
	}
	return uint64(1) << shift
}
func maxUint64() uint64 {
	return ^uint64(0)
}
func filterPendingForChain(chain []Block, pending []Transaction) ([]Transaction, int, error) {
	ledger, err := LedgerFromChain(chain)
	if err != nil {
		return nil, 0, fmt.Errorf("derive ledger for adopted chain: %w", err)
	}
	kept := make([]Transaction, 0, len(pending))
	dropped := 0
	for _, tx := range pending {
		if err := ledger.ApplyTransaction(tx); err != nil {
			dropped++
			continue
		}
		kept = append(kept, tx)
	}
	return kept, dropped, nil
}
func copyChain(chain []Block) []Block {
	copied := make([]Block, len(chain))
	for i, block := range chain {
		copied[i] = copyBlock(block)
	}
	return copied
}
func copyBlock(block Block) Block {
	block.Transactions = append([]Transaction(nil), block.Transactions...)
	return block
}
