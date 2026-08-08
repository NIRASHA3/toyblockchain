package node

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"toyblockchain/internal/blockchain"
)

var ErrBlockDoesNotExtendHead = errors.New("block does not extend local head")

// Config contains the local node settings used by the network layer.
type Config struct {
	DataPath    string
	ChainConfig blockchain.Config
	SelfURL     string
	Peers       []string
	Logger      *log.Logger
}

// Status is a small introspection view of the node.
type Status struct {
	Height       int    `json:"height"`
	HeadHash     string `json:"head_hash"`
	PendingCount int    `json:"pending_count"`
	PeerCount    int    `json:"peer_count"`
}

// Node owns one local blockchain state and protects it for concurrent use.
type Node struct {
	mu sync.RWMutex

	dataPath string
	cfg      blockchain.Config
	selfURL  string
	state    blockchain.State

	peers      map[string]struct{}
	seenTx     map[string]struct{}
	seenBlocks map[string]struct{}

	logger *log.Logger
}

// New loads a node from disk and prepares concurrency-safe network state.
func New(cfg Config) (*Node, error) {
	if strings.TrimSpace(cfg.DataPath) == "" {
		return nil, fmt.Errorf("node data path is required")
	}
	if err := cfg.ChainConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid blockchain config: %w", err)
	}

	state, err := blockchain.LoadState(cfg.DataPath)
	if err != nil {
		return nil, err
	}
	if err := blockchain.ValidateChainWithConfig(state.Chain, cfg.ChainConfig); err != nil {
		return nil, fmt.Errorf("invalid local chain: %w", err)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	n := &Node{
		dataPath:   cfg.DataPath,
		cfg:        cfg.ChainConfig,
		selfURL:    normalizePeer(cfg.SelfURL),
		state:      cloneState(state),
		peers:      make(map[string]struct{}),
		seenTx:     make(map[string]struct{}),
		seenBlocks: make(map[string]struct{}),
		logger:     logger,
	}

	for _, peer := range cfg.Peers {
		if normalized := normalizePeer(peer); normalized != "" {
			n.peers[normalized] = struct{}{}
		}
	}

	n.indexSeenLocked()

	return n, nil
}

// Status returns the current chain height, head hash, pending count, and peer count.
func (n *Node) Status() Status {
	n.mu.RLock()
	defer n.mu.RUnlock()

	head := n.state.Chain[len(n.state.Chain)-1]

	return Status{
		Height:       len(n.state.Chain) - 1,
		HeadHash:     head.Hash,
		PendingCount: len(n.state.Pending),
		PeerCount:    len(n.peers),
	}
}

// Snapshot returns a defensive copy of the current local state.
func (n *Node) Snapshot() blockchain.State {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return cloneState(n.state)
}

// Peers returns the node's known peers in stable order.
func (n *Node) Peers() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return sortedKeys(n.peers)
}

// SelfURL returns the node's own advertised base URL.
func (n *Node) SelfURL() string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.selfURL
}

// SetSelfURL updates the node's own advertised base URL.
// This is useful in tests where httptest assigns the URL after the node is created.
func (n *Node) SetSelfURL(url string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.selfURL = normalizePeer(url)
}

// AddPeer adds a peer address to the local peer set.
func (n *Node) AddPeer(peer string) bool {
	normalized := normalizePeer(peer)
	if normalized == "" {
		return false
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	if _, exists := n.peers[normalized]; exists {
		return false
	}

	n.peers[normalized] = struct{}{}
	return true
}

// AddTransaction validates and stores a transaction if it has not already been seen.
// The returned bool is false when the transaction is a duplicate.
func (n *Node) AddTransaction(tx blockchain.Transaction) (bool, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if _, exists := n.seenTx[tx.ID]; exists {
		return false, nil
	}

	if err := n.state.AddPending(tx); err != nil {
		return false, err
	}

	n.seenTx[tx.ID] = struct{}{}

	if err := blockchain.SaveState(n.dataPath, n.state); err != nil {
		return false, err
	}

	n.logger.Printf("transaction accepted id=%s pending=%d", tx.ID, len(n.state.Pending))

	return true, nil
}

// MinePending mines pending transactions into a block and updates local state safely.
func (n *Node) MinePending(ctx context.Context, now time.Time) (blockchain.Block, blockchain.MiningStats, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	block, stats, err := n.state.MinePending(ctx, n.cfg, now)
	if err != nil {
		return blockchain.Block{}, stats, err
	}

	n.seenBlocks[block.Hash] = struct{}{}
	for _, tx := range block.Transactions {
		n.seenTx[tx.ID] = struct{}{}
	}

	if err := blockchain.SaveState(n.dataPath, n.state); err != nil {
		return blockchain.Block{}, stats, err
	}

	n.logger.Printf("block mined height=%d hash=%s txs=%d", block.Height, block.Hash, len(block.Transactions))

	return block, stats, nil
}

// AcceptBlock validates a peer block and appends it only if it extends the current head.
// The returned bool is false when the block was already seen.
func (n *Node) AcceptBlock(block blockchain.Block) (bool, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if _, exists := n.seenBlocks[block.Hash]; exists {
		return false, nil
	}

	head := n.state.Chain[len(n.state.Chain)-1]
	if block.PrevHash != head.Hash {
		return false, fmt.Errorf("%w: got prev_hash %s, expected %s", ErrBlockDoesNotExtendHead, block.PrevHash, head.Hash)
	}
	if block.Height != head.Height+1 {
		return false, fmt.Errorf("%w: block height %d does not extend local height %d", ErrBlockDoesNotExtendHead, block.Height, head.Height)
	}

	candidate := cloneState(n.state)
	candidate.Chain = append(candidate.Chain, cloneBlock(block))
	candidate.Pending = removeConfirmedTransactions(candidate.Pending, block.Transactions)

	if err := blockchain.ValidateChainWithConfig(candidate.Chain, n.cfg); err != nil {
		return false, err
	}

	n.state = candidate
	n.seenBlocks[block.Hash] = struct{}{}
	for _, tx := range block.Transactions {
		n.seenTx[tx.ID] = struct{}{}
	}

	if err := blockchain.SaveState(n.dataPath, n.state); err != nil {
		return false, err
	}

	n.logger.Printf("block accepted height=%d hash=%s txs=%d", block.Height, block.Hash, len(block.Transactions))

	return true, nil
}

func (n *Node) indexSeenLocked() {
	for _, block := range n.state.Chain {
		n.seenBlocks[block.Hash] = struct{}{}
		for _, tx := range block.Transactions {
			n.seenTx[tx.ID] = struct{}{}
		}
	}

	for _, tx := range n.state.Pending {
		n.seenTx[tx.ID] = struct{}{}
	}
}

func (n *Node) resetSeenLocked() {
	n.seenTx = make(map[string]struct{})
	n.seenBlocks = make(map[string]struct{})
	n.indexSeenLocked()
}

func normalizePeer(peer string) string {
	peer = strings.TrimSpace(peer)
	peer = strings.TrimRight(peer, "/")
	return peer
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func removeConfirmedTransactions(pending []blockchain.Transaction, confirmed []blockchain.Transaction) []blockchain.Transaction {
	confirmedIDs := make(map[string]struct{}, len(confirmed))
	for _, tx := range confirmed {
		confirmedIDs[tx.ID] = struct{}{}
	}

	kept := make([]blockchain.Transaction, 0, len(pending))
	for _, tx := range pending {
		if _, included := confirmedIDs[tx.ID]; included {
			continue
		}
		kept = append(kept, tx)
	}

	return kept
}

func cloneState(state blockchain.State) blockchain.State {
	return blockchain.State{
		Chain:   cloneChain(state.Chain),
		Pending: cloneTransactions(state.Pending),
	}
}

func cloneChain(chain []blockchain.Block) []blockchain.Block {
	copied := make([]blockchain.Block, len(chain))
	for i, block := range chain {
		copied[i] = cloneBlock(block)
	}
	return copied
}

func cloneBlock(block blockchain.Block) blockchain.Block {
	block.Transactions = cloneTransactions(block.Transactions)
	return block
}

func cloneTransactions(txs []blockchain.Transaction) []blockchain.Transaction {
	copied := make([]blockchain.Transaction, len(txs))
	copy(copied, txs)
	return copied
}
