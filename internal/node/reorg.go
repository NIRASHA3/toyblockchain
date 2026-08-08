package node

import (
	"context"
	"fmt"
	"toyblockchain/internal/blockchain"
)

// ReorgResult describes a fork-resolution attempt against a peer chain.
type ReorgResult struct {
	Peer              string `json:"peer,omitempty"`
	LocalHeight       int    `json:"local_height"`
	CandidateHeight   int    `json:"candidate_height"`
	BeforeHeadHash    string `json:"before_head_hash"`
	CandidateHeadHash string `json:"candidate_head_hash"`
	AfterHeight       int    `json:"after_height"`
	AfterHeadHash     string `json:"after_head_hash"`
	Decision          string `json:"decision"`
	Adopted           bool   `json:"adopted"`
	Reason            string `json:"reason"`
	KeptPending       int    `json:"kept_pending"`
	DroppedPending    int    `json:"dropped_pending"`
}

// ResolveForkFromPeer downloads a peer's full chain, validates it, and applies the
// agreed fork-choice rule through the blockchain core.
func (n *Node) ResolveForkFromPeer(ctx context.Context, peer string) (ReorgResult, error) {
	peer = normalizePeer(peer)
	if peer == "" {
		return ReorgResult{}, fmt.Errorf("fork resolution peer is required")
	}
	status, err := fetchPeerStatus(ctx, peer)
	if err != nil {
		return ReorgResult{}, err
	}
	chain := make([]blockchain.Block, 0, status.Height+1)
	for height := 0; height <= status.Height; height++ {
		block, err := fetchPeerBlock(ctx, peer, height)
		if err != nil {
			return ReorgResult{}, err
		}
		chain = append(chain, block)
	}
	if len(chain) == 0 {
		return ReorgResult{}, fmt.Errorf("peer %s returned empty chain", peer)
	}
	candidateHead := chain[len(chain)-1]
	if candidateHead.Hash != status.HeadHash {
		return ReorgResult{}, fmt.Errorf("peer %s status head hash %s does not match downloaded head hash %s", peer, status.HeadHash, candidateHead.Hash)
	}
	candidate := blockchain.State{
		Chain:   chain,
		Pending: nil,
	}
	return n.ResolveCandidateChain(peer, candidate)
}

// ResolveCandidateChain compares the local chain with a candidate chain and adopts
// the candidate only when the blockchain fork-choice rule accepts it.
func (n *Node) ResolveCandidateChain(peer string, candidate blockchain.State) (ReorgResult, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	localBefore := cloneState(n.state)
	localHead := localBefore.Chain[len(localBefore.Chain)-1]
	candidateHeight := -1
	candidateHeadHash := ""
	if len(candidate.Chain) > 0 {
		candidateHeight = len(candidate.Chain) - 1
		candidateHeadHash = candidate.Chain[len(candidate.Chain)-1].Hash
	}
	resolved, forkResult, err := blockchain.ResolveFork(n.state, candidate, n.cfg)
	if err != nil {
		return ReorgResult{}, err
	}
	result := ReorgResult{
		Peer:              normalizePeer(peer),
		LocalHeight:       len(localBefore.Chain) - 1,
		CandidateHeight:   candidateHeight,
		BeforeHeadHash:    localHead.Hash,
		CandidateHeadHash: candidateHeadHash,
		Decision:          string(forkResult.Decision),
		Adopted:           forkResult.Adopted,
		Reason:            forkResult.Reason,
		KeptPending:       forkResult.KeptPending,
		DroppedPending:    forkResult.DroppedPending,
	}
	if forkResult.Adopted {
		adopted := cloneState(resolved)
		orphaned := orphanedConfirmedTransactions(localBefore.Chain, candidate.Chain)
		keptOrphans, droppedOrphans := addValidOrphansToPending(&adopted, orphaned)
		result.KeptPending += keptOrphans
		result.DroppedPending += droppedOrphans
		n.state = adopted
		n.resetSeenLocked()
		if err := blockchain.SaveState(n.dataPath, n.state); err != nil {
			return ReorgResult{}, err
		}
		n.logger.Printf("reorg adopted peer=%s local_height=%d candidate_height=%d new_head=%s kept_pending=%d dropped_pending=%d", result.Peer, result.LocalHeight, result.CandidateHeight, n.state.Chain[len(n.state.Chain)-1].Hash, result.KeptPending, result.DroppedPending)
	} else {
		n.logger.Printf("reorg skipped peer=%s local_height=%d candidate_height=%d reason=%s", result.Peer, result.LocalHeight, result.CandidateHeight, result.Reason)
	}
	afterHead := n.state.Chain[len(n.state.Chain)-1]
	result.AfterHeight = len(n.state.Chain) - 1
	result.AfterHeadHash = afterHead.Hash
	return result, nil
}

// orphanedConfirmedTransactions returns transactions that were confirmed in the old
// local chain but are not present in the candidate chain after reorganisation.
func orphanedConfirmedTransactions(localChain []blockchain.Block, candidateChain []blockchain.Block) []blockchain.Transaction {
	candidateTxIDs := make(map[string]struct{})
	for _, block := range candidateChain {
		for _, tx := range block.Transactions {
			if tx.ID == "" {
				continue
			}
			candidateTxIDs[tx.ID] = struct{}{}
		}
	}
	orphaned := make([]blockchain.Transaction, 0)
	for _, block := range localChain {
		for _, tx := range block.Transactions {
			if tx.ID == "" {
				continue
			}
			if _, exists := candidateTxIDs[tx.ID]; exists {
				continue
			}
			orphaned = append(orphaned, tx)
		}
	}
	return orphaned
}

// addValidOrphansToPending tries to return orphaned transactions to the pending pool.
// Invalid or now-conflicting transactions are safely dropped.
func addValidOrphansToPending(state *blockchain.State, orphaned []blockchain.Transaction) (int, int) {
	pendingTxIDs := make(map[string]struct{})
	for _, tx := range state.Pending {
		if tx.ID == "" {
			continue
		}
		pendingTxIDs[tx.ID] = struct{}{}
	}
	kept := 0
	dropped := 0
	for _, tx := range orphaned {
		if tx.ID == "" {
			continue
		}
		if _, exists := pendingTxIDs[tx.ID]; exists {
			continue
		}
		if err := state.AddPending(tx); err != nil {
			dropped++
			continue
		}
		pendingTxIDs[tx.ID] = struct{}{}
		kept++
	}
	return kept, dropped
}
