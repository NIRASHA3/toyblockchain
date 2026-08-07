package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"toyblockchain/internal/blockchain"
)

const peerRequestTimeout = 3 * time.Second

// transactionGossipRequest is the wire format used between peers.
type transactionGossipRequest struct {
	Origin      string                 `json:"origin,omitempty"`
	Transaction blockchain.Transaction `json:"transaction"`
}

// TransactionGossipResult describes what happened after a transaction was received.
type TransactionGossipResult struct {
	ID       string   `json:"id"`
	Accepted bool     `json:"accepted"`
	Gossiped int      `json:"gossiped"`
	Errors   []string `json:"errors,omitempty"`
}

// SubmitTransaction accepts a client-submitted transaction and gossips it to peers.
func (n *Node) SubmitTransaction(ctx context.Context, tx blockchain.Transaction) (TransactionGossipResult, error) {
	return n.receiveTransaction(ctx, tx, "")
}

// ReceivePeerTransaction accepts a transaction received from a peer and gossips it onward.
func (n *Node) ReceivePeerTransaction(ctx context.Context, tx blockchain.Transaction, origin string) (TransactionGossipResult, error) {
	return n.receiveTransaction(ctx, tx, origin)
}

func (n *Node) receiveTransaction(ctx context.Context, tx blockchain.Transaction, origin string) (TransactionGossipResult, error) {
	result := TransactionGossipResult{ID: tx.ID}

	accepted, err := n.AddTransaction(tx)
	if err != nil {
		return result, err
	}

	result.Accepted = accepted
	if !accepted {
		n.logger.Printf("duplicate transaction ignored id=%s", tx.ID)
		return result, nil
	}

	origin = normalizePeer(origin)
	selfURL := n.SelfURL()
	peers := n.Peers()

	for _, peer := range peers {
		peer = normalizePeer(peer)
		if peer == "" || peer == origin || peer == selfURL {
			continue
		}

		if err := postPeerTransaction(ctx, peer, tx, selfURL); err != nil {
			result.Errors = append(result.Errors, err.Error())
			n.logger.Printf("transaction gossip failed id=%s peer=%s error=%v", tx.ID, peer, err)
			continue
		}

		result.Gossiped++
		n.logger.Printf("transaction gossiped id=%s peer=%s", tx.ID, peer)
	}

	return result, nil
}

func postPeerTransaction(ctx context.Context, peer string, tx blockchain.Transaction, origin string) error {
	payload := transactionGossipRequest{
		Origin:      origin,
		Transaction: tx,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode peer transaction request: %w", err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, peerRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, peer+"/peer/transactions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create peer transaction request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("post transaction to %s: %w", peer, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("peer %s returned %s", peer, resp.Status)
	}

	return nil
}
