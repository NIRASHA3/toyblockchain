# Toy Blockchain and Ledger Simulator

A pure Go toy blockchain and ledger simulator developed for coursework. The project started as a single-node proof-of-work blockchain and was extended into a small local network of independent nodes for Assignment 2.

The implementation demonstrates blockchain fundamentals such as blocks, hashes, proof-of-work, signed transactions, wallet-based transfers, Merkle roots, ledger replay, REST APIs, transaction gossip, block gossip, chain synchronisation, cumulative-work fork resolution, and race-free shared state.

---

## Features

### Core Blockchain Features

- Deterministic genesis block
- Proof-of-work mining
- Configurable mining difficulty
- Concurrent mining using Go goroutines
- Difficulty retargeting
- JSON-based chain persistence
- Chain validation by full ledger replay
- Account-based balances
- Pending transaction pool
- Tamper detection
- Fork resolution using a cumulative proof-of-work / heaviest-valid-chain rule

### Wallet and Transaction Features

- Ed25519 wallet generation
- Encrypted wallet storage
- Signed transfers
- Faucet transactions for testing
- Transaction nonce validation
- Replay protection
- Duplicate transaction detection
- Local transaction signing using `tx-sign`

### Merkle Tree Features

- Merkle root stored in every block
- Merkle root recomputation during validation
- Merkle proof generation
- Merkle proof verification

### REST API Features

- Local REST API for single-node operation
- Optional API token protection for write endpoints
- Read endpoints for blocks, balances, transactions, validation, and Merkle proofs

### Networked Node Features

- Multiple independent node processes
- Local HTTP-based peer-to-peer communication
- Configurable peer list
- Transaction gossip with deduplication
- Block gossip with deduplication
- Chain synchronisation for new or lagging nodes
- Network fork resolution and reorganisation using cumulative proof-of-work
- Orphaned transaction return to pending pool after reorg
- Race-safe node state using mutex protection
- Three-node PowerShell cluster launcher

---

## Project Structure

```text
toyblockchain/
├── cmd/
│   └── toychain/
│       ├── main.go
│       ├── api.go
│       └── node_command.go
├── internal/
│   ├── blockchain/
│   │   ├── block.go
│   │   ├── config.go
│   │   ├── fork.go
│   │   ├── ledger.go
│   │   ├── merkle.go
│   │   ├── mining.go
│   │   ├── state.go
│   │   ├── transaction.go
│   │   ├── validate.go
│   │   └── wallet.go
│   └── node/
│       ├── api.go
│       ├── gossip.go
│       ├── node.go
│       ├── reorg.go
│       └── sync.go
├── scripts/
│   ├── start-cluster.ps1
│   └── stop-cluster.ps1
├── README.md
└── go.mod
```

---

## Requirements

- Go installed
- PowerShell for the provided cluster scripts
- GCC/MSYS2 setup is required only when running the Go race detector on Windows

For Windows race tests, use:

```powershell
$env:Path = "C:\msys64\ucrt64\bin;$env:Path"
$env:CGO_ENABLED="1"
```

---

## Build

```powershell
go build -o toychain.exe ./cmd/toychain
```

Run the program:

```powershell
.\toychain.exe help
```

---

## Testing and Verification

Run normal tests:

```powershell
go test ./...
```

Run race detector tests:

```powershell
$env:Path = "C:\msys64\ucrt64\bin;$env:Path"
$env:CGO_ENABLED="1"

go test -race ./...
```

Run vet:

```powershell
go vet ./...
```

Build check:

```powershell
go build -o toychain.exe ./cmd/toychain
Remove-Item toychain.exe -ErrorAction SilentlyContinue
```

Recommended final verification:

```powershell
$env:Path = "C:\msys64\ucrt64\bin;$env:Path"
$env:CGO_ENABLED="1"

go test ./...
go test -race ./...
go vet ./...
go build -o toychain.exe ./cmd/toychain
Remove-Item toychain.exe -ErrorAction SilentlyContinue
```

---

## Basic Single-Node Usage

Build the CLI:

```powershell
go build -o toychain.exe ./cmd/toychain
```

Create a new blockchain state:

```powershell
.\toychain.exe init -force
```

Create wallets:

```powershell
.\toychain.exe wallet new -out alice.wallet.json -passphrase alice-pass
.\toychain.exe wallet new -out bob.wallet.json -passphrase bob-pass
```

Read wallet addresses:

```powershell
$alice = (Get-Content .\alice.wallet.json -Raw | ConvertFrom-Json).address
$bob = (Get-Content .\bob.wallet.json -Raw | ConvertFrom-Json).address
```

Fund Alice:

```powershell
.\toychain.exe faucet -to "$alice" -amount 100
.\toychain.exe mine
```

Send a signed transfer:

```powershell
.\toychain.exe tx -wallet alice.wallet.json -passphrase alice-pass -to "$bob" -amount 25 -memo "payment"
```

Mine the transfer:

```powershell
.\toychain.exe mine
```

Show balances:

```powershell
.\toychain.exe balances
```

Validate the chain:

```powershell
.\toychain.exe validate
```

Print the chain:

```powershell
.\toychain.exe print
```

---

## CLI Commands

```text
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
                                           start single-node REST API server
node -addr 127.0.0.1:8081 -peers URLS       start networked node HTTP service
resolve-fork -candidate FILE [-dry-run]     compare with a competing state and adopt it if it has more cumulative work and is valid
tamper -height N -tx I -amount N            deliberately alter stored data for demo
```

---

## Global Flags

```text
-data string                 JSON state path
-difficulty int              proof-of-work leading-zero hex digits
-max-block-tx int            maximum transactions per block
-workers int                 mining workers; 0 means runtime.NumCPU
-timeout duration            mining timeout
-retarget-interval int       blocks per difficulty adjustment; 0 disables retargeting
-target-block-time duration  target time between blocks for retargeting
```

Example:

```powershell
.\toychain.exe -data node1.json -difficulty 1 -retarget-interval 0 init -force
```

---

## Wallets and Signed Transactions

Wallets use Ed25519 key pairs. The private key is stored in an encrypted wallet file. Transactions are signed locally using the wallet passphrase.

Create a wallet:

```powershell
.\toychain.exe wallet new -out alice.wallet.json -passphrase alice-pass
```

Show wallet metadata:

```powershell
.\toychain.exe wallet show -path alice.wallet.json
```

Create a signed transaction file without submitting it:

```powershell
.\toychain.exe tx-sign -wallet alice.wallet.json -passphrase alice-pass -to "$bob" -amount 25 -memo "network payment" -out tx.json
```

Submit the signed transaction to a networked node:

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8081/transactions -Body (Get-Content .\tx.json -Raw) -ContentType "application/json"
```

---

## Merkle Proofs

Each block stores a Merkle root calculated from the block transactions. Validation recomputes the Merkle root and rejects tampered blocks.

Generate a Merkle proof:

```powershell
.\toychain.exe merkle-proof -height 1 -tx 0
```

The command returns JSON containing:

```text
block_height
transaction_index
transaction_id
transaction_hash
merkle_root
proof
valid
```

---

## Tamper Detection Demo

Create and mine a transaction first, then deliberately modify a stored transaction amount:

```powershell
.\toychain.exe tamper -height 1 -tx 0 -amount 999999
```

Validate the chain:

```powershell
.\toychain.exe validate
```

Expected result:

```text
INVALID
```

Validation fails because the transaction data no longer matches the stored Merkle root and block hash.

---

## Single-Node REST API Mode

Start the REST API:

```powershell
.\toychain.exe serve -addr 127.0.0.1:8080
```

Start with API token protection for write endpoints:

```powershell
.\toychain.exe serve -addr 127.0.0.1:8080 -api-token dev-secret
```

Example read request:

```powershell
Invoke-RestMethod http://127.0.0.1:8080/health
```

Example write request with token:

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8080/mine -Headers @{"X-API-Token"="dev-secret"}
```

---

## Single-Node REST API Endpoints

| Method | Endpoint | Purpose |
|---|---|---|
| `GET` | `/health` | Check API health |
| `GET` | `/chain` | Return full chain state |
| `GET` | `/blocks` | Return blocks |
| `GET` | `/blocks/{height}` | Return block by height |
| `GET` | `/balances` | Return confirmed balances |
| `GET` | `/balances?pending=true` | Return balances including pending transactions |
| `GET` | `/transactions/{id}` | Find transaction by ID |
| `GET` | `/merkle-proof?height=N&tx=I` | Return Merkle proof |
| `GET` | `/validate` | Validate chain |
| `POST` | `/faucet` | Add faucet transaction |
| `POST` | `/transactions` | Submit signed transaction |
| `POST` | `/mine` | Mine pending transactions |

---

## Networked Node Mode

Assignment 2 extends the original single-node blockchain into a small local peer-to-peer network. Each node runs as an independent process with its own JSON state file and configured peer list. Nodes communicate over local HTTP endpoints.

Start one node manually:

```powershell
.\toychain.exe -data node1.json -difficulty 1 -retarget-interval 0 node -addr 127.0.0.1:8081 -peers http://127.0.0.1:8082,http://127.0.0.1:8083
```

Start another node:

```powershell
.\toychain.exe -data node2.json -difficulty 1 -retarget-interval 0 node -addr 127.0.0.1:8082 -peers http://127.0.0.1:8081,http://127.0.0.1:8083
```

Start a third node:

```powershell
.\toychain.exe -data node3.json -difficulty 1 -retarget-interval 0 node -addr 127.0.0.1:8083 -peers http://127.0.0.1:8081,http://127.0.0.1:8082
```

---

## Local Three-Node Cluster

A PowerShell cluster launcher is provided for local testing.

Start a three-node cluster:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start-cluster.ps1 -Reset
```

Expected health check:

```text
Node 1: health=ok, height=0, pending=0, peers=2
Node 2: health=ok, height=0, pending=0, peers=2
Node 3: health=ok, height=0, pending=0, peers=2
```

Stop the cluster:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\stop-cluster.ps1
```

Stop and clean generated cluster state:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\stop-cluster.ps1 -Clean
```

Useful cluster checks:

```powershell
Invoke-RestMethod http://127.0.0.1:8081/status
Invoke-RestMethod http://127.0.0.1:8082/status
Invoke-RestMethod http://127.0.0.1:8083/status

Invoke-RestMethod http://127.0.0.1:8081/peers
Invoke-RestMethod http://127.0.0.1:8081/pending
Invoke-RestMethod http://127.0.0.1:8081/chain
```

---

## Network API Endpoints

| Method | Endpoint | Purpose |
|---|---|---|
| `GET` | `/health` | Check node health |
| `GET` | `/status` | Show current height, head hash, pending count, and peer count |
| `GET` | `/peers` | List configured peers |
| `GET` | `/chain` | Return chain and pending pool |
| `GET` | `/blocks` | Return all blocks |
| `GET` | `/blocks/{height}` | Return a block by height |
| `GET` | `/pending` | Show pending transactions |
| `GET` | `/balances` | Show confirmed balances |
| `GET` | `/balances?pending=true` | Show balances including pending transactions |
| `GET` | `/validate` | Validate the local chain |
| `POST` | `/transactions` | Submit a signed transaction and gossip it |
| `POST` | `/mine` | Mine pending transactions and gossip the new block |
| `POST` | `/sync` | Download missing blocks from a peer |
| `POST` | `/resolve-fork` | Download and adopt a heavier valid peer chain |
| `POST` | `/peer/transactions` | Peer-to-peer transaction gossip endpoint |
| `POST` | `/peer/blocks` | Peer-to-peer block gossip endpoint |
| `GET` | `/peer/status` | Peer status endpoint used by sync and reorg |
| `GET` | `/peer/blocks/{height}` | Peer block download endpoint used by sync and reorg |

---

## Transaction Gossip

When a signed transaction is submitted to one node through `/transactions`, the node:

1. Validates the transaction.
2. Checks for duplicate transaction IDs.
3. Adds the transaction to the pending pool.
4. Records the transaction ID as seen.
5. Forwards the transaction to configured peers.

Peer nodes receive transactions through `/peer/transactions`.

Duplicate transaction IDs are ignored to prevent repeated forwarding and gossip loops.

Example:

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8081/transactions -Body (Get-Content .\tx.json -Raw) -ContentType "application/json"
```

Check pending transactions on each node:

```powershell
Invoke-RestMethod http://127.0.0.1:8081/pending
Invoke-RestMethod http://127.0.0.1:8082/pending
Invoke-RestMethod http://127.0.0.1:8083/pending
```

---

## Block Gossip

When a node mines a block through `/mine`, the node:

1. Selects pending transactions.
2. Mines a valid proof-of-work block.
3. Appends the block locally.
4. Removes confirmed transactions from pending.
5. Broadcasts the block to peers.

Peer nodes receive blocks through `/peer/blocks`.

A receiving node accepts a block only if:

- the block is valid,
- the proof of work is correct,
- the Merkle root is correct,
- the block height is correct,
- the block directly extends the local head.

Mine using the network API:

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8081/mine -ContentType "application/json"
```

Check all node heads:

```powershell
Invoke-RestMethod http://127.0.0.1:8081/status
Invoke-RestMethod http://127.0.0.1:8082/status
Invoke-RestMethod http://127.0.0.1:8083/status
```

---

## Chain Synchronisation

A new or lagging node can call `/sync` to download missing blocks from a peer. Blocks are downloaded one height at a time using peer endpoints and each block is validated before being appended.

Example:

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8082/sync -Body '{"peer":"http://127.0.0.1:8081"}' -ContentType "application/json"
```

Expected response fields:

```text
peer
before_height
peer_height
after_height
head_hash
blocks_downloaded
synced
```

If the node was at height `0` and the peer was at height `2`, a successful sync returns:

```text
before_height = 0
peer_height = 2
after_height = 2
blocks_downloaded = 2
synced = True
```

---

## Fork Resolution and Reorganisation

If a node receives a competing block that does not directly extend its local head, the node can resolve the fork by downloading the peer chain and applying the fork-choice rule.

The node adopts the candidate chain only if:

- the candidate chain is valid,
- the candidate chain has more cumulative proof-of-work than the local chain.

Cumulative work is calculated from block difficulty. In this toy proof-of-work system, each leading hexadecimal zero represents about 16 times more expected mining work, so a block's estimated work is calculated as `16^difficulty`.

During reorganisation:

- transactions from replaced local blocks are treated as orphaned,
- orphaned transactions not present in the adopted chain are revalidated,
- valid orphaned transactions return to the pending pool,
- invalid or conflicting orphaned transactions are dropped.

Manual fork resolution:

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8081/resolve-fork -Body '{"peer":"http://127.0.0.1:8082"}' -ContentType "application/json"
```

Important response fields:

```text
local_height
candidate_height
local_work
candidate_work
before_head_hash
candidate_head_hash
after_height
after_head_hash
decision
adopted
reason
kept_pending
dropped_pending
```

Example successful result:

```text
decision = adopt_candidate
adopted = True
reason = candidate chain has more cumulative proof-of-work and is valid
kept_pending = 1
dropped_pending = 0
```

---

## Concurrency Safety

The networked node wraps blockchain state using a mutex-protected node layer. Shared resources protected by the node layer include:

- chain state,
- pending transaction pool,
- peer list,
- seen transaction IDs,
- seen block hashes.

This is required because HTTP handlers, mining, gossip, sync, and fork resolution can access shared state concurrently.

The project should pass:

```powershell
go test -race ./...
```

---

## Design Notes

### Account Model

This project uses an account-based ledger model. Balances are calculated by replaying all confirmed transactions from the genesis block. There is no UTXO change-output system.

### Genesis Block

The genesis block is deterministic. This makes nodes compatible because all nodes start from the same known genesis hash.

### Proof of Work

Mining searches for a nonce that produces a block hash with the required number of leading zero hexadecimal digits.

For example, difficulty `1` requires a hash beginning with:

```text
0
```

Difficulty `3` requires a hash beginning with:

```text
000
```

### Concurrent Mining

Mining can use multiple goroutines. Each worker searches a different nonce sequence. The first worker to find a valid nonce returns the block result and the remaining workers are cancelled.

### Difficulty Retargeting

Difficulty retargeting can increase or decrease mining difficulty based on block production speed. For Assignment 2 network demonstrations, retargeting is commonly disabled using:

```powershell
-retarget-interval 0
```

This keeps manual multi-node tests predictable.

### Validation

Validation does not trust the saved JSON data. It recomputes and checks:

- canonical genesis block,
- block height,
- previous hash links,
- block hash,
- proof of work,
- Merkle root,
- difficulty rules,
- transaction IDs,
- signatures,
- nonces,
- duplicate transactions,
- balances.

### Mempool

The pending transaction pool stores valid transactions that have not yet been mined. When a block is mined or accepted from a peer, confirmed transactions are removed from pending.

### Gossip Deduplication

Each node tracks seen transaction IDs and seen block hashes. This prevents repeated forwarding between peers.

### Fork Choice

The implemented fork-choice rule uses cumulative proof-of-work instead of simple block count. A candidate chain is adopted only when it is valid and has more cumulative work than the local chain. This is closer to real proof-of-work blockchain behaviour because a shorter chain with higher difficulty can outweigh a longer low-difficulty chain.

---

## Assignment 2 Coverage

The networked implementation covers the main Assignment 2 requirements:

| Requirement | Implementation |
|---|---|
| Multiple independent nodes | `node` command with separate JSON state files |
| HTTP interface and peer list | `node -addr ... -peers ...` |
| Signed transactions | Ed25519 wallets and signed transfers |
| Transaction gossip | `/transactions` and `/peer/transactions` |
| Transaction deduplication | seen transaction ID tracking |
| Block gossip | `/mine` and `/peer/blocks` |
| Block validation before append | `AcceptBlock` and chain validation |
| Chain sync | `/sync`, `/peer/status`, `/peer/blocks/{height}` |
| Fork resolution | `/resolve-fork` and `ResolveForkFromPeer` using cumulative work |
| Reorganisation | adoption of heavier valid peer chain |
| Orphan transaction handling | valid orphaned transactions return to pending |
| Race-free state | mutex-protected `Node` wrapper |
| Introspection API | `/status`, `/peers`, `/pending`, `/balances`, `/chain` |
| Local cluster launcher | `scripts/start-cluster.ps1` and `scripts/stop-cluster.ps1` |
| Race detector testing | `go test -race ./...` |

---

## Known Constraints and Future Improvements

This is an educational toy blockchain, not a production cryptocurrency.

Current constraints:

- peer networking is local HTTP-based and intended for localhost multi-process testing,
- peers are configured manually,
- no automatic peer discovery,
- no NAT traversal,
- no public cross-machine deployment,
- no transaction fees,
- no gas model,
- no smart contracts,
- no EVM,
- no real economic finality,
- no persistent peer database,
- no automatic background sync loop,
- no advanced mempool prioritisation.

Possible future improvements:

- automatic peer discovery,
- periodic peer health checks,
- automatic background sync,
- transaction fees,
- block rewards,
- persistent mempool,
- improved peer banning for invalid data,
- Docker Compose cluster setup,
- smart contract execution layer.

---

## Quick Demonstration Flow

Start a clean three-node cluster:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start-cluster.ps1 -Reset
```

Check nodes:

```powershell
Invoke-RestMethod http://127.0.0.1:8081/status
Invoke-RestMethod http://127.0.0.1:8082/status
Invoke-RestMethod http://127.0.0.1:8083/status
```

Create wallets:

```powershell
.\toychain.exe wallet new -out alice.wallet.json -passphrase alice-pass
.\toychain.exe wallet new -out bob.wallet.json -passphrase bob-pass

$alice = (Get-Content .\alice.wallet.json -Raw | ConvertFrom-Json).address
$bob = (Get-Content .\bob.wallet.json -Raw | ConvertFrom-Json).address
```

Fund Alice on node 1 state before cluster testing, or use a prepared state file for demonstration. For simple API demonstrations, submit a signed transaction to one node and check that it appears on the other nodes.

Submit a signed transaction:

```powershell
.\toychain.exe -data .cluster\node1.json -difficulty 1 -retarget-interval 0 tx-sign -wallet alice.wallet.json -passphrase alice-pass -to "$bob" -amount 25 -memo "demo" -out tx.json

Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8081/transactions -Body (Get-Content .\tx.json -Raw) -ContentType "application/json"
```

Check pending pools:

```powershell
Invoke-RestMethod http://127.0.0.1:8081/pending
Invoke-RestMethod http://127.0.0.1:8082/pending
Invoke-RestMethod http://127.0.0.1:8083/pending
```

Mine on node 1:

```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8081/mine -ContentType "application/json"
```

Check all nodes have the same head:

```powershell
Invoke-RestMethod http://127.0.0.1:8081/status
Invoke-RestMethod http://127.0.0.1:8082/status
Invoke-RestMethod http://127.0.0.1:8083/status
```

Stop cluster:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\stop-cluster.ps1 -Clean
```

---

