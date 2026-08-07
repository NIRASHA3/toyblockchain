package node

import (
	"context"
	"testing"
	"time"

	"toyblockchain/internal/blockchain"
)

func testNodeConfig(t *testing.T, peers ...string) Config {
	t.Helper()

	return Config{
		DataPath: t.TempDir() + "/node.json",
		ChainConfig: blockchain.Config{
			Difficulty:       1,
			MaxBlockTx:       5,
			Workers:          1,
			RetargetInterval: 0,
			TargetBlockTime:  10 * time.Second,
		},
		Peers: peers,
	}
}

func TestNewNodeStartsFromGenesis(t *testing.T) {
	n, err := New(testNodeConfig(t, "http://127.0.0.1:8082/", "http://127.0.0.1:8083"))
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	status := n.Status()

	if status.Height != 0 {
		t.Fatalf("expected height 0, got %d", status.Height)
	}
	if status.PendingCount != 0 {
		t.Fatalf("expected empty pending pool, got %d", status.PendingCount)
	}
	if status.PeerCount != 2 {
		t.Fatalf("expected 2 peers, got %d", status.PeerCount)
	}
}

func TestAddTransactionDeduplicates(t *testing.T) {
	n, err := New(testNodeConfig(t))
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	tx, err := blockchain.NewFaucet("alice", 100, "test faucet", time.Now())
	if err != nil {
		t.Fatalf("new faucet tx: %v", err)
	}

	accepted, err := n.AddTransaction(tx)
	if err != nil {
		t.Fatalf("add transaction: %v", err)
	}
	if !accepted {
		t.Fatal("expected first transaction to be accepted")
	}

	accepted, err = n.AddTransaction(tx)
	if err != nil {
		t.Fatalf("add duplicate transaction: %v", err)
	}
	if accepted {
		t.Fatal("expected duplicate transaction to be ignored")
	}

	status := n.Status()
	if status.PendingCount != 1 {
		t.Fatalf("expected pending count 1, got %d", status.PendingCount)
	}
}

func TestMinePendingRemovesMinedTransaction(t *testing.T) {
	n, err := New(testNodeConfig(t))
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	tx, err := blockchain.NewFaucet("alice", 100, "test faucet", time.Now())
	if err != nil {
		t.Fatalf("new faucet tx: %v", err)
	}

	accepted, err := n.AddTransaction(tx)
	if err != nil {
		t.Fatalf("add transaction: %v", err)
	}
	if !accepted {
		t.Fatal("expected transaction to be accepted")
	}

	block, _, err := n.MinePending(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("mine pending: %v", err)
	}

	if block.Height != 1 {
		t.Fatalf("expected mined block height 1, got %d", block.Height)
	}
	if len(block.Transactions) != 1 {
		t.Fatalf("expected 1 transaction in block, got %d", len(block.Transactions))
	}

	status := n.Status()
	if status.Height != 1 {
		t.Fatalf("expected node height 1, got %d", status.Height)
	}
	if status.PendingCount != 0 {
		t.Fatalf("expected pending count 0, got %d", status.PendingCount)
	}
}

func TestAcceptBlockThatExtendsHead(t *testing.T) {
	miner, err := New(testNodeConfig(t))
	if err != nil {
		t.Fatalf("new miner node: %v", err)
	}

	peer, err := New(testNodeConfig(t))
	if err != nil {
		t.Fatalf("new peer node: %v", err)
	}

	tx, err := blockchain.NewFaucet("alice", 100, "test faucet", time.Now())
	if err != nil {
		t.Fatalf("new faucet tx: %v", err)
	}

	if _, err := miner.AddTransaction(tx); err != nil {
		t.Fatalf("add transaction to miner: %v", err)
	}

	block, _, err := miner.MinePending(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("mine block: %v", err)
	}

	accepted, err := peer.AcceptBlock(block)
	if err != nil {
		t.Fatalf("accept block: %v", err)
	}
	if !accepted {
		t.Fatal("expected peer to accept new block")
	}

	status := peer.Status()
	if status.Height != 1 {
		t.Fatalf("expected peer height 1, got %d", status.Height)
	}
}
