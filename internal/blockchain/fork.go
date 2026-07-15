package blockchain

import "fmt"

// ForkDecision describes the outcome of comparing a local chain with a competing chain.
type ForkDecision string

const (
	ForkDecisionKeepLocal      ForkDecision = "keep_local"
	ForkDecisionAdoptCandidate ForkDecision = "adopt_candidate"
)

// ForkResolutionResult summarises a longest-valid-chain fork resolution attempt.
type ForkResolutionResult struct {
	Decision        ForkDecision `json:"decision"`
	Adopted         bool         `json:"adopted"`
	LocalHeight     int          `json:"local_height"`
	CandidateHeight int          `json:"candidate_height"`
	KeptPending     int          `json:"kept_pending"`
	DroppedPending  int          `json:"dropped_pending"`
	Reason          string       `json:"reason"`
}

// ResolveFork validates the local and candidate states, then applies a longest-valid-chain rule.
//
// If the candidate confirmed chain is strictly longer than the local confirmed chain,
// the candidate chain is adopted. The local pending pool is then replayed on top of
// the adopted chain, keeping only transactions that remain valid. Candidate pending
// transactions are intentionally not imported, because the fork-choice rule applies
// to confirmed blocks, not to a peer's mempool.
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
	if candidateHeight <= localHeight {
		return local, ForkResolutionResult{
			Decision:        ForkDecisionKeepLocal,
			Adopted:         false,
			LocalHeight:     localHeight,
			CandidateHeight: candidateHeight,
			KeptPending:     len(local.Pending),
			DroppedPending:  0,
			Reason:          "candidate chain is not longer than local chain",
		}, nil
	}

	pending, dropped, err := filterPendingForChain(candidate.Chain, local.Pending)
	if err != nil {
		return State{}, ForkResolutionResult{}, err
	}
	adopted := State{Chain: copyChain(candidate.Chain), Pending: pending}
	return adopted, ForkResolutionResult{
		Decision:        ForkDecisionAdoptCandidate,
		Adopted:         true,
		LocalHeight:     localHeight,
		CandidateHeight: candidateHeight,
		KeptPending:     len(pending),
		DroppedPending:  dropped,
		Reason:          "candidate chain is longer and valid",
	}, nil
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
	copy(copied, chain)
	return copied
}
