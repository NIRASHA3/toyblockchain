package node

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"toyblockchain/internal/blockchain"
)

// SyncResult describes a catch-up attempt from a peer.
type SyncResult struct {
	Peer             string   `json:"peer"`
	BeforeHeight     int      `json:"before_height"`
	PeerHeight       int      `json:"peer_height"`
	AfterHeight      int      `json:"after_height"`
	HeadHash         string   `json:"head_hash"`
	BlocksDownloaded int      `json:"blocks_downloaded"`
	Synced           bool     `json:"synced"`
	Errors           []string `json:"errors,omitempty"`
}

// SyncFromPeer downloads missing blocks from a peer and validates each block before appending.
// This path is intended for a new or lagging node whose local chain is a prefix of the peer chain.
func (n *Node) SyncFromPeer(ctx context.Context, peer string) (SyncResult, error) {
	peer = normalizePeer(peer)
	if peer == "" {
		return SyncResult{}, fmt.Errorf("sync peer is required")
	}

	localBefore := n.Status()

	result := SyncResult{
		Peer:         peer,
		BeforeHeight: localBefore.Height,
		AfterHeight:  localBefore.Height,
		HeadHash:     localBefore.HeadHash,
	}

	peerStatus, err := fetchPeerStatus(ctx, peer)
	if err != nil {
		return result, err
	}

	result.PeerHeight = peerStatus.Height

	if peerStatus.Height <= localBefore.Height {
		result.Synced = localBefore.Height == peerStatus.Height && localBefore.HeadHash == peerStatus.HeadHash
		return result, nil
	}

	for height := localBefore.Height + 1; height <= peerStatus.Height; height++ {
		block, err := fetchPeerBlock(ctx, peer, height)
		if err != nil {
			return result, err
		}

		accepted, err := n.AcceptBlock(block)
		if err != nil {
			return result, fmt.Errorf("accept synced block height %d from %s: %w", height, peer, err)
		}
		if accepted {
			result.BlocksDownloaded++
		}
	}

	localAfter := n.Status()

	result.AfterHeight = localAfter.Height
	result.HeadHash = localAfter.HeadHash
	result.Synced = localAfter.Height == peerStatus.Height && localAfter.HeadHash == peerStatus.HeadHash

	return result, nil
}

func fetchPeerStatus(ctx context.Context, peer string) (Status, error) {
	requestCtx, cancel := context.WithTimeout(ctx, peerRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, peer+"/peer/status", nil)
	if err != nil {
		return Status{}, fmt.Errorf("create peer status request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Status{}, fmt.Errorf("get peer status from %s: %w", peer, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Status{}, fmt.Errorf("peer %s returned %s for status", peer, resp.Status)
	}

	var status Status
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return Status{}, fmt.Errorf("decode peer status from %s: %w", peer, err)
	}

	return status, nil
}

func fetchPeerBlock(ctx context.Context, peer string, height int) (blockchain.Block, error) {
	requestCtx, cancel := context.WithTimeout(ctx, peerRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, fmt.Sprintf("%s/peer/blocks/%d", peer, height), nil)
	if err != nil {
		return blockchain.Block{}, fmt.Errorf("create peer block request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return blockchain.Block{}, fmt.Errorf("get block height %d from %s: %w", height, peer, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return blockchain.Block{}, fmt.Errorf("peer %s returned %s for block height %d", peer, resp.Status, height)
	}

	var response blockResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return blockchain.Block{}, fmt.Errorf("decode block height %d from %s: %w", height, peer, err)
	}

	return response.Block, nil
}
