package main

import (
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
	peersRaw := fs.String("peers", "", "comma-separated peer base URLs, for example http://127.0.0.1:8082,http://127.0.0.1:8083")

	if err := fs.Parse(args); err != nil {
		return err
	}

	logger := log.New(stderr, fmt.Sprintf("[node %s] ", *addr), log.LstdFlags)

	n, err := networknode.New(networknode.Config{
		DataPath:    cfg.dataPath,
		ChainConfig: bcfg,
		Peers:       splitPeerList(*peersRaw),
		Logger:      logger,
	})
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           n.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	fmt.Fprintf(stdout, "serving networked node on %s using state %s\n", *addr, cfg.dataPath)
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
