package node

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"toyblockchain/internal/blockchain"
)

func TestHandlerStatusAndPeers(t *testing.T) {
	n, err := New(testNodeConfig(t, "http://127.0.0.1:8082/", "http://127.0.0.1:8083"))
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	server := httptest.NewServer(n.Handler())
	defer server.Close()

	var status Status
	getJSON(t, server.URL+"/status", &status)
	if status.Height != 0 {
		t.Fatalf("expected height 0, got %d", status.Height)
	}
	if status.PendingCount != 0 {
		t.Fatalf("expected pending count 0, got %d", status.PendingCount)
	}
	if status.PeerCount != 2 {
		t.Fatalf("expected peer count 2, got %d", status.PeerCount)
	}

	var peers peersResponse
	getJSON(t, server.URL+"/peers", &peers)
	if peers.Count != 2 {
		t.Fatalf("expected 2 peers, got %d", peers.Count)
	}
	if peers.Peers[0] != "http://127.0.0.1:8082" {
		t.Fatalf("expected normalized first peer, got %q", peers.Peers[0])
	}
}

func TestDiscoverPeersEndpointLearnsPeersFromSeed(t *testing.T) {
	peerA := newPeerListTestServer(t, nil)
	defer peerA.Close()

	peerB := newPeerListTestServer(t, nil)
	defer peerB.Close()

	seedNode, err := New(testNodeConfig(t, peerA.URL, peerB.URL))
	if err != nil {
		t.Fatalf("new seed node: %v", err)
	}

	seedServer := httptest.NewServer(seedNode.Handler())
	defer seedServer.Close()
	seedNode.SetSelfURL(seedServer.URL)

	joiningNode, err := New(testNodeConfig(t, seedServer.URL))
	if err != nil {
		t.Fatalf("new joining node: %v", err)
	}

	joiningServer := httptest.NewServer(joiningNode.Handler())
	defer joiningServer.Close()
	joiningNode.SetSelfURL(joiningServer.URL)

	var result DiscoveryResult
	postJSON(t, joiningServer.URL+"/discover-peers", nil, http.StatusOK, &result)

	if result.BeforeCount != 1 {
		t.Fatalf("expected one seed peer before discovery, got %d", result.BeforeCount)
	}
	if result.PeersAdded != 2 {
		t.Fatalf("expected two discovered peers, got %d", result.PeersAdded)
	}
	if result.AfterCount != 3 {
		t.Fatalf("expected three peers after discovery, got %d", result.AfterCount)
	}

	peers := joiningNode.Peers()

	if !containsString(peers, seedServer.URL) {
		t.Fatalf("expected joining node to keep seed peer %s, peers=%v", seedServer.URL, peers)
	}
	if !containsString(peers, peerA.URL) {
		t.Fatalf("expected joining node to discover peer A %s, peers=%v", peerA.URL, peers)
	}
	if !containsString(peers, peerB.URL) {
		t.Fatalf("expected joining node to discover peer B %s, peers=%v", peerB.URL, peers)
	}
}

func TestHandlerChainBlocksPendingAndValidate(t *testing.T) {
	n, err := New(testNodeConfig(t))
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	server := httptest.NewServer(n.Handler())
	defer server.Close()

	var chain chainResponse
	getJSON(t, server.URL+"/chain", &chain)
	if chain.Height != 0 {
		t.Fatalf("expected chain height 0, got %d", chain.Height)
	}
	if chain.BlockCount != 1 {
		t.Fatalf("expected one genesis block, got %d", chain.BlockCount)
	}
	if chain.PendingCount != 0 {
		t.Fatalf("expected no pending transactions, got %d", chain.PendingCount)
	}

	var blocks blocksResponse
	getJSON(t, server.URL+"/blocks", &blocks)

	if blocks.Count != 1 {
		t.Fatalf("expected one block, got %d", blocks.Count)
	}

	var block blockResponse
	getJSON(t, server.URL+"/blocks/0", &block)

	if block.Block.Height != 0 {
		t.Fatalf("expected genesis height 0, got %d", block.Block.Height)
	}

	var pending pendingResponse
	getJSON(t, server.URL+"/pending", &pending)

	if pending.Count != 0 {
		t.Fatalf("expected empty pending pool, got %d", pending.Count)
	}

	var validation validateResponse
	getJSON(t, server.URL+"/validate", &validation)
	if !validation.Valid {
		t.Fatalf("expected valid chain, got error %q", validation.Error)
	}
	if validation.BlocksChecked != 1 {
		t.Fatalf("expected one checked block, got %d", validation.BlocksChecked)
	}
}

func TestHandlerBalancesIncludingPending(t *testing.T) {
	n, err := New(testNodeConfig(t))
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	tx, err := blockchain.NewFaucet("alice", 100, "test faucet", time.Now())
	if err != nil {
		t.Fatalf("new faucet transaction: %v", err)
	}
	if _, err := n.AddTransaction(tx); err != nil {
		t.Fatalf("add transaction: %v", err)
	}

	server := httptest.NewServer(n.Handler())
	defer server.Close()

	var confirmed balancesResponse
	getJSON(t, server.URL+"/balances", &confirmed)

	if confirmed.IncludePending {
		t.Fatal("expected confirmed-only balances")
	}
	if confirmed.Balances["alice"] != 0 {
		t.Fatalf("expected confirmed alice balance 0, got %d", confirmed.Balances["alice"])
	}

	var pending balancesResponse
	getJSON(t, server.URL+"/balances?pending=true", &pending)

	if !pending.IncludePending {
		t.Fatal("expected pending balances")
	}
	if pending.Balances["alice"] != 100 {
		t.Fatalf("expected pending alice balance 100, got %d", pending.Balances["alice"])
	}
}

func TestHandlerRejectsWrongMethod(t *testing.T) {
	n, err := New(testNodeConfig(t))
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	server := httptest.NewServer(n.Handler())
	defer server.Close()

	resp, err := http.Post(server.URL+"/status", "application/json", nil)
	if err != nil {
		t.Fatalf("post status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestTransactionEndpointGossipsToPeerAndDeduplicates(t *testing.T) {
	alice, err := blockchain.NewWallet()
	if err != nil {
		t.Fatalf("new alice wallet: %v", err)
	}

	bob, err := blockchain.NewWallet()
	if err != nil {
		t.Fatalf("new bob wallet: %v", err)
	}

	nodeA, nodeB := newSyncedFundedNodePair(t, alice.Address)

	serverA := httptest.NewServer(nodeA.Handler())
	defer serverA.Close()

	serverB := httptest.NewServer(nodeB.Handler())
	defer serverB.Close()

	nodeA.SetSelfURL(serverA.URL)
	nodeB.SetSelfURL(serverB.URL)
	nodeA.AddPeer(serverB.URL)
	nodeB.AddPeer(serverA.URL)

	tx, err := blockchain.NewSignedTransfer(alice, bob.Address, 25, 1, "gossip test", time.Now())
	if err != nil {
		t.Fatalf("new signed transfer: %v", err)
	}

	var first transactionResponse
	postJSON(t, serverA.URL+"/transactions", tx, http.StatusCreated, &first)

	if !first.Accepted {
		t.Fatal("expected transaction to be accepted by node A")
	}
	if first.Gossiped != 1 {
		t.Fatalf("expected transaction to be gossiped to one peer, got %d", first.Gossiped)
	}

	var nodeBPending pendingResponse
	getJSON(t, serverB.URL+"/pending", &nodeBPending)

	if nodeBPending.Count != 1 {
		t.Fatalf("expected node B pending count 1, got %d", nodeBPending.Count)
	}
	if nodeBPending.Pending[0].ID != tx.ID {
		t.Fatalf("expected node B to receive tx %s, got %s", tx.ID, nodeBPending.Pending[0].ID)
	}

	var duplicate transactionResponse
	postJSON(t, serverA.URL+"/transactions", tx, http.StatusOK, &duplicate)

	if duplicate.Accepted {
		t.Fatal("expected duplicate transaction to be ignored")
	}
	if duplicate.Gossiped != 0 {
		t.Fatalf("expected duplicate transaction not to be gossiped, got %d", duplicate.Gossiped)
	}
}

func TestTransactionEndpointRejectsInvalidSignature(t *testing.T) {
	alice, err := blockchain.NewWallet()
	if err != nil {
		t.Fatalf("new alice wallet: %v", err)
	}

	bob, err := blockchain.NewWallet()
	if err != nil {
		t.Fatalf("new bob wallet: %v", err)
	}

	n, err := New(testNodeConfig(t))
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	fundWallet(t, n, alice.Address)

	server := httptest.NewServer(n.Handler())
	defer server.Close()

	tx, err := blockchain.NewSignedTransfer(alice, bob.Address, 25, 1, "invalid signature test", time.Now())
	if err != nil {
		t.Fatalf("new signed transfer: %v", err)
	}

	tx.Signature = strings.Repeat("00", 64)
	tx.ID = tx.ComputeID()

	var response errorResponse
	postJSON(t, server.URL+"/transactions", tx, http.StatusBadRequest, &response)

	var pending pendingResponse
	getJSON(t, server.URL+"/pending", &pending)

	if pending.Count != 0 {
		t.Fatalf("expected invalid transaction not to enter pending pool, got %d", pending.Count)
	}
}

func TestPeerTransactionEndpointAcceptsAndDoesNotForwardToOrigin(t *testing.T) {
	alice, err := blockchain.NewWallet()
	if err != nil {
		t.Fatalf("new alice wallet: %v", err)
	}

	bob, err := blockchain.NewWallet()
	if err != nil {
		t.Fatalf("new bob wallet: %v", err)
	}

	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("origin peer should not receive transaction back")
	}))
	defer peer.Close()

	n, err := New(testNodeConfig(t, peer.URL))
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	fundWallet(t, n, alice.Address)

	server := httptest.NewServer(n.Handler())
	defer server.Close()
	n.SetSelfURL(server.URL)

	tx, err := blockchain.NewSignedTransfer(alice, bob.Address, 25, 1, "origin skip test", time.Now())
	if err != nil {
		t.Fatalf("new signed transfer: %v", err)
	}

	request := transactionGossipRequest{
		Origin:      peer.URL,
		Transaction: tx,
	}

	var response transactionResponse
	postJSON(t, server.URL+"/peer/transactions", request, http.StatusCreated, &response)

	if !response.Accepted {
		t.Fatal("expected peer transaction to be accepted")
	}
	if response.Gossiped != 0 {
		t.Fatalf("expected transaction not to be sent back to origin, got gossiped=%d", response.Gossiped)
	}
}

func TestMineEndpointGossipsBlockToPeerAndClearsPending(t *testing.T) {
	alice, err := blockchain.NewWallet()
	if err != nil {
		t.Fatalf("new alice wallet: %v", err)
	}

	bob, err := blockchain.NewWallet()
	if err != nil {
		t.Fatalf("new bob wallet: %v", err)
	}

	nodeA, nodeB := newSyncedFundedNodePair(t, alice.Address)

	serverA := httptest.NewServer(nodeA.Handler())
	defer serverA.Close()

	serverB := httptest.NewServer(nodeB.Handler())
	defer serverB.Close()

	nodeA.SetSelfURL(serverA.URL)
	nodeB.SetSelfURL(serverB.URL)
	nodeA.AddPeer(serverB.URL)
	nodeB.AddPeer(serverA.URL)

	tx, err := blockchain.NewSignedTransfer(alice, bob.Address, 25, 1, "block gossip test", time.Now())
	if err != nil {
		t.Fatalf("new signed transfer: %v", err)
	}

	if _, err := nodeA.SubmitTransaction(context.Background(), tx); err != nil {
		t.Fatalf("submit tx to node A: %v", err)
	}

	var before pendingResponse
	getJSON(t, serverB.URL+"/pending", &before)

	if before.Count != 1 {
		t.Fatalf("expected node B pending count 1 before mining, got %d", before.Count)
	}

	var mined mineResponse
	postJSON(t, serverA.URL+"/mine", nil, http.StatusCreated, &mined)

	if mined.Block.Height != 2 {
		t.Fatalf("expected mined block height 2, got %d", mined.Block.Height)
	}
	if mined.Gossiped != 1 {
		t.Fatalf("expected mined block to be gossiped to one peer, got %d", mined.Gossiped)
	}

	var statusA Status
	getJSON(t, serverA.URL+"/status", &statusA)

	if statusA.Height != 2 {
		t.Fatalf("expected node A height 2, got %d", statusA.Height)
	}
	if statusA.PendingCount != 0 {
		t.Fatalf("expected node A pending count 0, got %d", statusA.PendingCount)
	}

	var statusB Status
	getJSON(t, serverB.URL+"/status", &statusB)

	if statusB.Height != 2 {
		t.Fatalf("expected node B height 2 after block gossip, got %d", statusB.Height)
	}
	if statusB.PendingCount != 0 {
		t.Fatalf("expected node B pending count 0 after accepted block, got %d", statusB.PendingCount)
	}
	if statusB.HeadHash != mined.Block.Hash {
		t.Fatalf("expected node B head hash %s, got %s", mined.Block.Hash, statusB.HeadHash)
	}
}

func TestPeerBlockEndpointDeduplicatesBlock(t *testing.T) {
	miner, err := New(testNodeConfig(t))
	if err != nil {
		t.Fatalf("new miner node: %v", err)
	}

	peer, err := New(testNodeConfig(t))
	if err != nil {
		t.Fatalf("new peer node: %v", err)
	}

	tx, err := blockchain.NewFaucet("alice", 100, "block duplicate test", time.Now())
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

	server := httptest.NewServer(peer.Handler())
	defer server.Close()
	peer.SetSelfURL(server.URL)

	request := blockGossipRequest{
		Origin: "http://127.0.0.1:9999",
		Block:  block,
	}

	var first peerBlockResponse
	postJSON(t, server.URL+"/peer/blocks", request, http.StatusCreated, &first)

	if !first.Accepted {
		t.Fatal("expected first block to be accepted")
	}

	var duplicate peerBlockResponse
	postJSON(t, server.URL+"/peer/blocks", request, http.StatusOK, &duplicate)

	if duplicate.Accepted {
		t.Fatal("expected duplicate block to be ignored")
	}
}

func newSyncedFundedNodePair(t *testing.T, fundedAddress string) (*Node, *Node) {
	t.Helper()

	cfgA := testNodeConfig(t)
	nodeA, err := New(cfgA)
	if err != nil {
		t.Fatalf("new node A: %v", err)
	}

	fundWallet(t, nodeA, fundedAddress)

	cfgB := testNodeConfig(t)
	if err := blockchain.SaveState(cfgB.DataPath, nodeA.Snapshot()); err != nil {
		t.Fatalf("save node A state for node B: %v", err)
	}

	nodeB, err := New(cfgB)
	if err != nil {
		t.Fatalf("new node B from synced state: %v", err)
	}

	return nodeA, nodeB
}

func fundWallet(t *testing.T, n *Node, address string) {
	t.Helper()

	tx, err := blockchain.NewFaucet(address, 100, "test funding", time.Now())
	if err != nil {
		t.Fatalf("new faucet transaction: %v", err)
	}

	if _, err := n.AddTransaction(tx); err != nil {
		t.Fatalf("add faucet transaction: %v", err)
	}

	if _, _, err := n.MinePending(context.Background(), time.Now()); err != nil {
		t.Fatalf("mine funding block: %v", err)
	}
}

func newPeerListTestServer(t *testing.T, peers []string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/peers" {
			http.NotFound(w, r)
			return
		}

		writeJSON(w, http.StatusOK, peersResponse{
			Count: len(peers),
			Peers: peers,
		})
	}))
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}

func getJSON(t *testing.T, url string, target any) {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s returned status %d", url, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("decode %s response: %v", url, err)
	}
}

func postJSON(t *testing.T, url string, body any, expectedStatus int, target any) {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}

		reader = bytes.NewReader(data)
	}

	resp, err := http.Post(url, "application/json", reader)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		t.Fatalf("POST %s returned status %d, expected %d", url, resp.StatusCode, expectedStatus)
	}

	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			t.Fatalf("decode POST %s response: %v", url, err)
		}
	}
}
