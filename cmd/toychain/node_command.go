package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"toyblockchain/internal/blockchain"
	networknode "toyblockchain/internal/node"
)

func cmdNode(args []string, cfg cliConfig, bcfg blockchain.Config, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("node", flag.ContinueOnError)
	fs.SetOutput(stderr)

	addr := fs.String("addr", "127.0.0.1:8081", "networked node HTTP listen address")
	advertise := fs.String("advertise", "", "peer-visible node base URL, for example http://node1:8081")
	peersRaw := fs.String("peers", "", "comma-separated peer base URLs, for example http://127.0.0.1:8082,http://127.0.0.1:8083")
	discover := fs.Bool("discover", false, "discover additional peers from configured seed peers before serving")

	if err := fs.Parse(args); err != nil {
		return err
	}

	logger := log.New(stderr, fmt.Sprintf("[node %s] ", *addr), log.LstdFlags)

	selfURL := advertisedNodeURL(*addr, *advertise)
	n, err := networknode.New(networknode.Config{
		DataPath:    cfg.dataPath,
		ChainConfig: bcfg,
		SelfURL:     selfURL,
		Peers:       splitPeerList(*peersRaw),
		Logger:      logger,
	})
	if err != nil {
		return err
	}

	if *discover {
		discoveryCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		result, discoveryErr := n.DiscoverPeers(discoveryCtx)
		cancel()

		if discoveryErr != nil {
			fmt.Fprintf(stderr, "peer discovery failed: %v\n", discoveryErr)
		} else {
			fmt.Fprintf(
				stdout,
				"peer discovery: before=%d after=%d contacted=%d added=%d\n",
				result.BeforeCount,
				result.AfterCount,
				result.PeersContacted,
				result.PeersAdded,
			)
		}

		for _, discoveryErr := range result.Errors {
			fmt.Fprintf(stderr, "peer discovery warning: %s\n", discoveryErr)
		}
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           n.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	fmt.Fprintf(stdout, "serving networked node on %s using state %s\n", *addr, cfg.dataPath)
	fmt.Fprintf(stdout, "advertised node URL: %s\n", selfURL)

	if peers := n.Peers(); len(peers) > 0 {
		fmt.Fprintf(stdout, "configured peers: %s\n", strings.Join(peers, ", "))
	} else {
		fmt.Fprintln(stdout, "configured peers: none")
	}

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func splitPeerList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	peers := make([]string, 0, len(parts))

	for _, part := range parts {
		peer := strings.TrimSpace(part)
		if peer == "" {
			continue
		}

		peers = append(peers, peer)
	}

	return peers
}

func advertisedNodeURL(addr string, advertise string) string {
	advertise = strings.TrimSpace(advertise)
	if advertise != "" {
		return nodeBaseURL(advertise)
	}

	return nodeBaseURL(addr)
}

func nodeBaseURL(addr string) string {
	addr = strings.TrimSpace(addr)
	addr = strings.TrimRight(addr, "/")

	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}

	if strings.HasPrefix(addr, ":") {
		return "http://127.0.0.1" + addr
	}

	return "http://" + addr
}
