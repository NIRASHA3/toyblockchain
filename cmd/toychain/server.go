package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"toyblockchain/internal/blockchain"
)

type apiServer struct {
	dataPath string
	cfg      blockchain.Config
	apiToken string
}

type apiErrorResponse struct {
	Error string `json:"error"`
}

type healthResponse struct {
	Status         string `json:"status"`
	Service        string `json:"service"`
	ReadOnly       bool   `json:"read_only"`
	WriteEnabled   bool   `json:"write_enabled"`
	WriteProtected bool   `json:"write_protected"`
}

type chainResponse struct {
	Height       int                      `json:"height"`
	BlockCount   int                      `json:"block_count"`
	PendingCount int                      `json:"pending_count"`
	Chain        []blockchain.Block       `json:"chain"`
	Pending      []blockchain.Transaction `json:"pending"`
}

type blocksResponse struct {
	Count  int                `json:"count"`
	Blocks []blockchain.Block `json:"blocks"`
}

type balancesResponse struct {
	IncludePending bool                `json:"include_pending"`
	Balances       blockchain.Balances `json:"balances"`
}

type validateResponse struct {
	Valid         bool   `json:"valid"`
	BlocksChecked int    `json:"blocks_checked,omitempty"`
	Error         string `json:"error,omitempty"`
}

type transactionLookupResponse struct {
	Transaction      blockchain.Transaction `json:"transaction"`
	BlockHeight      int                    `json:"block_height"`
	TransactionIndex int                    `json:"transaction_index"`
	Pending          bool                   `json:"pending"`
}

type merkleProofResponse struct {
	BlockHeight      int                          `json:"block_height"`
	TransactionIndex int                          `json:"transaction_index"`
	TransactionID    string                       `json:"transaction_id"`
	TransactionHash  string                       `json:"transaction_hash"`
	MerkleRoot       string                       `json:"merkle_root"`
	Proof            []blockchain.MerkleProofStep `json:"proof"`
	Valid            bool                         `json:"valid"`
}

type faucetRequest struct {
	To     string `json:"to"`
	Amount int64  `json:"amount"`
	Memo   string `json:"memo"`
}

type transactionSubmitResponse struct {
	Status       string                 `json:"status"`
	Transaction  blockchain.Transaction `json:"transaction"`
	PendingCount int                    `json:"pending_count"`
}

type mineResponse struct {
	Status       string           `json:"status"`
	Block        blockchain.Block `json:"block"`
	Nonce        uint64           `json:"nonce"`
	Attempts     uint64           `json:"attempts"`
	Duration     string           `json:"duration"`
	Workers      int              `json:"workers"`
	PendingCount int              `json:"pending_count"`
}

func newAPIServer(dataPath string, cfg blockchain.Config, apiToken string) http.Handler {
	s := &apiServer{dataPath: dataPath, cfg: cfg, apiToken: apiToken}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/chain", s.handleChain)
	mux.HandleFunc("/blocks", s.handleBlocks)
	mux.HandleFunc("/blocks/", s.handleBlockByHeight)
	mux.HandleFunc("/balances", s.handleBalances)
	mux.HandleFunc("/transactions", s.handleSubmitTransaction)
	mux.HandleFunc("/transactions/", s.handleTransactionByID)
	mux.HandleFunc("/faucet", s.handleFaucet)
	mux.HandleFunc("/mine", s.handleMine)
	mux.HandleFunc("/merkle-proof", s.handleMerkleProof)
	mux.HandleFunc("/validate", s.handleValidate)
	return mux
}

func (s *apiServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Service: "toychain", ReadOnly: false, WriteEnabled: true, WriteProtected: s.apiToken != ""})
}

func (s *apiServer) handleChain(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	state, ok := s.loadValidStateForAPI(w)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, chainResponse{
		Height:       len(state.Chain) - 1,
		BlockCount:   len(state.Chain),
		PendingCount: len(state.Pending),
		Chain:        state.Chain,
		Pending:      state.Pending,
	})
}

func (s *apiServer) handleBlocks(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	state, ok := s.loadValidStateForAPI(w)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, blocksResponse{Count: len(state.Chain), Blocks: state.Chain})
}

func (s *apiServer) handleBlockByHeight(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	heightText := strings.TrimPrefix(r.URL.Path, "/blocks/")
	height, err := strconv.Atoi(heightText)
	if err != nil || heightText == "" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid block height %q", heightText))
		return
	}
	state, ok := s.loadValidStateForAPI(w)
	if !ok {
		return
	}
	if height < 0 || height >= len(state.Chain) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("block height %d not found", height))
		return
	}
	writeJSON(w, http.StatusOK, state.Chain[height])
}

func (s *apiServer) handleBalances(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	state, ok := s.loadValidStateForAPI(w)
	if !ok {
		return
	}
	includePending := r.URL.Query().Get("pending") == "true"
	var (
		balances blockchain.Balances
		err      error
	)
	if includePending {
		balances, err = state.BalancesIncludingPending()
	} else {
		balances, err = state.Balances()
	}
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, balancesResponse{IncludePending: includePending, Balances: balances})
}

func (s *apiServer) handleTransactionByID(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	txID := strings.TrimPrefix(r.URL.Path, "/transactions/")
	if txID == "" {
		writeError(w, http.StatusBadRequest, "transaction id is required")
		return
	}
	state, ok := s.loadValidStateForAPI(w)
	if !ok {
		return
	}
	if result, found := findTransaction(state, txID); found {
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeError(w, http.StatusNotFound, fmt.Sprintf("transaction %q not found", txID))
}

func (s *apiServer) handleSubmitTransaction(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !s.requireWriteToken(w, r) {
		return
	}
	var tx blockchain.Transaction
	if err := decodeJSONBody(w, r, &tx); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if tx.IsFaucet() {
		writeError(w, http.StatusBadRequest, "use POST /faucet for faucet transactions")
		return
	}
	state, ok := s.loadValidStateForAPI(w)
	if !ok {
		return
	}
	if err := state.AddPending(tx); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := blockchain.SaveState(s.dataPath, state); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, transactionSubmitResponse{Status: "accepted", Transaction: tx, PendingCount: len(state.Pending)})
}

func (s *apiServer) handleFaucet(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !s.requireWriteToken(w, r) {
		return
	}
	var req faucetRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	state, ok := s.loadValidStateForAPI(w)
	if !ok {
		return
	}
	memo := req.Memo
	if strings.TrimSpace(memo) == "" {
		memo = "faucet funding"
	}
	tx, err := blockchain.NewFaucet(req.To, req.Amount, memo, time.Now())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := state.AddPending(tx); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := blockchain.SaveState(s.dataPath, state); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, transactionSubmitResponse{Status: "accepted", Transaction: tx, PendingCount: len(state.Pending)})
}

func (s *apiServer) handleMine(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if !s.requireWriteToken(w, r) {
		return
	}
	if r.Body != nil && r.ContentLength != 0 {
		var ignored map[string]any
		if err := decodeJSONBody(w, r, &ignored); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	state, ok := s.loadValidStateForAPI(w)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	block, stats, err := state.MinePending(ctx, s.cfg, time.Now())
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := blockchain.SaveState(s.dataPath, state); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, mineResponse{
		Status:       "mined",
		Block:        block,
		Nonce:        stats.Nonce,
		Attempts:     stats.Attempts,
		Duration:     stats.Duration.Round(time.Millisecond).String(),
		Workers:      stats.Workers,
		PendingCount: len(state.Pending),
	})
}

func (s *apiServer) handleMerkleProof(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	height, err := requiredIntQuery(r, "height")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	txIndex, err := requiredIntQuery(r, "tx")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	state, ok := s.loadValidStateForAPI(w)
	if !ok {
		return
	}
	result, err := buildMerkleProofResponse(state, height, txIndex)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *apiServer) handleValidate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	state, err := blockchain.LoadState(s.dataPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := blockchain.ValidateChain(state.Chain, s.cfg.Difficulty); err != nil {
		writeJSON(w, http.StatusOK, validateResponse{Valid: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, validateResponse{Valid: true, BlocksChecked: len(state.Chain)})
}

func (s *apiServer) requireWriteToken(w http.ResponseWriter, r *http.Request) bool {
	if s.apiToken == "" {
		return true
	}
	provided := r.Header.Get("X-API-Token")
	if provided == "" {
		writeError(w, http.StatusUnauthorized, "missing API token")
		return false
	}
	if subtle.ConstantTimeCompare([]byte(provided), []byte(s.apiToken)) != 1 {
		writeError(w, http.StatusForbidden, "invalid API token")
		return false
	}
	return true
}

func (s *apiServer) loadValidStateForAPI(w http.ResponseWriter) (blockchain.State, bool) {
	state, err := blockchain.LoadState(s.dataPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return blockchain.State{}, false
	}
	if err := blockchain.ValidateChain(state.Chain, s.cfg.Difficulty); err != nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("invalid chain: %v", err))
		return blockchain.State{}, false
	}
	return state, true
}

func findTransaction(state blockchain.State, txID string) (transactionLookupResponse, bool) {
	for _, block := range state.Chain {
		for index, tx := range block.Transactions {
			if tx.ID == txID {
				return transactionLookupResponse{Transaction: tx, BlockHeight: block.Height, TransactionIndex: index, Pending: false}, true
			}
		}
	}
	for index, tx := range state.Pending {
		if tx.ID == txID {
			return transactionLookupResponse{Transaction: tx, BlockHeight: -1, TransactionIndex: index, Pending: true}, true
		}
	}
	return transactionLookupResponse{}, false
}

func buildMerkleProofResponse(state blockchain.State, height int, txIndex int) (merkleProofResponse, error) {
	if height < 0 || height >= len(state.Chain) {
		return merkleProofResponse{}, fmt.Errorf("height %d out of range", height)
	}
	block := state.Chain[height]
	if txIndex < 0 || txIndex >= len(block.Transactions) {
		return merkleProofResponse{}, fmt.Errorf("transaction index %d out of range for block %d", txIndex, height)
	}
	tx := block.Transactions[txIndex]
	txHash := blockchain.TransactionHash(tx)
	proof, err := blockchain.BuildMerkleProof(block.Transactions, txIndex)
	if err != nil {
		return merkleProofResponse{}, err
	}
	return merkleProofResponse{
		BlockHeight:      block.Height,
		TransactionIndex: txIndex,
		TransactionID:    tx.ID,
		TransactionHash:  txHash,
		MerkleRoot:       block.MerkleRoot,
		Proof:            proof,
		Valid:            blockchain.VerifyMerkleProof(txHash, proof, block.MerkleRoot),
	}, nil
}

func requiredIntQuery(r *http.Request, name string) (int, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return 0, fmt.Errorf("query parameter %q is required", name)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("query parameter %q must be an integer", name)
	}
	return parsed, nil
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	writeError(w, http.StatusMethodNotAllowed, fmt.Sprintf("method %s not allowed", r.Method))
	return false
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, value any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("decode JSON body: %w", err)
	}
	if decoder.More() {
		return fmt.Errorf("decode JSON body: unexpected extra JSON data")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiErrorResponse{Error: message})
}
