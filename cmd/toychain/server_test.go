package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"toyblockchain/internal/blockchain"
)

type apiTestFixture struct {
	ChainPath   string
	AliceAddr   string
	BobAddr     string
	AliceWallet string
	BobWallet   string
	TxID        string
}

func buildAPITestChain(t *testing.T) apiTestFixture {
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
	return apiTestFixture{ChainPath: chainPath, AliceAddr: aliceMeta.Address, BobAddr: bobMeta.Address, AliceWallet: aliceWallet, BobWallet: bobWallet, TxID: state.Chain[2].Transactions[0].ID}
}

func performAPIRequest(t *testing.T, handler http.Handler, method string, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func performJSONAPIRequest(t *testing.T, handler http.Handler, method string, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatalf("encode request body: %v", err)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestAPIHealthAndValidate(t *testing.T) {
	fixture := buildAPITestChain(t)
	handler := newAPIServer(fixture.ChainPath, blockchain.Config{Difficulty: 1, MaxBlockTx: blockchain.DefaultMaxBlockTx})

	rec := performAPIRequest(t, handler, http.MethodGet, "/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status": "ok"`) || !strings.Contains(rec.Body.String(), `"write_enabled": true`) {
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

func TestAPIBlockchainEndpoints(t *testing.T) {
	fixture := buildAPITestChain(t)
	handler := newAPIServer(fixture.ChainPath, blockchain.Config{Difficulty: 1, MaxBlockTx: blockchain.DefaultMaxBlockTx})

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
	if balances.Balances[fixture.AliceAddr] != 60 || balances.Balances[fixture.BobAddr] != 40 {
		t.Fatalf("balances = %+v, want alice=60 bob=40", balances.Balances)
	}

	rec = performAPIRequest(t, handler, http.MethodGet, "/transactions/"+fixture.TxID)
	if rec.Code != http.StatusOK {
		t.Fatalf("transaction status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var txResponse transactionLookupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &txResponse); err != nil {
		t.Fatalf("decode transaction: %v", err)
	}
	if txResponse.Transaction.ID != fixture.TxID || txResponse.BlockHeight != 2 || txResponse.Pending {
		t.Fatalf("transaction response = %+v", txResponse)
	}
}

func TestAPIMerkleProofAndErrors(t *testing.T) {
	fixture := buildAPITestChain(t)
	handler := newAPIServer(fixture.ChainPath, blockchain.Config{Difficulty: 1, MaxBlockTx: blockchain.DefaultMaxBlockTx})

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

func TestAPIFaucetAndMine(t *testing.T) {
	fixture := buildAPITestChain(t)
	handler := newAPIServer(fixture.ChainPath, blockchain.Config{Difficulty: 1, MaxBlockTx: blockchain.DefaultMaxBlockTx})

	rec := performJSONAPIRequest(t, handler, http.MethodPost, "/faucet", faucetRequest{To: fixture.BobAddr, Amount: 25, Memo: "api faucet"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("faucet status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var submit transactionSubmitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &submit); err != nil {
		t.Fatalf("decode faucet response: %v", err)
	}
	if submit.PendingCount != 1 || !submit.Transaction.IsFaucet() || submit.Transaction.To != fixture.BobAddr {
		t.Fatalf("faucet response = %+v", submit)
	}

	rec = performJSONAPIRequest(t, handler, http.MethodPost, "/mine", map[string]any{})
	if rec.Code != http.StatusCreated {
		t.Fatalf("mine status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var mined mineResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &mined); err != nil {
		t.Fatalf("decode mine response: %v", err)
	}
	if mined.Block.Height != 3 || mined.PendingCount != 0 || mined.Status != "mined" {
		t.Fatalf("mine response = %+v", mined)
	}

	rec = performAPIRequest(t, handler, http.MethodGet, "/balances")
	if rec.Code != http.StatusOK {
		t.Fatalf("balances status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var balances balancesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &balances); err != nil {
		t.Fatalf("decode balances: %v", err)
	}
	if balances.Balances[fixture.BobAddr] != 65 {
		t.Fatalf("bob balance = %d, want 65", balances.Balances[fixture.BobAddr])
	}
}

func TestAPISignedTransactionSubmission(t *testing.T) {
	fixture := buildAPITestChain(t)
	handler := newAPIServer(fixture.ChainPath, blockchain.Config{Difficulty: 1, MaxBlockTx: blockchain.DefaultMaxBlockTx})

	state, err := blockchain.LoadState(fixture.ChainPath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	wallet, err := blockchain.LoadEncryptedWallet(fixture.AliceWallet, "alice-pass")
	if err != nil {
		t.Fatalf("load alice wallet: %v", err)
	}
	nonce, err := state.NextNonce(wallet.Address)
	if err != nil {
		t.Fatalf("next nonce: %v", err)
	}
	tx, err := blockchain.NewSignedTransfer(wallet, fixture.BobAddr, 10, nonce, "api signed tx", time.Now())
	if err != nil {
		t.Fatalf("new signed transfer: %v", err)
	}

	rec := performJSONAPIRequest(t, handler, http.MethodPost, "/transactions", tx)
	if rec.Code != http.StatusCreated {
		t.Fatalf("submit status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var submit transactionSubmitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &submit); err != nil {
		t.Fatalf("decode submit response: %v", err)
	}
	if submit.PendingCount != 1 || submit.Transaction.ID != tx.ID {
		t.Fatalf("submit response = %+v", submit)
	}

	rec = performAPIRequest(t, handler, http.MethodGet, "/balances?pending=true")
	if rec.Code != http.StatusOK {
		t.Fatalf("pending balances status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var balances balancesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &balances); err != nil {
		t.Fatalf("decode pending balances: %v", err)
	}
	if balances.Balances[fixture.AliceAddr] != 50 || balances.Balances[fixture.BobAddr] != 50 {
		t.Fatalf("pending balances = %+v, want alice=50 bob=50", balances.Balances)
	}
}

func TestAPISignedTransactionRejectsUnsignedAndFaucetSubmission(t *testing.T) {
	fixture := buildAPITestChain(t)
	handler := newAPIServer(fixture.ChainPath, blockchain.Config{Difficulty: 1, MaxBlockTx: blockchain.DefaultMaxBlockTx})

	unsigned := blockchain.Transaction{ID: "bad", From: fixture.AliceAddr, To: fixture.BobAddr, Amount: 1, CreatedAt: time.Now().UnixNano(), Nonce: 2}
	rec := performJSONAPIRequest(t, handler, http.MethodPost, "/transactions", unsigned)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unsigned status = %d, body = %s", rec.Code, rec.Body.String())
	}

	faucetTx, err := blockchain.NewFaucet(fixture.BobAddr, 1, "bad route", time.Now())
	if err != nil {
		t.Fatalf("new faucet: %v", err)
	}
	rec = performJSONAPIRequest(t, handler, http.MethodPost, "/transactions", faucetTx)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("faucet via transactions status = %d, body = %s", rec.Code, rec.Body.String())
	}
}
