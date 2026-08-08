package node

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"toyblockchain/internal/blockchain"
)

func TestSyncEndpointDownloadsMissingBlocksFromPeer(t *testing.T) {
	alice, err := blockchain.NewWallet()
	if err != nil {
		t.Fatalf("new alice wallet: %v", err)
	}

	peerNode, err := New(testNodeConfig(t))
	if err != nil {
		t.Fatalf("new peer node: %v", err)
	}

	fundWallet(t, peerNode, alice.Address)
	addAndMineFaucet(t, peerNode, "bob", 25, "second block")

	newNode, err := New(testNodeConfig(t))
	if err != nil {
		t.Fatalf("new syncing node: %v", err)
	}

	peerServer := httptest.NewServer(peerNode.Handler())
	defer peerServer.Close()

	newServer := httptest.NewServer(newNode.Handler())
	defer newServer.Close()

	peerNode.SetSelfURL(peerServer.URL)
	newNode.SetSelfURL(newServer.URL)
	newNode.AddPeer(peerServer.URL)

	var before Status
	getJSON(t, newServer.URL+"/status", &before)

	if before.Height != 0 {
		t.Fatalf("expected new node to start at height 0, got %d", before.Height)
	}

	var result syncResponse
	postJSON(t, newServer.URL+"/sync", syncRequest{Peer: peerServer.URL}, http.StatusOK, &result)

	if !result.Synced {
		t.Fatalf("expected sync result to be synced")
	}
	if result.BeforeHeight != 0 {
		t.Fatalf("expected before height 0, got %d", result.BeforeHeight)
	}
	if result.PeerHeight != 2 {
		t.Fatalf("expected peer height 2, got %d", result.PeerHeight)
	}
	if result.AfterHeight != 2 {
		t.Fatalf("expected after height 2, got %d", result.AfterHeight)
	}
	if result.BlocksDownloaded != 2 {
		t.Fatalf("expected 2 downloaded blocks, got %d", result.BlocksDownloaded)
	}

	var peerStatus Status
	getJSON(t, peerServer.URL+"/status", &peerStatus)

	var syncedStatus Status
	getJSON(t, newServer.URL+"/status", &syncedStatus)

	if syncedStatus.Height != peerStatus.Height {
		t.Fatalf("expected synced height %d, got %d", peerStatus.Height, syncedStatus.Height)
	}
	if syncedStatus.HeadHash != peerStatus.HeadHash {
		t.Fatalf("expected synced head hash %s, got %s", peerStatus.HeadHash, syncedStatus.HeadHash)
	}
}

func TestSyncEndpointUsesConfiguredPeerWhenRequestIsEmpty(t *testing.T) {
	peerNode, err := New(testNodeConfig(t))
	if err != nil {
		t.Fatalf("new peer node: %v", err)
	}

	addAndMineFaucet(t, peerNode, "alice", 100, "fund alice")

	newNode, err := New(testNodeConfig(t))
	if err != nil {
		t.Fatalf("new syncing node: %v", err)
	}

	peerServer := httptest.NewServer(peerNode.Handler())
	defer peerServer.Close()

	newServer := httptest.NewServer(newNode.Handler())
	defer newServer.Close()

	peerNode.SetSelfURL(peerServer.URL)
	newNode.SetSelfURL(newServer.URL)
	newNode.AddPeer(peerServer.URL)

	var result syncResponse
	postJSON(t, newServer.URL+"/sync", nil, http.StatusOK, &result)

	if !result.Synced {
		t.Fatal("expected empty sync request to use configured peer and sync")
	}
	if result.AfterHeight != 1 {
		t.Fatalf("expected after height 1, got %d", result.AfterHeight)
	}
	if result.BlocksDownloaded != 1 {
		t.Fatalf("expected 1 downloaded block, got %d", result.BlocksDownloaded)
	}
}

func TestSyncEndpointNoopsWhenAlreadyUpToDate(t *testing.T) {
	peerNode, syncingNode := newSyncedFundedNodePair(t, "alice")

	peerServer := httptest.NewServer(peerNode.Handler())
	defer peerServer.Close()

	syncingServer := httptest.NewServer(syncingNode.Handler())
	defer syncingServer.Close()

	peerNode.SetSelfURL(peerServer.URL)
	syncingNode.SetSelfURL(syncingServer.URL)
	syncingNode.AddPeer(peerServer.URL)

	var result syncResponse
	postJSON(t, syncingServer.URL+"/sync", syncRequest{Peer: peerServer.URL}, http.StatusOK, &result)

	if !result.Synced {
		t.Fatal("expected already up-to-date node to report synced")
	}
	if result.BlocksDownloaded != 0 {
		t.Fatalf("expected 0 downloaded blocks, got %d", result.BlocksDownloaded)
	}
	if result.BeforeHeight != 1 || result.AfterHeight != 1 {
		t.Fatalf("expected height to remain 1, before=%d after=%d", result.BeforeHeight, result.AfterHeight)
	}
}

func TestPeerStatusAndPeerBlockEndpoints(t *testing.T) {
	n, err := New(testNodeConfig(t))
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	addAndMineFaucet(t, n, "alice", 100, "peer endpoint test")

	server := httptest.NewServer(n.Handler())
	defer server.Close()

	var status Status
	getJSON(t, server.URL+"/peer/status", &status)

	if status.Height != 1 {
		t.Fatalf("expected peer status height 1, got %d", status.Height)
	}

	var block blockResponse
	getJSON(t, server.URL+"/peer/blocks/1", &block)

	if block.Block.Height != 1 {
		t.Fatalf("expected peer block height 1, got %d", block.Block.Height)
	}
	if block.Block.Hash != status.HeadHash {
		t.Fatalf("expected block hash %s, got %s", status.HeadHash, block.Block.Hash)
	}
}

func addAndMineFaucet(t *testing.T, n *Node, to string, amount int64, memo string) {
	t.Helper()

	tx, err := blockchain.NewFaucet(to, amount, memo, time.Now())
	if err != nil {
		t.Fatalf("new faucet transaction: %v", err)
	}

	if _, err := n.AddTransaction(tx); err != nil {
		t.Fatalf("add faucet transaction: %v", err)
	}

	if _, _, err := n.MinePending(context.Background(), time.Now()); err != nil {
		t.Fatalf("mine faucet transaction: %v", err)
	}
}
