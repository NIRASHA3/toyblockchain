# Toy Blockchain and Ledger Simulator

A pure-Go command-line toy blockchain and ledger simulator that demonstrates deterministic block hashing, faucet-funded transactions, proof-of-work mining, full-chain validation, tamper detection, JSON persistence, and automated tests.

This project is intentionally small and local. It does not connect to any external blockchain network, peer node, wallet, RPC endpoint, or third-party blockchain SDK.

## Features

- Deterministic genesis block
- SHA-256 block hashing
- Previous-hash block linking
- Faucet-funded account model
- Transfer transactions
- Pending transaction pool
- Proof-of-work mining with configurable difficulty
- Concurrent mining workers
- Full-chain validation
- Tamper detection
- Balance calculation by replaying the chain
- JSON file persistence
- Command-line interface
- Unit tests for core blockchain behaviour

## Requirements

- Go 1.22 or newer

Check your Go version:

```bash
go version
```

## Project Structure

```text
toyblockchain/
  cmd/
    toychain/
      main.go
      main_test.go
      cli_invalid_test.go
  internal/
    blockchain/
      block.go
      blockchain_test.go
      chain_integrity_test.go
      config.go
      errors.go
      invalid_transaction_test.go
      ledger.go
      mining.go
      state.go
      storage.go
      transaction.go
      validation.go
  reports/
    research_report.md
  go.mod
  README.md
```

## Build and Test

Run all unit tests:

```bash
go test ./...
```

Run Go vet:

```bash
go vet ./...
```

Build the CLI:

```bash
go build -o toychain ./cmd/toychain
```

On Windows PowerShell, build with:

```powershell
go build -o toychain.exe ./cmd/toychain
```

## Verification

The project was verified with:

```bash
go test ./...
go vet ./...
go build -o toychain ./cmd/toychain
```

On Windows PowerShell:

```powershell
go test ./...
go vet ./...
go build -o toychain.exe ./cmd/toychain
```

The automated tests cover:

- deterministic genesis block creation
- deterministic block hashing
- proof-of-work target checks
- honest-chain validation
- tamper detection
- zero amount rejection
- negative amount rejection
- overspending rejection
- pending-pool overspending rejection
- transaction ID validation
- previous-hash-link validation
- JSON persistence
- CLI error handling

## Command-Line Usage

General format:

```bash
./toychain -data chain.json -difficulty 3 <command>
```

On Windows PowerShell:

```powershell
.\toychain.exe -data chain.json -difficulty 3 <command>
```

Common flags:

| Flag | Description | Default |
|---|---|---|
| `-data` | JSON file used to save and load blockchain state | `chain.json` |
| `-difficulty` | Number of leading zero hex digits required in mined block hash | `3` |
| `-max-block-tx` | Maximum number of transactions included in one mined block | `5` |
| `-workers` | Number of mining workers. If `0`, it uses available CPU count | `0` |
| `-mine-timeout` | Mining timeout duration | `15s` |

## Commands

### Initialise a Chain

```bash
./toychain -data demo.json -difficulty 3 init -force
```

Windows PowerShell:

```powershell
.\toychain.exe -data demo.json -difficulty 3 init -force
```

This creates a new blockchain file with only the deterministic genesis block.

### Add Faucet Funds

```bash
./toychain -data demo.json -difficulty 3 faucet -to alice -amount 100
```

Windows PowerShell:

```powershell
.\toychain.exe -data demo.json -difficulty 3 faucet -to alice -amount 100
```

This adds a pending faucet transaction that gives Alice 100 units.

### View Pending Transactions

```bash
./toychain -data demo.json -difficulty 3 pending
```

Windows PowerShell:

```powershell
.\toychain.exe -data demo.json -difficulty 3 pending
```

### Mine Pending Transactions

```bash
./toychain -data demo.json -difficulty 3 mine
```

Windows PowerShell:

```powershell
.\toychain.exe -data demo.json -difficulty 3 mine
```

Mining searches for a nonce so that the block hash satisfies the configured difficulty target.

### Add a Transfer Transaction

```bash
./toychain -data demo.json -difficulty 3 tx -from alice -to bob -amount 40
```

Windows PowerShell:

```powershell
.\toychain.exe -data demo.json -difficulty 3 tx -from alice -to bob -amount 40
```

The transaction is rejected if the sender does not have enough confirmed and available balance.

### Show Balances

```bash
./toychain -data demo.json -difficulty 3 balances
```

Windows PowerShell:

```powershell
.\toychain.exe -data demo.json -difficulty 3 balances
```

Balances are derived by replaying the confirmed chain from genesis to the latest block.

### Validate the Chain

```bash
./toychain -data demo.json -difficulty 3 validate
```

Windows PowerShell:

```powershell
.\toychain.exe -data demo.json -difficulty 3 validate
```

Validation checks:

- block height sequence
- stored hash equals recomputed hash
- proof-of-work target
- genesis previous hash
- previous-hash links
- timestamp ordering
- transaction syntax
- transaction ID correctness
- sufficient sender balances

### Print the Chain

```bash
./toychain -data demo.json -difficulty 3 print
```

Windows PowerShell:

```powershell
.\toychain.exe -data demo.json -difficulty 3 print
```

### Tamper with a Transaction

```bash
./toychain -data demo.json tamper -height 1 -tx 0 -amount 999
```

Windows PowerShell:

```powershell
.\toychain.exe -data demo.json tamper -height 1 -tx 0 -amount 999
```

This deliberately changes a transaction amount without re-mining. Running validation after this should fail.

## End-to-End Example

Windows PowerShell example:

```powershell
Remove-Item e2e_chain.json -ErrorAction SilentlyContinue

.\toychain.exe -data e2e_chain.json -difficulty 3 init -force
.\toychain.exe -data e2e_chain.json -difficulty 3 faucet -to alice -amount 100
.\toychain.exe -data e2e_chain.json -difficulty 3 pending
.\toychain.exe -data e2e_chain.json -difficulty 3 mine
.\toychain.exe -data e2e_chain.json -difficulty 3 balances

.\toychain.exe -data e2e_chain.json -difficulty 3 tx -from alice -to bob -amount 40
.\toychain.exe -data e2e_chain.json -difficulty 3 pending
.\toychain.exe -data e2e_chain.json -difficulty 3 mine
.\toychain.exe -data e2e_chain.json -difficulty 3 balances
.\toychain.exe -data e2e_chain.json -difficulty 3 validate
.\toychain.exe -data e2e_chain.json -difficulty 3 print
```

Expected final balances:

```text
ACCOUNT  BALANCE
alice    60
bob      40
```

Expected validation result:

```text
VALID: 3 blocks checked at difficulty 3
```

## Invalid Transaction Examples

Zero amount is rejected:

```powershell
.\toychain.exe -data e2e_chain.json -difficulty 3 tx -from alice -to bob -amount 0
```

Negative amount is rejected:

```powershell
.\toychain.exe -data e2e_chain.json -difficulty 3 tx -from alice -to bob -amount -10
```

Overspending is rejected:

```powershell
.\toychain.exe -data e2e_chain.json -difficulty 3 tx -from alice -to bob -amount 150
```

## Tamper Detection Example

Create a copy of a valid chain:

```powershell
Copy-Item e2e_chain.json tamper_e2e.json -Force
```

Validate before tampering:

```powershell
.\toychain.exe -data tamper_e2e.json -difficulty 3 validate
```

Tamper with block 1:

```powershell
.\toychain.exe -data tamper_e2e.json tamper -height 1 -tx 0 -amount 999
```

Validate again:

```powershell
.\toychain.exe -data tamper_e2e.json -difficulty 3 validate
```

Expected result:

```text
INVALID: block 1 failed hash check: stored hash does not match recomputed hash
```

## JSON Persistence

The blockchain state is saved to the JSON file passed through the `-data` flag.

Example:

```powershell
.\toychain.exe -data demo.json -difficulty 3 init -force
.\toychain.exe -data demo.json -difficulty 3 faucet -to alice -amount 100
.\toychain.exe -data demo.json -difficulty 3 mine
```

The file `demo.json` will contain the chain and pending transaction pool:

```json
{
  "chain": [
    {
      "height": 0,
      "timestamp": 0,
      "transactions": [],
      "prev_hash": "0000000000000000000000000000000000000000000000000000000000000000",
      "nonce": 47296,
      "hash": "00000ced59ade982305577d2c37a075a180e0b1e0c86566febff9bd2f4320a49"
    }
  ],
  "pending": []
}
```

## Design Notes

### Deterministic Hashing

A block hash is computed using SHA-256 over a stable canonical payload. The block’s own `Hash` field is excluded from the hash calculation.

The hash input includes:

1. block height
2. Unix timestamp
3. previous block hash
4. nonce
5. transaction count
6. each transaction in order

For each transaction, the hash input includes:

1. transaction index
2. transaction ID
3. sender
4. recipient
5. amount
6. creation timestamp
7. memo

String values are length-prefixed before writing to the hash payload. This prevents ambiguous concatenation.

### Previous-Hash Linking

Each non-genesis block stores the previous block’s hash in its `PrevHash` field. During validation, the program checks that:

```text
currentBlock.PrevHash == previousBlock.Hash
```

This creates the tamper-evident chain structure.

### Ledger Model

Balances are not stored as the source of truth. They are calculated by replaying confirmed transactions from the chain.

The special `FAUCET` account introduces initial funds. Normal transfer transactions must have:

- non-empty sender
- non-empty recipient
- positive amount
- correct transaction ID
- sufficient sender balance

### Mining

Mining searches for a nonce that makes the block hash start with the required number of leading zero hexadecimal digits.

For example, with difficulty `3`, a valid block hash must begin with:

```text
000
```

Mining uses goroutines to split the nonce space across workers. When one worker finds a valid nonce, the context is cancelled and the remaining workers stop.

### Validation

Validation fails fast and reports the first offending block.

It checks:

- chain is not empty
- height sequence is correct
- block hash matches recomputed hash
- block hash satisfies proof-of-work difficulty
- genesis block has the fixed previous hash
- previous-hash links are correct
- timestamps do not move backwards
- transaction structure is valid
- transaction IDs match their contents
- sender balances are sufficient

## Known Constraints and Future Improvements

This is a local educational blockchain simulator, not production money software.

Current constraints:

- no peer-to-peer network
- no distributed consensus
- no transaction signatures
- no wallet/key management
- no Merkle tree
- no fork choice rule
- no real finality
- no transaction fees
- no smart contracts

Useful future improvements:

1. Add digital signatures so only the owner of an account can spend funds.
2. Add public/private key wallet generation.
3. Add Merkle roots and Merkle proof verification.
4. Add a REST API for block and transaction lookup.
5. Add peer-to-peer node communication.
6. Add proof-of-authority or fork-resolution logic.
7. Add difficulty retargeting.

## Research Report

The research report is available at:

```text
reports/research_report.md
```

It covers:

- problem analysis
- architecture
- hashing scheme
- validation strategy
- Go feature choices
- tamper-evidence experiment
- difficulty-versus-effort experiment
- blockchain design discussion
- constraints and future improvements

