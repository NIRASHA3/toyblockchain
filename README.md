# Toy Blockchain and Ledger Simulator

A pure-Go command-line toy blockchain and ledger simulator that demonstrates deterministic block hashing, Merkle-root-based block headers, faucet-funded transactions, wallet-based signed transfers, proof-of-work mining, full-chain validation, tamper detection, encrypted wallet storage, JSON persistence, and automated tests.

This project is intentionally local and educational. It does not connect to any external blockchain network, peer node, RPC endpoint, or third-party blockchain SDK.

## Features

- Deterministic canonical genesis block
- SHA-256 block hashing
- Previous-hash block linking
- Per-block stored difficulty
- Merkle root stored in every block
- Transaction hash leaves and deterministic Merkle root calculation
- Faucet-funded account model
- Encrypted Ed25519 wallet generation
- Public-key-derived wallet addresses
- Signed transfer transactions
- Transaction nonce validation and replay protection
- Duplicate transaction ID detection
- Pending transaction pool
- Proof-of-work mining with configurable difficulty
- Concurrent mining workers
- Full-chain validation
- Tamper detection
- Balance calculation by replaying the chain
- Balance overflow protection
- JSON file persistence
- Command-line interface
- Unit tests for core blockchain and wallet behaviour

## Requirements

- Go 1.22 or newer

Check your Go version:

```bash
go version
```

The project uses only the Go standard library.

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
      merkle.go
      merkle_test.go
      mining.go
      state.go
      storage.go
      transaction.go
      validation.go
      wallet.go
      wallet_test.go
      test_helpers_test.go
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

On Windows PowerShell:

```powershell
go build -o toychain.exe ./cmd/toychain
```

## Verification

The project was verified with:

```powershell
go test ./...
go vet ./...
go build -o toychain.exe ./cmd/toychain
```

The automated tests cover deterministic hashing, canonical genesis validation, Merkle root calculation, Merkle-root tamper detection, proof-of-work target checks, signed transaction validation, wallet encryption/decryption, wrong-passphrase rejection, nonce validation, duplicate transaction rejection, invalid amount rejection, overspending rejection, pending-pool overspending rejection, previous-hash-link validation, JSON persistence, CLI error handling, and tamper detection.

## Command-Line Usage

General format:

```bash
./toychain -data chain.json -difficulty 3 COMMAND
```

On Windows PowerShell:

```powershell
.\toychain.exe -data chain.json -difficulty 3 COMMAND
```

Common flags:

| Flag | Description | Default |
|---|---|---|
| `-data` | JSON file used to save and load blockchain state | `toychain.json` |
| `-difficulty` | Number of leading zero hex digits required in mined block hash | `3` |
| `-max-block-tx` | Maximum number of transactions included in one mined block | `5` |
| `-workers` | Number of mining workers. If `0`, it uses available CPU count | `0` |
| `-timeout` | Mining timeout duration | `15s` |

## Wallet Commands

### Create an Encrypted Wallet

```powershell
.\toychain.exe wallet new -out alice.wallet.json -passphrase alice-pass
.\toychain.exe wallet new -out bob.wallet.json -passphrase bob-pass
```

Each wallet contains an Ed25519 public/private key pair. The private key is encrypted using AES-256-GCM with a standard-library-only passphrase-derived key.

### Show Wallet Address

```powershell
.\toychain.exe wallet show -path alice.wallet.json
.\toychain.exe wallet show -path bob.wallet.json
```

The output shows the wallet address and public key. The private key is not printed.

## Blockchain Commands

### Initialise a Chain

```powershell
.\toychain.exe -data demo.json -difficulty 3 init -force
```

### Add Faucet Funds

Use the address from `wallet show`:

```powershell
.\toychain.exe -data demo.json -difficulty 3 faucet -to ALICE_ADDRESS -amount 100
```

### View Pending Transactions

```powershell
.\toychain.exe -data demo.json -difficulty 3 pending
```

### Mine Pending Transactions

```powershell
.\toychain.exe -data demo.json -difficulty 3 mine
```

### Add a Signed Transfer Transaction

```powershell
.\toychain.exe -data demo.json -difficulty 3 tx -wallet alice.wallet.json -passphrase alice-pass -to BOB_ADDRESS -amount 40
```

The sender is derived from the wallet address. The transaction is signed using the decrypted private key. The chain validates the signature using the public key stored in the transaction.

### Show Balances

```powershell
.\toychain.exe -data demo.json -difficulty 3 balances
```

Balances are derived by replaying the confirmed chain from genesis to the latest block. Balances are displayed by wallet address because the blockchain identity is the public-key-derived address, not a local human name.

### Validate the Chain

```powershell
.\toychain.exe -data demo.json -difficulty 3 validate
```

Validation checks block structure, canonical genesis, stored hashes, recomputed hashes, Merkle roots, proof-of-work, previous-hash links, timestamps, transaction IDs, signatures, nonces, duplicate transaction IDs, sender balances, and overflow rules.

### Print the Chain

```powershell
.\toychain.exe -data demo.json -difficulty 3 print
```

Printed blocks include height, timestamp, difficulty, previous hash, Merkle root, nonce, block hash, and transaction count.

### Tamper with a Transaction

```powershell
.\toychain.exe -data demo.json tamper -height 1 -tx 0 -amount 999
```

This deliberately changes a transaction amount without recalculating the Merkle root or re-mining. Running validation after this should fail.

## End-to-End Example

Windows PowerShell example:

```powershell
Remove-Item demo.json -ErrorAction SilentlyContinue
Remove-Item alice.wallet.json -ErrorAction SilentlyContinue
Remove-Item bob.wallet.json -ErrorAction SilentlyContinue

.\toychain.exe wallet new -out alice.wallet.json -passphrase alice-pass
.\toychain.exe wallet new -out bob.wallet.json -passphrase bob-pass

.\toychain.exe wallet show -path alice.wallet.json
.\toychain.exe wallet show -path bob.wallet.json
```

Copy the two addresses, then run:

```powershell
.\toychain.exe -data demo.json -difficulty 3 init -force
.\toychain.exe -data demo.json -difficulty 3 faucet -to ALICE_ADDRESS -amount 100
.\toychain.exe -data demo.json -difficulty 3 mine
.\toychain.exe -data demo.json -difficulty 3 tx -wallet alice.wallet.json -passphrase alice-pass -to BOB_ADDRESS -amount 40
.\toychain.exe -data demo.json -difficulty 3 mine
.\toychain.exe -data demo.json -difficulty 3 balances
.\toychain.exe -data demo.json -difficulty 3 validate
.\toychain.exe -data demo.json -difficulty 3 print
```

Expected final balances:

```text
ALICE_ADDRESS    60
BOB_ADDRESS      40
```

Expected validation result:

```text
VALID: 3 blocks checked
```

## Invalid Transaction Examples

Zero amount is rejected:

```powershell
.\toychain.exe -data demo.json -difficulty 3 tx -wallet alice.wallet.json -passphrase alice-pass -to BOB_ADDRESS -amount 0
```

Negative amount is rejected:

```powershell
.\toychain.exe -data demo.json -difficulty 3 tx -wallet alice.wallet.json -passphrase alice-pass -to BOB_ADDRESS -amount -10
```

Overspending is rejected:

```powershell
.\toychain.exe -data demo.json -difficulty 3 tx -wallet alice.wallet.json -passphrase alice-pass -to BOB_ADDRESS -amount 150
```

Wrong wallet passphrase is rejected:

```powershell
.\toychain.exe -data demo.json -difficulty 3 tx -wallet alice.wallet.json -passphrase wrong-pass -to BOB_ADDRESS -amount 10
```

## JSON Persistence

The blockchain state is saved to the JSON file passed through the `-data` flag. The encrypted wallets are saved separately as wallet JSON files.

A clean genesis-only chain contains:

```json
{
  "chain": [
    {
      "height": 0,
      "timestamp": 0,
      "difficulty": 5,
      "transactions": [],
      "merkle_root": "0000000000000000000000000000000000000000000000000000000000000000",
      "prev_hash": "0000000000000000000000000000000000000000000000000000000000000000",
      "nonce": 2417102,
      "hash": "0000050c5cad3e6cb229bb04eacc3c580834a93285f16f6dece119029021fcfd"
    }
  ],
  "pending": []
}
```

## Design Notes

### Deterministic Hashing and Merkle Root

A block hash is computed using SHA-256 over a stable canonical block-header payload. The block's own `Hash` field is excluded from the hash calculation.

The block hash input includes:

1. block height,
2. Unix timestamp,
3. difficulty,
4. previous block hash,
5. Merkle root,
6. nonce.

The Merkle root commits to the full transaction list. Each transaction is hashed into a leaf using transaction ID, sender, recipient, amount, creation timestamp, memo, transaction nonce, public key, and signature. Leaf hashes are paired and hashed upward until one root remains. If a level has an odd number of hashes, the final hash is duplicated for that level.

Changing any transaction changes its transaction hash, which changes the Merkle root, which then invalidates the block hash unless the block is rebuilt and re-mined.

### Wallets and Signatures

Each wallet uses an Ed25519 public/private key pair. The account address is derived from the public key. A transfer transaction contains the sender address, recipient address, amount, nonce, public key, and signature.

During validation:

1. the public key is decoded,
2. the sender address is recalculated from the public key,
3. the signature is verified against the transaction signing payload,
4. the nonce is checked against the expected sender nonce,
5. the ledger rules are applied.

This prevents a user from creating a transaction from someone else's address without the correct private key.

### Encrypted Wallet Storage

Wallet files store public information such as address and public key in plaintext, but the private key is encrypted. The project uses AES-256-GCM from the Go standard library and a standard-library-only iterative SHA-256 key derivation function.

For production wallet software, a memory-hard KDF such as Argon2id or scrypt would be stronger. This project keeps the implementation standard-library-only for learning and assessment compatibility.

### Ledger Model

Balances are not stored as the source of truth. They are calculated by replaying confirmed transactions from the chain. Pending transactions are also replayed when checking whether a new pending transaction is valid.

### Validation

Validation fails fast and reports the first offending block. It checks canonical genesis, height sequence, stored Merkle root, stored hash, recomputed hash, proof-of-work, previous-hash links, timestamps, transaction syntax, transaction IDs, signatures, nonces, duplicate IDs, sufficient balances, and overflow rules.

## Known Constraints and Future Improvements

This is a local educational blockchain simulator, not production money software.

Current constraints:

- no peer-to-peer network
- no distributed consensus
- no Merkle proof command yet
- no fork choice rule
- no real finality
- no transaction fees
- no smart contracts
- wallet passphrases are supplied through CLI flags, which can be exposed in shell history
- the standard-library-only KDF is educational and weaker than Argon2id/scrypt

Useful future improvements:

1. Use interactive hidden passphrase input.
2. Replace the educational KDF with Argon2id or scrypt.
3. Add Merkle proof generation and verification.
4. Add a REST API for block and transaction lookup.
5. Add peer-to-peer node communication.
6. Add proof-of-authority or fork-resolution logic.
7. Add difficulty retargeting.

## Research Report

The research report is available at:

```text
reports/research_report.md
```

## Quick Final Check

Before submission, run:

```powershell
go test ./...
go vet ./...
go build -o toychain.exe ./cmd/toychain
```
