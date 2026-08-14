package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"toyblockchain/internal/blockchain"
)

const maxNodeRequestBodyBytes int64 = 1 << 20

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}
type peersResponse struct {
	Count int      `json:"count"`
	Peers []string `json:"peers"`
}
type chainResponse struct {
	Height       int                      `json:"height"`
	BlockCount   int                      `json:"block_count"`
	HeadHash     string                   `json:"head_hash"`
	PendingCount int                      `json:"pending_count"`
	Chain        []blockchain.Block       `json:"chain"`
	Pending      []blockchain.Transaction `json:"pending"`
}
type blocksResponse struct {
	Count  int                `json:"count"`
	Blocks []blockchain.Block `json:"blocks"`
}
type blockResponse struct {
	Block blockchain.Block `json:"block"`
}
type pendingResponse struct {
	Count   int                      `json:"count"`
	Pending []blockchain.Transaction `json:"pending"`
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
type errorResponse struct {
	Error string `json:"error"`
}
type transactionResponse struct {
	ID       string   `json:"id"`
	Accepted bool     `json:"accepted"`
	Gossiped int      `json:"gossiped"`
	Errors   []string `json:"errors,omitempty"`
}
type mineResponse struct {
	Block    blockchain.Block       `json:"block"`
	Stats    blockchain.MiningStats `json:"stats"`
	Gossiped int                    `json:"gossiped"`
	Errors   []string               `json:"errors,omitempty"`
}
type peerBlockResponse struct {
	Height   int          `json:"height"`
	Hash     string       `json:"hash"`
	Accepted bool         `json:"accepted"`
	Gossiped int          `json:"gossiped"`
	Reorg    *ReorgResult `json:"reorg,omitempty"`
	Errors   []string     `json:"errors,omitempty"`
}
type syncRequest struct {
	Peer string `json:"peer"`
}
type syncResponse struct {
	Peer             string   `json:"peer"`
	BeforeHeight     int      `json:"before_height"`
	PeerHeight       int      `json:"peer_height"`
	AfterHeight      int      `json:"after_height"`
	HeadHash         string   `json:"head_hash"`
	BlocksDownloaded int      `json:"blocks_downloaded"`
	Synced           bool     `json:"synced"`
	Errors           []string `json:"errors,omitempty"`
}

// Handler returns the HTTP API for this networked node.
func (n *Node) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", n.handleHealth)
	mux.HandleFunc("/status", n.handleStatus)
	mux.HandleFunc("/peers", n.handlePeers)
	mux.HandleFunc("/chain", n.handleChain)
	mux.HandleFunc("/blocks", n.handleBlocks)
	mux.HandleFunc("/blocks/", n.handleBlockByHeight)
	mux.HandleFunc("/pending", n.handlePending)
	mux.HandleFunc("/balances", n.handleBalances)
	mux.HandleFunc("/validate", n.handleValidate)
	mux.HandleFunc("/transactions", n.handleSubmitTransaction)
	mux.HandleFunc("/peer/transactions", n.handlePeerTransaction)
	mux.HandleFunc("/mine", n.handleMine)
	mux.HandleFunc("/peer/blocks", n.handlePeerBlock)
	mux.HandleFunc("/sync", n.handleSync)
	mux.HandleFunc("/resolve-fork", n.handleResolveFork)
	mux.HandleFunc("/peer/status", n.handleStatus)
	mux.HandleFunc("/peer/blocks/", n.handlePeerBlockByHeight)
	mux.HandleFunc("/discover-peers", n.handleDiscoverPeers)
	return mux
}
func (n *Node) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Service: "toyblockchain-node",
	})
}
func (n *Node) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, n.Status())
}
func (n *Node) handlePeers(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	peers := n.Peers()
	writeJSON(w, http.StatusOK, peersResponse{
		Count: len(peers),
		Peers: peers,
	})
}
func (n *Node) handleDiscoverPeers(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	result, err := n.DiscoverPeers(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}
func (n *Node) handleChain(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	state := n.Snapshot()
	head := state.Chain[len(state.Chain)-1]
	writeJSON(w, http.StatusOK, chainResponse{
		Height:       len(state.Chain) - 1,
		BlockCount:   len(state.Chain),
		HeadHash:     head.Hash,
		PendingCount: len(state.Pending),
		Chain:        state.Chain,
		Pending:      state.Pending,
	})
}
func (n *Node) handleBlocks(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	state := n.Snapshot()
	writeJSON(w, http.StatusOK, blocksResponse{
		Count:  len(state.Chain),
		Blocks: state.Chain,
	})
}
func (n *Node) handleBlockByHeight(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	n.writeBlockByHeight(w, strings.TrimPrefix(r.URL.Path, "/blocks/"))
}
func (n *Node) handlePeerBlockByHeight(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	n.writeBlockByHeight(w, strings.TrimPrefix(r.URL.Path, "/peer/blocks/"))
}
func (n *Node) writeBlockByHeight(w http.ResponseWriter, rawHeight string) {
	if rawHeight == "" {
		writeError(w, http.StatusBadRequest, "missing block height")
		return
	}
	height, err := strconv.Atoi(rawHeight)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid block height %q", rawHeight))
		return
	}
	state := n.Snapshot()
	if height < 0 || height >= len(state.Chain) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("block height %d not found", height))
		return
	}
	writeJSON(w, http.StatusOK, blockResponse{
		Block: state.Chain[height],
	})
}
func (n *Node) handlePending(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	state := n.Snapshot()
	writeJSON(w, http.StatusOK, pendingResponse{
		Count:   len(state.Pending),
		Pending: state.Pending,
	})
}
func (n *Node) handleBalances(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	includePending := strings.EqualFold(r.URL.Query().Get("pending"), "true")
	state := n.Snapshot()
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
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, balancesResponse{
		IncludePending: includePending,
		Balances:       balances,
	})
}
func (n *Node) handleValidate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	state := n.Snapshot()
	if err := blockchain.ValidateChainWithConfig(state.Chain, n.cfg); err != nil {
		writeJSON(w, http.StatusOK, validateResponse{
			Valid: false,
			Error: err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, validateResponse{
		Valid:         true,
		BlocksChecked: len(state.Chain),
	})
}
func (n *Node) handleSubmitTransaction(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var tx blockchain.Transaction
	if !decodeStrictJSON(w, r, &tx, "transaction") {
		return
	}
	result, err := n.SubmitTransaction(r.Context(), tx)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	status := http.StatusOK
	if result.Accepted {
		status = http.StatusCreated
	}
	writeJSON(w, status, transactionResponse{
		ID:       result.ID,
		Accepted: result.Accepted,
		Gossiped: result.Gossiped,
		Errors:   result.Errors,
	})
}
func (n *Node) handlePeerTransaction(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var request transactionGossipRequest
	if !decodeStrictJSON(w, r, &request, "peer transaction") {
		return
	}
	result, err := n.ReceivePeerTransaction(r.Context(), request.Transaction, request.Origin)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	status := http.StatusOK
	if result.Accepted {
		status = http.StatusCreated
	}
	writeJSON(w, status, transactionResponse{
		ID:       result.ID,
		Accepted: result.Accepted,
		Gossiped: result.Gossiped,
		Errors:   result.Errors,
	})
}
func (n *Node) handleMine(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	block, stats, result, err := n.MineAndGossip(r.Context(), time.Now())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, mineResponse{
		Block:    block,
		Stats:    stats,
		Gossiped: result.Gossiped,
		Errors:   result.Errors,
	})
}
func (n *Node) handlePeerBlock(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var request blockGossipRequest
	if !decodeStrictJSON(w, r, &request, "peer block") {
		return
	}
	result, err := n.ReceivePeerBlock(r.Context(), request.Block, request.Origin)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	status := http.StatusOK
	if result.Accepted {
		status = http.StatusCreated
	}
	writeJSON(w, status, peerBlockResponse{
		Height:   result.Height,
		Hash:     result.Hash,
		Accepted: result.Accepted,
		Gossiped: result.Gossiped,
		Reorg:    result.Reorg,
		Errors:   result.Errors,
	})
}
func (n *Node) handleSync(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	peer, ok := n.peerFromRequest(w, r, "sync")
	if !ok {
		return
	}
	result, err := n.SyncFromPeer(r.Context(), peer)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, syncResponse{
		Peer:             result.Peer,
		BeforeHeight:     result.BeforeHeight,
		PeerHeight:       result.PeerHeight,
		AfterHeight:      result.AfterHeight,
		HeadHash:         result.HeadHash,
		BlocksDownloaded: result.BlocksDownloaded,
		Synced:           result.Synced,
		Errors:           result.Errors,
	})
}
func (n *Node) handleResolveFork(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	peer, ok := n.peerFromRequest(w, r, "fork resolution")
	if !ok {
		return
	}
	result, err := n.ResolveForkFromPeer(r.Context(), peer)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (n *Node) peerFromRequest(w http.ResponseWriter, r *http.Request, action string) (string, bool) {
	var request syncRequest
	if !decodeOptionalStrictJSON(w, r, &request, fmt.Sprintf("%s request", action)) {
		return "", false
	}
	peer := normalizePeer(request.Peer)
	if peer == "" {
		peers := n.Peers()
		if len(peers) == 0 {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("%s peer is required when node has no configured peers", action))
			return "", false
		}
		peer = peers[0]
	}
	return peer, true
}
func decodeStrictJSON(w http.ResponseWriter, r *http.Request, dst any, label string) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxNodeRequestBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("decode %s: %v", label, err))
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("decode %s: request body must contain only one JSON value", label))
		return false
	}
	return true
}
func decodeOptionalStrictJSON(w http.ResponseWriter, r *http.Request, dst any, label string) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxNodeRequestBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		writeError(w, http.StatusBadRequest, fmt.Sprintf("decode %s: %v", label, err))
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("decode %s: request body must contain only one JSON value", label))
		return false
	}
	return true
}
func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
