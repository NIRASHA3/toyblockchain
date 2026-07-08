# Toy Blockchain and Ledger Simulator

A pure-Go command-line toy blockchain built for the Golang Developer Assessment. It uses only the Go standard library and implements deterministic block hashing, a faucet-funded ledger, proof-of-work mining, validation, JSON persistence, and unit tests.

## Requirements

- Go 1.22 or newer
- No third-party dependencies

## Build and test

```bash
go test ./...
go vet ./...
go build -o toychain ./cmd/toychain
```

## Quick start

```bash
# Create a chain file
./toychain -data demo.json -difficulty 3 init -force

# Add initial funds to Alice. This goes to the pending pool.
./toychain -data demo.json -difficulty 3 faucet -to alice -amount 100

# Mine the pending faucet transaction into a block.
./toychain -data demo.json -difficulty 3 mine

# Add and mine a transfer.
./toychain -data demo.json -difficulty 3 tx -from alice -to bob -amount 40
./toychain -data demo.json -difficulty 3 mine

# Inspect state.
./toychain -data demo.json -difficulty 3 print
./toychain -data demo.json -difficulty 3 balances
./toychain -data demo.json -difficulty 3 validate
```

## CLI reference

Global flags:

```text
-data string          JSON state path, default toychain.json
-difficulty int       proof-of-work leading-zero hex digits, 0..5, default 3
-max-block-tx int     maximum transactions per block, default 5
-workers int          mining workers; 0 means runtime.NumCPU
-timeout duration     mining timeout, default 15s
```

Commands:

```text
init [-force]                         create state file
faucet -to ACCOUNT -amount N          add funding transaction to pending pool
tx -from A -to B -amount N            add transfer transaction to pending pool
mine                                  mine pending transactions into a block
print                                 print readable chain
validate                              validate chain integrity
balances [-pending]                   show account balances
pending                               list pending transactions
tamper -height N -tx I -amount N      deliberately alter stored data for demo
```

## Design decisions

- **Pure Go:** The implementation uses only the standard library: `crypto/sha256`, `encoding/json`, `context`, `flag`, `sync`, and related packages.
- **Deterministic genesis:** The genesis block has height `0`, timestamp `0`, previous hash of 64 zeroes, and a precomputed nonce/hash that satisfies all supported difficulties up to 5.
- **Stable hashing:** A block hash is SHA-256 over a canonical byte payload. The payload includes height, timestamp, previous hash, nonce, transaction count, and each transaction field in order. The block's own `Hash` field is excluded.
- **Ledger from replay:** Balances are never stored as authoritative state. They are derived by replaying confirmed transactions from genesis to the chain tip.
- **Faucet funding:** `FAUCET` is a special sender account used to introduce initial funds. Normal accounts cannot overspend.
- **Concurrent mining:** Mining splits nonce search across goroutines and cancels workers using `context` when a result is found or a timeout expires.
- **Persistence:** The chain and pending pool are saved as JSON. Writes use a temporary file followed by rename to reduce partial-write risk.

## Known limitations

This is intentionally a single-process local simulator. It does not implement networking, distributed consensus, transaction signatures, Merkle roots, wallets, fork choice, mempool gossip, or real finality. Difficulty is capped at 5 to keep mining practical during review.
