# Toy Blockchain and Ledger Simulator

A pure-Go command-line toy blockchain and ledger simulator that demonstrates deterministic block hashing, Merkle-root-based block headers, faucet-funded transactions, wallet-based signed transfers, REST API read and write endpoints, proof-of-work mining, full-chain validation, tamper detection, encrypted wallet storage, JSON persistence, and automated tests.

This project is intentionally local and educational. It does not connect to any external blockchain network, peer node, RPC endpoint, or third-party blockchain SDK.

## Features

- Deterministic canonical genesis block
- SHA-256 block hashing
- Previous-hash block linking
- Per-block stored difficulty
- Merkle root stored in every block
- Transaction hash leaves and deterministic Merkle root calculation
- Merkle proof generation and verification
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
- REST API for chain exploration, faucet funding, signed transaction submission, and mining
- REST API binds to localhost by default and supports optional token protection for write endpoints
- Unit tests for core blockchain, wallet, Merkle, CLI, and API behaviour

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
      server.go
      main_test.go
      cli_invalid_test.go
      server_test.go
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

The automated tests cover deterministic hashing, canonical genesis validation, Merkle root calculation, Merkle-root tamper detection, Merkle proof generation/verification, proof-of-work target checks, signed transaction validation, wallet encryption/decryption, wrong-passphrase rejection, nonce validation, duplicate transaction rejection, invalid amount rejection, overspending rejection, pending-pool overspending rejection, previous-hash-link validation, JSON persistence, CLI error handling, REST API read and write endpoints, and tamper detection.

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

### Export a Signed Transaction JSON File

The `tx-sign` command signs a transaction locally but does not submit it to the pending pool. This is useful for the REST API, because the server should receive already-signed transactions instead of wallet passphrases.

```powershell
.\toychain.exe -data demo.json -difficulty 3 tx-sign -wallet alice.wallet.json -passphrase alice-pass -to BOB_ADDRESS -amount 40 -out signed_tx.json
```

The output file contains a complete signed transaction with sender address, recipient address, amount, nonce, public key, signature, and deterministic transaction ID.

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

### Generate a Merkle Proof

```powershell
.\toychain.exe -data demo.json -difficulty 3 merkle-proof -height 2 -tx 0
```

The command prints JSON containing the block height, transaction index, transaction ID, transaction hash, Merkle root, sibling proof path, and a `valid` boolean produced by local proof verification.

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
.\toychain.exe -data demo.json -difficulty 3 merkle-proof -height 2 -tx 0
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

## REST API

The project includes a local HTTP API using Go's standard `net/http` package. The API works like a small blockchain explorer and local node API. It can read chain data, validate the chain, accept faucet transactions, accept already-signed transfer transactions, and mine pending transactions.

Important security design: the API does **not** receive wallet passphrases, private keys, or wallet file paths. Wallets are unlocked only by the CLI/client. The API receives already-signed transactions and verifies them before adding them to the pending pool.

Start the server. By default, the API should be bound to localhost only:

```powershell
.\toychain.exe -data demo.json -difficulty 3 serve
```

You can also provide the address explicitly:

```powershell
.\toychain.exe -data demo.json -difficulty 3 serve -addr 127.0.0.1:8080
```

Optional write-endpoint protection can be enabled with `-api-token`. When a token is configured, `POST /faucet`, `POST /transactions`, and `POST /mine` require the `X-API-Token` header. Read endpoints remain open.

```powershell
.\toychain.exe -data demo.json -difficulty 3 serve -addr 127.0.0.1:8080 -api-token dev-secret
```

Read endpoints:

| Method | Endpoint | Purpose |
|---|---|---|
| `GET` | `/health` | Check server status |
| `GET` | `/chain` | Return chain metadata, blocks, and pending transactions |
| `GET` | `/blocks` | Return all blocks |
| `GET` | `/blocks/{height}` | Return one block by height |
| `GET` | `/balances` | Return confirmed balances |
| `GET` | `/balances?pending=true` | Return balances including pending transactions |
| `GET` | `/transactions/{id}` | Find a confirmed or pending transaction by ID |
| `GET` | `/merkle-proof?height=2&tx=0` | Generate and verify a Merkle proof |
| `GET` | `/validate` | Validate the chain and return JSON result |

Write endpoints:

| Method | Endpoint | Purpose |
|---|---|---|
| `POST` | `/faucet` | Add a faucet funding transaction to the pending pool |
| `POST` | `/transactions` | Submit an already-signed transfer transaction to the pending pool |
| `POST` | `/mine` | Mine pending transactions into a new block |

PowerShell read examples:

```powershell
Invoke-RestMethod http://127.0.0.1:8080/health
Invoke-RestMethod http://127.0.0.1:8080/blocks/2
Invoke-RestMethod http://127.0.0.1:8080/balances
Invoke-RestMethod "http://127.0.0.1:8080/merkle-proof?height=2&tx=0"
Invoke-RestMethod http://127.0.0.1:8080/validate
```

If the server was started with `-api-token`, prepare headers before calling write endpoints:

```powershell
$headers = @{ "X-API-Token" = "dev-secret" }
```

If no API token was configured, omit the `-Headers $headers` part from the write examples.

### Faucet API Example

If you are running the API examples in a new PowerShell terminal, load the wallet addresses again first:

```powershell
$alice = (Get-Content .\alice.wallet.json -Raw | ConvertFrom-Json).address
$bob = (Get-Content .\bob.wallet.json -Raw | ConvertFrom-Json).address

Write-Host "Alice = $alice"
Write-Host "Bob = $bob"
```

Then submit a faucet transaction and mine it:

```powershell
$faucetBody = @{ to = $alice; amount = 100; memo = "api faucet" } | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/faucet -Headers $headers -Body $faucetBody -ContentType "application/json"
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/mine -Headers $headers -Body '{}' -ContentType "application/json"
```

### Signed Transaction API Example

First, sign the transaction locally using the CLI. This keeps the wallet passphrase on the client side:

```powershell
.\toychain.exe -data demo.json -difficulty 3 tx-sign -wallet alice.wallet.json -passphrase alice-pass -to $bob -amount 40 -out signed_tx.json
```

Then submit the already-signed transaction to the API and mine it:

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/transactions -Headers $headers -Body (Get-Content signed_tx.json -Raw) -ContentType "application/json"
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/mine -Headers $headers -Body '{}' -ContentType "application/json"
```

This design is closer to a standard blockchain node model: clients sign transactions locally, while the node/API verifies signatures, nonce sequence, duplicate transaction IDs, balances, and chain validity.

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

### Merkle Proofs

A Merkle proof is a small list of sibling hashes that proves a transaction belongs to a block's Merkle root. The verifier starts from the transaction hash and combines it with each sibling hash in the correct left/right order until a root is reconstructed. If the reconstructed root equals the block's stored Merkle root, the transaction is included in that block.

The CLI command `merkle-proof` demonstrates this by building a proof for a selected block height and transaction index, then verifying it locally before printing the JSON result.

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

### REST API Design

The REST API loads the JSON state file, validates the chain for normal endpoints, and returns structured JSON responses. Invalid paths, unsupported methods, invalid block heights, invalid transaction IDs, invalid Merkle proof indexes, malformed request bodies, unsigned transactions, duplicate transactions, nonce errors, and insufficient balances return structured JSON error responses.

The write API is intentionally designed without wallet passphrases. `POST /transactions` accepts an already-signed transaction JSON object. The server verifies the transaction ID, signature, sender public key, nonce, duplicate transaction ID, and ledger rules before saving it to the pending pool. This keeps private key handling on the client side and is closer to a standard blockchain node model.

For safer local testing, the server binds to `127.0.0.1:8080` by default instead of listening on all network interfaces. The optional `-api-token` flag protects state-changing endpoints with the `X-API-Token` header. This is not a replacement for full production API security, but it prevents accidental unauthenticated writes during local development.

## Known Constraints and Future Improvements

This is a local educational blockchain simulator, not production money software.

Current constraints:

- no peer-to-peer network
- no distributed consensus
- no fork choice rule
- no real finality
- no transaction fees
- no smart contracts
- wallet passphrases are supplied through CLI flags, which can be exposed in shell history
- the standard-library-only KDF is educational and weaker than Argon2id/scrypt

Useful future improvements:

1. Use interactive hidden passphrase input.
2. Replace the educational KDF with Argon2id or scrypt.
3. Add HTTPS support, stronger authentication, rate limiting, and role-based API access.
4. Add peer-to-peer node communication.
5. Add proof-of-authority or fork-resolution logic.
6. Add difficulty retargeting.

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