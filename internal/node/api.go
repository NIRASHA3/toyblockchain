package node

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"toyblockchain/internal/blockchain"
)

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

	rawHeight := strings.TrimPrefix(r.URL.Path, "/blocks/")
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
