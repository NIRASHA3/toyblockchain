package node

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
)

const maxDiscoveryContacts = 32

// DiscoveryResult describes one peer-discovery run.
type DiscoveryResult struct {
	BeforeCount    int      `json:"before_count"`
	AfterCount     int      `json:"after_count"`
	PeersContacted int      `json:"peers_contacted"`
	PeersAdded     int      `json:"peers_added"`
	Discovered     []string `json:"discovered,omitempty"`
	Peers          []string `json:"peers"`
	Errors         []string `json:"errors,omitempty"`
}

// DiscoverPeers asks known peers for their peer lists and adds newly discovered peers.
//
// The process starts from the node's configured peers. Each reachable peer is queried
// through GET /peers. Newly discovered peers are added to the local peer set and may
// also be queried, up to maxDiscoveryContacts, so a node can learn a small local
// network from a single seed peer.
func (n *Node) DiscoverPeers(ctx context.Context) (DiscoveryResult, error) {
	initialPeers := n.Peers()
	result := DiscoveryResult{
		BeforeCount: len(initialPeers),
	}

	queue := append([]string(nil), initialPeers...)
	contacted := make(map[string]struct{})
	selfURL := n.SelfURL()

	for len(queue) > 0 && len(contacted) < maxDiscoveryContacts {
		peer, ok := nextDiscoveryPeer(&queue, contacted, selfURL)
		if !ok {
			continue
		}

		n.discoverFromPeer(ctx, peer, &queue, &result)
	}

	sort.Strings(result.Discovered)

	result.Peers = n.Peers()
	result.AfterCount = len(result.Peers)

	return result, nil
}

func nextDiscoveryPeer(queue *[]string, contacted map[string]struct{}, selfURL string) (string, bool) {
	peer := normalizePeer((*queue)[0])
	*queue = (*queue)[1:]

	if peer == "" || peer == selfURL {
		return "", false
	}

	if _, exists := contacted[peer]; exists {
		return "", false
	}

	contacted[peer] = struct{}{}
	return peer, true
}

func (n *Node) discoverFromPeer(ctx context.Context, peer string, queue *[]string, result *DiscoveryResult) {
	peerList, err := fetchPeerList(ctx, peer)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		n.logger.Printf("peer discovery failed peer=%s error=%v", peer, err)
		return
	}

	result.PeersContacted++

	for _, discovered := range peerList {
		n.addDiscoveredPeer(peer, discovered, queue, result)
	}
}

func (n *Node) addDiscoveredPeer(sourcePeer string, discovered string, queue *[]string, result *DiscoveryResult) {
	discovered = normalizePeer(discovered)
	if discovered == "" || discovered == n.SelfURL() {
		return
	}

	if !n.AddPeer(discovered) {
		return
	}

	result.PeersAdded++
	result.Discovered = append(result.Discovered, discovered)
	*queue = append(*queue, discovered)

	n.logger.Printf("peer discovered via=%s peer=%s", sourcePeer, discovered)
}

func fetchPeerList(ctx context.Context, peer string) ([]string, error) {
	peer = normalizePeer(peer)
	if peer == "" {
		return nil, fmt.Errorf("peer URL is required")
	}

	requestCtx, cancel := context.WithTimeout(ctx, peerRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, peer+"/peers", nil)
	if err != nil {
		return nil, fmt.Errorf("create peer discovery request for %s: %w", peer, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discover peers from %s: %w", peer, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("peer %s returned %s during discovery", peer, resp.Status)
	}

	var response peersResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxNodeRequestBodyBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("decode peer discovery response from %s: %w", peer, err)
	}

	return response.Peers, nil
}
