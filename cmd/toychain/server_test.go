package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"toyblockchain/internal/blockchain"
)

func buildAPITestChain(t *testing.T) (string, string, string, string) {
	t.Helper()
	base := t.TempDir()
	chainPath := base + "/chain.json"
	aliceWallet := base + "/alice.wallet.json"
	bobWallet := base + "/bob.wallet.json"
	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := run([]string{"wallet", "new", "-out", aliceWallet, "-passphrase", "alice-pass"}, &out, &errOut); err != nil {
		t.Fatalf("wallet new alice: %v", err)
	}
	out.Reset()
	if err := run([]string{"wallet", "new", "-out", bobWallet, "-passphrase", "bob-pass"}, &out, &errOut); err != nil {
		t.Fatalf("wallet new bob: %v", err)
	}
	aliceMeta, err := blockchain.ReadWalletMetadata(aliceWallet)
	if err != nil {
		t.Fatalf("read alice metadata: %v", err)
	}
	bobMeta, err := blockchain.ReadWalletMetadata(bobWallet)
	if err != nil {
		t.Fatalf("read bob metadata: %v", err)
	}

	commands := [][]string{
		{"-data", chainPath, "-difficulty", "1", "init", "-force"},
		{"-data", chainPath, "-difficulty", "1", "faucet", "-to", aliceMeta.Address, "-amount", "100"},
		{"-data", chainPath, "-difficulty", "1", "mine"},
		{"-data", chainPath, "-difficulty", "1", "tx", "-wallet", aliceWallet, "-passphrase", "alice-pass", "-to", bobMeta.Address, "-amount", "40"},
		{"-data", chainPath, "-difficulty", "1", "mine"},
	}
	for _, args := range commands {
		if err := run(args, &out, &errOut); err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
	}
	state, err := blockchain.LoadState(chainPath)
	if err != nil {
		t.Fatalf("load chain: %v", err)
	}
	return chainPath, aliceMeta.Address, bobMeta.Address, state.Chain[2].Transactions[0].ID
}

func performAPIRequest(t *testing.T, handler http.Handler, method string, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestReadOnlyAPIHealthAndValidate(t *testing.T) {
	chainPath, _, _, _ := buildAPITestChain(t)
	handler := newAPIServer(chainPath, blockchain.Config{Difficulty: 1, MaxBlockTx: blockchain.DefaultMaxBlockTx})

	rec := performAPIRequest(t, handler, http.MethodGet, "/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status": "ok"`) || !strings.Contains(rec.Body.String(), `"read_only": true`) {
		t.Fatalf("health body missing expected fields: %s", rec.Body.String())
	}

	rec = performAPIRequest(t, handler, http.MethodGet, "/validate")
	if rec.Code != http.StatusOK {
		t.Fatalf("validate status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var validation validateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &validation); err != nil {
		t.Fatalf("decode validate response: %v", err)
	}
	if !validation.Valid || validation.BlocksChecked != 3 {
		t.Fatalf("validate response = %+v, want valid with 3 blocks", validation)
	}
}

func TestReadOnlyAPIBlockchainEndpoints(t *testing.T) {
	chainPath, alice, bob, txID := buildAPITestChain(t)
	handler := newAPIServer(chainPath, blockchain.Config{Difficulty: 1, MaxBlockTx: blockchain.DefaultMaxBlockTx})

	rec := performAPIRequest(t, handler, http.MethodGet, "/blocks/2")
	if rec.Code != http.StatusOK {
		t.Fatalf("block status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var block blockchain.Block
	if err := json.Unmarshal(rec.Body.Bytes(), &block); err != nil {
		t.Fatalf("decode block: %v", err)
	}
	if block.Height != 2 || block.MerkleRoot == "" || len(block.Transactions) != 1 {
		t.Fatalf("block response = %+v", block)
	}

	rec = performAPIRequest(t, handler, http.MethodGet, "/balances")
	if rec.Code != http.StatusOK {
		t.Fatalf("balances status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var balances balancesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &balances); err != nil {
		t.Fatalf("decode balances: %v", err)
	}
	if balances.Balances[alice] != 60 || balances.Balances[bob] != 40 {
		t.Fatalf("balances = %+v, want alice=60 bob=40", balances.Balances)
	}

	rec = performAPIRequest(t, handler, http.MethodGet, "/transactions/"+txID)
	if rec.Code != http.StatusOK {
		t.Fatalf("transaction status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var txResponse transactionLookupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &txResponse); err != nil {
		t.Fatalf("decode transaction: %v", err)
	}
	if txResponse.Transaction.ID != txID || txResponse.BlockHeight != 2 || txResponse.Pending {
		t.Fatalf("transaction response = %+v", txResponse)
	}
}

func TestReadOnlyAPIMerkleProofAndErrors(t *testing.T) {
	chainPath, _, _, _ := buildAPITestChain(t)
	handler := newAPIServer(chainPath, blockchain.Config{Difficulty: 1, MaxBlockTx: blockchain.DefaultMaxBlockTx})

	rec := performAPIRequest(t, handler, http.MethodGet, "/merkle-proof?height=2&tx=0")
	if rec.Code != http.StatusOK {
		t.Fatalf("merkle proof status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var proof merkleProofResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &proof); err != nil {
		t.Fatalf("decode proof: %v", err)
	}
	if !proof.Valid || proof.BlockHeight != 2 || proof.TransactionIndex != 0 || proof.MerkleRoot == "" {
		t.Fatalf("proof response = %+v", proof)
	}

	rec = performAPIRequest(t, handler, http.MethodGet, "/merkle-proof?height=2&tx=99")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("invalid proof status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = performAPIRequest(t, handler, http.MethodPost, "/health")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
