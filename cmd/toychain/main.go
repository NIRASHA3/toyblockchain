package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"toyblockchain/internal/blockchain"
)

var errValidationFailed = errors.New("chain validation failed")

type cliConfig struct {
	dataPath   string
	difficulty int
	maxBlockTx int
	workers    int
	timeout    time.Duration
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, errValidationFailed) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	global := flag.NewFlagSet("toychain", flag.ContinueOnError)
	global.SetOutput(stderr)

	cfg := cliConfig{}
	global.StringVar(&cfg.dataPath, "data", "toychain.json", "path to JSON chain state")
	global.IntVar(&cfg.difficulty, "difficulty", blockchain.DefaultDifficulty, "proof-of-work leading-zero hex digits")
	global.IntVar(&cfg.maxBlockTx, "max-block-tx", blockchain.DefaultMaxBlockTx, "maximum transactions per block")
	global.IntVar(&cfg.workers, "workers", 0, "mining workers; 0 means runtime.NumCPU")
	global.DurationVar(&cfg.timeout, "timeout", 15*time.Second, "mining timeout")

	if err := global.Parse(args); err != nil {
		return err
	}

	remaining := global.Args()
	if len(remaining) == 0 {
		printUsage(stdout)
		return nil
	}

	bcfg := blockchain.Config{Difficulty: cfg.difficulty, MaxBlockTx: cfg.maxBlockTx, Workers: cfg.workers}
	if err := bcfg.Validate(); err != nil {
		return err
	}

	command := remaining[0]
	commandArgs := remaining[1:]

	switch command {
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	case "wallet":
		return cmdWallet(commandArgs, stdout, stderr)
	case "init":
		return cmdInit(commandArgs, cfg, bcfg, stdout, stderr)
	case "faucet":
		return cmdFaucet(commandArgs, cfg, bcfg, stdout, stderr)
	case "tx":
		return cmdTransfer(commandArgs, cfg, bcfg, stdout, stderr)
	case "tx-sign":
		return cmdSignTransaction(commandArgs, cfg, bcfg, stdout, stderr)
	case "mine":
		return cmdMine(commandArgs, cfg, bcfg, stdout, stderr)
	case "print":
		return cmdPrint(commandArgs, cfg, bcfg, stdout, stderr)
	case "validate":
		return cmdValidate(commandArgs, cfg, bcfg, stdout, stderr)
	case "balances":
		return cmdBalances(commandArgs, cfg, bcfg, stdout, stderr)
	case "pending":
		return cmdPending(commandArgs, cfg, bcfg, stdout, stderr)
	case "merkle-proof":
		return cmdMerkleProof(commandArgs, cfg, bcfg, stdout, stderr)
	case "serve":
		return cmdServe(commandArgs, cfg, bcfg, stdout, stderr)
	case "tamper":
		return cmdTamper(commandArgs, cfg, bcfg, stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func cmdWallet(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printWalletUsage(stdout)
		return nil
	}
	switch args[0] {
	case "new":
		return cmdWalletNew(args[1:], stdout, stderr)
	case "show":
		return cmdWalletShow(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown wallet command %q", args[0])
	}
}

func cmdWalletNew(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("wallet new", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outPath := fs.String("out", "wallet.json", "encrypted wallet output file")
	passphrase := fs.String("passphrase", "", "wallet encryption passphrase")
	if err := fs.Parse(args); err != nil {
		return err
	}
	wallet, err := blockchain.NewWallet()
	if err != nil {
		return err
	}
	if err := blockchain.SaveEncryptedWallet(*outPath, wallet, *passphrase); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "created encrypted wallet %s\naddress: %s\n", *outPath, wallet.Address)
	return nil
}

func cmdWalletShow(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("wallet show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("path", "", "wallet file path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	metadata, err := blockchain.ReadWalletMetadata(*path)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "address: %s\npublic_key: %s\n", metadata.Address, metadata.PublicKey)
	return nil
}

func cmdInit(args []string, cfg cliConfig, _ blockchain.Config, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	force := fs.Bool("force", false, "overwrite existing state")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*force {
		if _, err := os.Stat(cfg.dataPath); err == nil {
			return fmt.Errorf("state file %q already exists; use init -force to overwrite", cfg.dataPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat state file %q: %w", cfg.dataPath, err)
		}
	}
	state := blockchain.NewState()
	if err := blockchain.SaveState(cfg.dataPath, state); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "initialised chain at %s with genesis hash %s\n", cfg.dataPath, state.Chain[0].Hash)
	return nil
}

func cmdFaucet(args []string, cfg cliConfig, bcfg blockchain.Config, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("faucet", flag.ContinueOnError)
	fs.SetOutput(stderr)
	to := fs.String("to", "", "recipient wallet address")
	amount := fs.Int64("amount", 0, "amount to mint")
	memo := fs.String("memo", "faucet funding", "transaction memo")
	if err := fs.Parse(args); err != nil {
		return err
	}
	state, err := loadValidState(cfg, bcfg)
	if err != nil {
		return err
	}
	tx, err := blockchain.NewFaucet(*to, *amount, *memo, time.Now())
	if err != nil {
		return err
	}
	if err := state.AddPending(tx); err != nil {
		return err
	}
	if err := blockchain.SaveState(cfg.dataPath, state); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "added faucet transaction %s: %s -> %s amount=%d\n", short(tx.ID), tx.From, tx.To, tx.Amount)
	return nil
}

func cmdTransfer(args []string, cfg cliConfig, bcfg blockchain.Config, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("tx", flag.ContinueOnError)
	fs.SetOutput(stderr)
	walletPath := fs.String("wallet", "", "encrypted sender wallet file")
	passphrase := fs.String("passphrase", "", "wallet passphrase")
	to := fs.String("to", "", "recipient wallet address")
	amount := fs.Int64("amount", 0, "amount to send")
	memo := fs.String("memo", "", "transaction memo")
	if err := fs.Parse(args); err != nil {
		return err
	}
	state, err := loadValidState(cfg, bcfg)
	if err != nil {
		return err
	}
	tx, err := signTransactionFromWallet(state, *walletPath, *passphrase, *to, *amount, *memo, time.Now())
	if err != nil {
		return err
	}
	if err := state.AddPending(tx); err != nil {
		return err
	}
	if err := blockchain.SaveState(cfg.dataPath, state); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "added signed pending transaction %s: %s -> %s amount=%d nonce=%d\n", short(tx.ID), tx.From, tx.To, tx.Amount, tx.Nonce)
	return nil
}

func cmdSignTransaction(args []string, cfg cliConfig, bcfg blockchain.Config, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("tx-sign", flag.ContinueOnError)
	fs.SetOutput(stderr)
	walletPath := fs.String("wallet", "", "encrypted sender wallet file")
	passphrase := fs.String("passphrase", "", "wallet passphrase")
	to := fs.String("to", "", "recipient wallet address")
	amount := fs.Int64("amount", 0, "amount to send")
	memo := fs.String("memo", "", "transaction memo")
	outPath := fs.String("out", "", "output signed transaction JSON file; omit or use - for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	state, err := loadValidState(cfg, bcfg)
	if err != nil {
		return err
	}
	tx, err := signTransactionFromWallet(state, *walletPath, *passphrase, *to, *amount, *memo, time.Now())
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if *outPath == "" || *outPath == "-" {
		_, err = stdout.Write(data)
		return err
	}
	if err := os.WriteFile(*outPath, data, 0o600); err != nil {
		return fmt.Errorf("write signed transaction %q: %w", *outPath, err)
	}
	fmt.Fprintf(stdout, "wrote signed transaction %s to %s\n", short(tx.ID), *outPath)
	return nil
}

func signTransactionFromWallet(state blockchain.State, walletPath string, passphrase string, to string, amount int64, memo string, now time.Time) (blockchain.Transaction, error) {
	wallet, err := blockchain.LoadEncryptedWallet(walletPath, passphrase)
	if err != nil {
		return blockchain.Transaction{}, err
	}
	nonce, err := state.NextNonce(wallet.Address)
	if err != nil {
		return blockchain.Transaction{}, err
	}
	return blockchain.NewSignedTransfer(wallet, to, amount, nonce, memo, now)
}

func cmdMine(args []string, cfg cliConfig, bcfg blockchain.Config, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("mine", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	state, err := loadValidState(cfg, bcfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()
	block, stats, err := state.MinePending(ctx, bcfg, time.Now())
	if err != nil {
		return err
	}
	if err := blockchain.SaveState(cfg.dataPath, state); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "mined block height=%d difficulty=%d hash=%s nonce=%d attempts=%d duration=%s workers=%d\n", block.Height, block.Difficulty, block.Hash, stats.Nonce, stats.Attempts, stats.Duration.Round(time.Millisecond), stats.Workers)
	return nil
}

func cmdPrint(args []string, cfg cliConfig, bcfg blockchain.Config, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("print", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	state, err := loadValidState(cfg, bcfg)
	if err != nil {
		return err
	}
	for _, block := range state.Chain {
		fmt.Fprintf(stdout, "Block %d\n", block.Height)
		fmt.Fprintf(stdout, "  timestamp:  %d\n", block.Timestamp)
		fmt.Fprintf(stdout, "  difficulty: %d\n", block.Difficulty)
		fmt.Fprintf(stdout, "  prev_hash:   %s\n", block.PrevHash)
		fmt.Fprintf(stdout, "  merkle_root: %s\n", block.MerkleRoot)
		fmt.Fprintf(stdout, "  nonce:       %d\n", block.Nonce)
		fmt.Fprintf(stdout, "  hash:        %s\n", block.Hash)
		fmt.Fprintf(stdout, "  tx_count:    %d\n", len(block.Transactions))
		for i, tx := range block.Transactions {
			fmt.Fprintf(stdout, "    [%d] %s -> %s amount=%d nonce=%d id=%s memo=%q\n", i, tx.From, tx.To, tx.Amount, tx.Nonce, short(tx.ID), tx.Memo)
		}
	}
	return nil
}

func cmdValidate(args []string, cfg cliConfig, bcfg blockchain.Config, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	state, err := blockchain.LoadState(cfg.dataPath)
	if err != nil {
		return err
	}
	if err := blockchain.ValidateChain(state.Chain, bcfg.Difficulty); err != nil {
		fmt.Fprintf(stdout, "INVALID: %v\n", err)
		return fmt.Errorf("%w: %v", errValidationFailed, err)
	}
	fmt.Fprintf(stdout, "VALID: %d blocks checked\n", len(state.Chain))
	return nil
}

func cmdBalances(args []string, cfg cliConfig, bcfg blockchain.Config, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("balances", flag.ContinueOnError)
	fs.SetOutput(stderr)
	includePending := fs.Bool("pending", false, "include pending transactions")
	if err := fs.Parse(args); err != nil {
		return err
	}
	state, err := loadValidState(cfg, bcfg)
	if err != nil {
		return err
	}
	var balances blockchain.Balances
	if *includePending {
		balances, err = state.BalancesIncludingPending()
	} else {
		balances, err = state.Balances()
	}
	if err != nil {
		return err
	}
	printBalances(stdout, balances)
	return nil
}

func cmdPending(args []string, cfg cliConfig, bcfg blockchain.Config, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("pending", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	state, err := loadValidState(cfg, bcfg)
	if err != nil {
		return err
	}
	if len(state.Pending) == 0 {
		fmt.Fprintln(stdout, "no pending transactions")
		return nil
	}
	for i, tx := range state.Pending {
		fmt.Fprintf(stdout, "[%d] %s -> %s amount=%d nonce=%d id=%s memo=%q\n", i, tx.From, tx.To, tx.Amount, tx.Nonce, short(tx.ID), tx.Memo)
	}
	return nil
}

func cmdMerkleProof(args []string, cfg cliConfig, bcfg blockchain.Config, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("merkle-proof", flag.ContinueOnError)
	fs.SetOutput(stderr)
	height := fs.Int("height", -1, "block height")
	txIndex := fs.Int("tx", -1, "transaction index inside the block")
	if err := fs.Parse(args); err != nil {
		return err
	}

	state, err := loadValidState(cfg, bcfg)
	if err != nil {
		return err
	}
	if *height < 0 || *height >= len(state.Chain) {
		return fmt.Errorf("height %d out of range", *height)
	}
	block := state.Chain[*height]
	if *txIndex < 0 || *txIndex >= len(block.Transactions) {
		return fmt.Errorf("transaction index %d out of range for block %d", *txIndex, *height)
	}

	tx := block.Transactions[*txIndex]
	txHash := blockchain.TransactionHash(tx)
	proof, err := blockchain.BuildMerkleProof(block.Transactions, *txIndex)
	if err != nil {
		return err
	}

	result := struct {
		BlockHeight      int                          `json:"block_height"`
		TransactionIndex int                          `json:"transaction_index"`
		TransactionID    string                       `json:"transaction_id"`
		TransactionHash  string                       `json:"transaction_hash"`
		MerkleRoot       string                       `json:"merkle_root"`
		Proof            []blockchain.MerkleProofStep `json:"proof"`
		Valid            bool                         `json:"valid"`
	}{
		BlockHeight:      block.Height,
		TransactionIndex: *txIndex,
		TransactionID:    tx.ID,
		TransactionHash:  txHash,
		MerkleRoot:       block.MerkleRoot,
		Proof:            proof,
		Valid:            blockchain.VerifyMerkleProof(txHash, proof, block.MerkleRoot),
	}

	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func cmdServe(args []string, cfg cliConfig, bcfg blockchain.Config, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:8080", "HTTP server address")
	apiToken := fs.String("api-token", "", "optional token required by write API endpoints via X-API-Token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	handler := newAPIServer(cfg.dataPath, bcfg, *apiToken)
	server := &http.Server{Addr: *addr, Handler: handler}
	fmt.Fprintf(stdout, "serving REST API on %s using state %s\n", *addr, cfg.dataPath)
	if *apiToken != "" {
		fmt.Fprintln(stdout, "write endpoints require X-API-Token")
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func cmdTamper(args []string, cfg cliConfig, bcfg blockchain.Config, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("tamper", flag.ContinueOnError)
	fs.SetOutput(stderr)
	height := fs.Int("height", 1, "block height to alter")
	txIndex := fs.Int("tx", 0, "transaction index inside the block")
	amount := fs.Int64("amount", 999999, "new amount to write without re-mining")
	if err := fs.Parse(args); err != nil {
		return err
	}
	state, err := loadValidState(cfg, bcfg)
	if err != nil {
		return err
	}
	if *height < 0 || *height >= len(state.Chain) {
		return fmt.Errorf("height %d out of range", *height)
	}
	if *txIndex < 0 || *txIndex >= len(state.Chain[*height].Transactions) {
		return fmt.Errorf("transaction index %d out of range for block %d", *txIndex, *height)
	}
	before := state.Chain[*height].Transactions[*txIndex].Amount
	state.Chain[*height].Transactions[*txIndex].Amount = *amount
	if err := blockchain.SaveState(cfg.dataPath, state); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "tampered block=%d tx=%d amount %d -> %d; run validate to see detection\n", *height, *txIndex, before, *amount)
	return nil
}

func loadValidState(cfg cliConfig, bcfg blockchain.Config) (blockchain.State, error) {
	state, err := blockchain.LoadState(cfg.dataPath)
	if err != nil {
		return blockchain.State{}, err
	}
	if err := blockchain.ValidateChain(state.Chain, bcfg.Difficulty); err != nil {
		return blockchain.State{}, fmt.Errorf("refusing to operate on invalid chain: %w", err)
	}
	return state, nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, strings.TrimSpace(`toychain - a local proof-of-work toy blockchain

Usage:
  toychain [global flags] <command> [command flags]

Global flags:
  -data string        JSON state path (default "toychain.json")
  -difficulty int     leading zero hex digits, 1..5 (default 3)
  -max-block-tx int   max transactions per mined block (default 5)
  -workers int        mining workers, 0 means NumCPU
  -timeout duration   mining timeout (default 15s)

Commands:
  wallet new -out FILE -passphrase PASS       create encrypted Ed25519 wallet
  wallet show -path FILE                      show wallet address/public key
  init [-force]                               create state file
  faucet -to ADDRESS -amount N                add funding transaction to pending pool
  tx -wallet FILE -passphrase PASS -to ADDRESS -amount N
                                             add signed transfer to pending pool
  tx-sign -wallet FILE -passphrase PASS -to ADDRESS -amount N [-out FILE]
                                             write signed transaction JSON without submitting
  mine                                        mine pending transactions into a block
  print                                       print readable chain
  validate                                    validate chain integrity
  balances [-pending]                         show account balances
  pending                                     list pending transactions
  merkle-proof -height N -tx I                print a transaction Merkle proof
  serve [-addr 127.0.0.1:8080] [-api-token TOKEN]
                                             start REST API server
  tamper -height N -tx I -amount N            deliberately alter stored data for demo`))
}

func printWalletUsage(w io.Writer) {
	fmt.Fprintln(w, strings.TrimSpace(`wallet commands:
  wallet new -out FILE -passphrase PASS
  wallet show -path FILE`))
}

func printBalances(w io.Writer, balances blockchain.Balances) {
	accounts := make([]string, 0, len(balances))
	for account := range balances {
		accounts = append(accounts, account)
	}
	sort.Strings(accounts)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ACCOUNT\tBALANCE")
	for _, account := range accounts {
		fmt.Fprintf(tw, "%s\t%d\n", account, balances[account])
	}
	if err := tw.Flush(); err != nil {
		fmt.Fprintf(w, "failed to flush balance table: %v\n", err)
	}
}

func short(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}
