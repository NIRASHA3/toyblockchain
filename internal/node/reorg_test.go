package node

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"toyblockchain/internal/blockchain"
)

func TestResolveForkEndpointAdoptsLongerPeerChainAndReturnsOrphanTransaction(t *testing.T) {
	alice, err := blockchain.NewWallet()
	if err != nil {
		t.Fatalf("new alice wallet: %v", err)
	}
	bob, err := blockchain.NewWallet()
	if err != nil {
		t.Fatalf("new bob wallet: %v", err)
	}

	nodeA, nodeB := newSyncedFundedNodePair(t, alice.Address)

	orphanTx, err := blockchain.NewSignedTransfer(alice, bob.Address, 25, 1, "orphan should return", time.Now())
	if err != nil {
		t.Fatalf("new signed transfer: %v", err)
	}

	if _, err := nodeA.AddTransaction(orphanTx); err != nil {
		t.Fatalf("add orphan tx to node A: %v", err)
	}
	if _, _, err := nodeA.MinePending(context.Background(), time.Now()); err != nil {
		t.Fatalf("mine node A fork block: %v", err)
	}

	addAndMineFaucet(t, nodeB, "charlie", 10, "candidate fork block")
	addAndMineFaucet(t, nodeB, "dana", 10, "candidate longer block")

	serverA := httptest.NewServer(nodeA.Handler())
	defer serverA.Close()
	serverB := httptest.NewServer(nodeB.Handler())
	defer serverB.Close()

	nodeA.SetSelfURL(serverA.URL)
	nodeB.SetSelfURL(serverB.URL)
	nodeA.AddPeer(serverB.URL)
	nodeB.AddPeer(serverA.URL)

	var beforeA Status
	getJSON(t, serverA.URL+"/status", &beforeA)
	if beforeA.Height != 2 {
		t.Fatalf("expected node A before height 2, got %d", beforeA.Height)
	}

	var beforeB Status
	getJSON(t, serverB.URL+"/status", &beforeB)
	if beforeB.Height != 3 {
		t.Fatalf("expected node B candidate height 3, got %d", beforeB.Height)
	}

	var result ReorgResult
	postJSON(t, serverA.URL+"/resolve-fork", syncRequest{Peer: serverB.URL}, http.StatusOK, &result)

	if !result.Adopted {
		t.Fatalf("expected longer peer chain to be adopted, decision=%s reason=%s", result.Decision, result.Reason)
	}
	if result.LocalHeight != 2 {
		t.Fatalf("expected local height 2 before reorg, got %d", result.LocalHeight)
	}
	if result.CandidateHeight != 3 {
		t.Fatalf("expected candidate height 3, got %d", result.CandidateHeight)
	}
	if result.AfterHeight != 3 {
		t.Fatalf("expected after height 3, got %d", result.AfterHeight)
	}

	var afterA Status
	getJSON(t, serverA.URL+"/status", &afterA)

	if afterA.Height != beforeB.Height {
		t.Fatalf("expected node A height %d after reorg, got %d", beforeB.Height, afterA.Height)
	}
	if afterA.HeadHash != beforeB.HeadHash {
		t.Fatalf("expected node A head hash %s after reorg, got %s", beforeB.HeadHash, afterA.HeadHash)
	}

	var pending pendingResponse
	getJSON(t, serverA.URL+"/pending", &pending)

	if pending.Count != 1 {
		t.Fatalf("expected one valid orphan transaction returned to pending, got %d", pending.Count)
	}
	if pending.Pending[0].ID != orphanTx.ID {
		t.Fatalf("expected orphan transaction %s in pending, got %s", orphanTx.ID, pending.Pending[0].ID)
	}
}

func TestPeerBlockCanTriggerForkResolutionFromOrigin(t *testing.T) {
	alice, err := blockchain.NewWallet()
	if err != nil {
		t.Fatalf("new alice wallet: %v", err)
	}
	bob, err := blockchain.NewWallet()
	if err != nil {
		t.Fatalf("new bob wallet: %v", err)
	}

	nodeA, nodeB := newSyncedFundedNodePair(t, alice.Address)

	orphanTx, err := blockchain.NewSignedTransfer(alice, bob.Address, 25, 1, "automatic reorg orphan", time.Now())
	if err != nil {
		t.Fatalf("new signed transfer: %v", err)
	}

	if _, err := nodeA.AddTransaction(orphanTx); err != nil {
		t.Fatalf("add orphan tx to node A: %v", err)
	}
	if _, _, err := nodeA.MinePending(context.Background(), time.Now()); err != nil {
		t.Fatalf("mine node A fork block: %v", err)
	}

	addAndMineFaucet(t, nodeB, "charlie", 10, "candidate fork block")
	addAndMineFaucet(t, nodeB, "dana", 10, "candidate longer block")

	serverA := httptest.NewServer(nodeA.Handler())
	defer serverA.Close()
	serverB := httptest.NewServer(nodeB.Handler())
	defer serverB.Close()

	nodeA.SetSelfURL(serverA.URL)
	nodeB.SetSelfURL(serverB.URL)
	nodeA.AddPeer(serverB.URL)
	nodeB.AddPeer(serverA.URL)

	latestCandidateBlock := nodeB.Snapshot().Chain[3]

	request := blockGossipRequest{
		Origin: serverB.URL,
		Block:  latestCandidateBlock,
	}

	var response peerBlockResponse
	postJSON(t, serverA.URL+"/peer/blocks", request, http.StatusOK, &response)

	if response.Reorg == nil {
		t.Fatal("expected competing peer block to trigger fork resolution")
	}
	if !response.Reorg.Adopted {
		t.Fatalf("expected automatic reorg to adopt longer chain, decision=%s reason=%s", response.Reorg.Decision, response.Reorg.Reason)
	}

	var statusA Status
	getJSON(t, serverA.URL+"/status", &statusA)

	var statusB Status
	getJSON(t, serverB.URL+"/status", &statusB)

	if statusA.Height != statusB.Height {
		t.Fatalf("expected node A height %d, got %d", statusB.Height, statusA.Height)
	}
	if statusA.HeadHash != statusB.HeadHash {
		t.Fatalf("expected node A head hash %s, got %s", statusB.HeadHash, statusA.HeadHash)
	}
}
