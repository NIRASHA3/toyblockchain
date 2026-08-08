# Research Report: Toy Blockchain and Ledger Simulator
## 1. Project scope and problem analysis
This project implements a local command-line blockchain and ledger simulator in pure Go. The system demonstrates how blockchain data structures, proof-of-work mining, signed transactions, Merkle-root-based transaction commitment, ledger replay, tamper detection, difficulty retargeting, HTTP APIs, peer communication, gossip propagation, chain synchronisation, and fork resolution work together inside a small blockchain network.
The implementation runs without third-party blockchain frameworks. Each node stores its own JSON state file and exposes an HTTP interface. In networked mode, multiple local node processes are started on different ports and connected through configured peer URLs. This allows transactions and blocks to propagate between nodes, allows lagging nodes to synchronise from peers, and allows nodes to resolve forks by adopting a longer valid chain.
The main problem addressed by this project is how a blockchain node can maintain a consistent local view of the ledger while receiving transactions, mined blocks, and competing chain data from other nodes. The implementation focuses on correctness and observability rather than production-level scalability. Each critical state transition is validated by replaying the chain and checking block structure, proof-of-work, Merkle roots, digital signatures, sender nonces, duplicate transaction IDs, and account balances.
## 2. Research background
### 2.1 Proof-of-work and longest-chain reasoning
Nakamoto's Bitcoin design introduced a peer-to-peer electronic cash system based on proof-of-work, block linking, and a longest proof-of-work chain rule. The key security idea is that rewriting past history requires redoing the proof-of-work for the changed block and then catching up with the honest network. This project follows the same educational principle in simplified form: a block is accepted only if its hash satisfies the configured proof-of-work target, and a competing chain is adopted only if it is valid and longer than the local chain.
The simulator uses block count as its fork-choice metric rather than cumulative work. This is simpler for demonstration and easier to test, but it is less accurate than a production proof-of-work system where the chain with the most accumulated work is normally preferred.
### 2.2 Gossip propagation and temporary forks
Blockchain networks rely on propagation of transactions and blocks between peers. Research on Bitcoin propagation shows that network delay can cause nodes to temporarily observe different heads. When two valid blocks are mined close together, different peers may accept different blocks first, causing a temporary fork. A fork-choice rule is then required so nodes can eventually converge when one branch becomes preferable.
This simulator reproduces the same concept locally. A node broadcasts valid transactions and mined blocks to configured peers. If a peer receives a block that directly extends its current head, it validates and appends it. If the block does not extend the local head, the peer treats it as fork evidence and can download the peer chain for fork resolution.
### 2.3 Blockchain challenges and opportunities
Blockchain systems combine cryptography, consensus, distributed systems, and application design. Survey literature identifies important challenges such as consensus design, scalability, privacy, and security. This simulator intentionally focuses on a small subset of those problems: transaction authenticity, tamper-evident block structure, local peer communication, chain synchronisation, and fork handling. It does not attempt to implement public blockchain economics, smart contracts, validator incentives, or a production peer-discovery protocol.
### 2.4 Race-free implementation in Go
The networked node handles concurrent HTTP requests, mining, gossip, synchronisation, and fork resolution. These operations can access shared state such as the chain, pending transaction pool, peer list, and deduplication maps. Go's race detector is designed to detect data races during runtime testing by using the `-race` flag. This project therefore uses a mutex-protected node wrapper and verifies the implementation with:
```powershell
go test -race ./...
```
Passing the race detector does not prove every possible concurrent execution is safe, but it provides strong practical evidence that the tested paths do not contain runtime data races.
## 3. Architecture
The project is organised into three main areas:
```text
cmd/toychain
internal/blockchain
internal/node
```
The `cmd/toychain` package contains the command-line interface, the single-node REST API, and the networked node command. It parses global flags such as `-data`, `-difficulty`, `-retarget-interval`, and `-target-block-time`, then calls the appropriate blockchain or node operation.
The `internal/blockchain` package contains the core blockchain domain logic. It defines blocks, transactions, wallets, Merkle roots, proof-of-work mining, validation, ledger replay, state persistence, difficulty retargeting, and local fork resolution.
The `internal/node` package adds the networked node layer. It wraps the blockchain state with concurrency protection and exposes HTTP endpoints for status inspection, transaction gossip, block gossip, chain synchronisation, and network fork resolution.
The `scripts` folder contains PowerShell scripts for starting and stopping a three-node local cluster.
## 4. Main data types
The main blockchain types are:
- `Wallet`: Ed25519 public/private key pair and derived address.
- `Transaction`: sender, recipient, amount, creation time, memo, nonce, public key, signature, and deterministic ID.
- `Block`: height, timestamp, difficulty, transaction list, Merkle root, previous hash, nonce, and block hash.
- `State`: confirmed chain plus pending transaction pool.
- `LedgerState`: replayed balances, sender nonces, and seen transaction IDs.
- `MerkleProofStep`: sibling hash and left/right position used for Merkle proof verification.
- `ForkResolutionResult`: result of comparing a local state with a competing candidate state.
The main networked node types are:
- `Node`: concurrency-safe wrapper around local blockchain state.
- `Status`: current height, head hash, pending transaction count, and peer count.
- `TransactionGossipResult`: transaction acceptance and gossip forwarding result.
- `BlockGossipResult`: block acceptance, forwarding, and optional reorganisation result.
- `SyncResult`: result of downloading missing blocks from a peer.
- `ReorgResult`: result of downloading and resolving a competing peer chain.
## 5. Hashing and Merkle-root design
A block hash is computed using SHA-256 over a canonical block-header payload. The block's own `Hash` field is excluded from the hash calculation. The field order is:
1. block height,
2. Unix timestamp,
3. difficulty,
4. previous block hash,
5. Merkle root,
6. nonce.
The transaction list is committed through the Merkle root. Each transaction is first hashed into a Merkle leaf. The leaf commits to the transaction ID, sender, recipient, amount, timestamp, memo, nonce, public key, and signature. Leaves are then paired and hashed upward until one Merkle root remains. If a tree level has an odd number of hashes, the final hash is duplicated for that level.
This design improves the earlier direct-transaction hashing approach because the block header stores a compact transaction commitment. If any transaction field is changed, the recomputed transaction hash changes, the recomputed Merkle root changes, and validation fails.
## 6. Wallets and signed transactions
Wallets use Ed25519 keys from the Go standard library. The wallet address is derived from the public key. A normal transfer transaction includes the sender address, recipient address, amount, nonce, sender public key, and signature.
During validation, the system checks that:
- the sender address matches the public key,
- the signature verifies against the transaction signing payload,
- the transaction ID matches the transaction contents,
- the sender nonce is the expected next nonce,
- the transaction ID has not already appeared,
- the sender has enough balance.
This prevents a user from spending from an address simply by typing the address string. The user must prove ownership of the corresponding private key.
Wallet private keys are encrypted before being written to disk. The implementation uses AES-256-GCM for authenticated encryption. Because the project stays within the Go standard library, the passphrase-derived key is created using an iterative SHA-256 process with a random salt. This is acceptable for an educational exercise, but production wallets should use a memory-hard KDF such as Argon2id or scrypt.
## 7. Validation strategy
Validation scans the chain from genesis to the current head and fails on the first invalid block or transaction. It checks:
- chain is not empty,
- genesis block matches the canonical genesis block,
- block height equals its position,
- previous-hash links are correct,
- timestamps do not move backwards,
- stored Merkle root matches the recomputed Merkle root,
- stored block hash matches the recomputed block hash,
- hash satisfies the stored proof-of-work difficulty,
- block difficulty matches the retargeting rule,
- transaction syntax is valid,
- transaction IDs match transaction fields,
- non-faucet transactions contain valid signatures,
- sender nonces are sequential,
- duplicate transaction IDs are rejected,
- non-faucet transactions have sufficient balance,
- balance overflow is prevented.
This means the JSON state file is never blindly trusted. Even if a user manually edits a saved block or transaction, validation recomputes the data and rejects inconsistent state.
## 8. Mining, goroutines, and difficulty retargeting
Mining searches for a nonce that produces a block hash with the required number of leading zero hexadecimal digits. For example, difficulty `3` requires a hash beginning with `000`.
The implementation supports concurrent mining using goroutines. Each worker searches a different nonce sequence. When one worker finds a valid nonce, it sends the result through a channel and the remaining workers are cancelled through a context.
Difficulty retargeting allows the chain to adjust mining difficulty after a configured interval. The node compares recent mining time against the target block time and adjusts difficulty by at most one level. Validation recalculates the expected difficulty for every block and rejects blocks whose stored difficulty does not match the rule. During local multi-node demonstrations, retargeting is often disabled with:
```powershell
-retarget-interval 0
```
This keeps manual experiments predictable.
## 9. REST API design
The project provides two HTTP modes.
The `serve` command starts the single-node REST API. It supports local chain inspection, faucet funding, signed transaction submission, mining, Merkle proofs, validation, and local fork resolution. Write endpoints can be protected with an optional `X-API-Token` header.
The `node` command starts a networked node. It exposes introspection endpoints and peer endpoints used for gossip, synchronisation, and reorganisation.
The API intentionally does not receive wallet file paths, private keys, or wallet passphrases. Transactions are signed locally with the CLI command:
```powershell
.\toychain.exe tx-sign -wallet alice.wallet.json -passphrase alice-pass -to ADDRESS -amount 25 -out tx.json
```
The signed JSON transaction can then be submitted to a node. This keeps private-key handling on the client side and makes the node responsible only for verification.
## 10. Networked node design
Each networked node runs as an independent process with its own JSON state file. A node is started with an address and peer list:
```powershell
.\toychain.exe -data node1.json -difficulty 1 -retarget-interval 0 node -addr 127.0.0.1:8081 -peers http://127.0.0.1:8082,http://127.0.0.1:8083
```
The node layer protects shared state with a mutex. Protected state includes:
- chain and pending pool,
- peer list,
- seen transaction IDs,
- seen block hashes,
- local self URL.
The HTTP handlers call methods on the `Node` type rather than directly mutating blockchain state. This keeps chain updates, pending-pool updates, gossip deduplication, synchronisation, and reorganisation safe under concurrent requests.
## 11. Network API and wire format
The network API includes read endpoints and peer endpoints.
Important read and control endpoints:
```text
GET  /health
GET  /status
GET  /peers
GET  /chain
GET  /blocks
GET  /blocks/{height}
GET  /pending
GET  /balances
GET  /validate
POST /transactions
POST /mine
POST /sync
POST /resolve-fork
```
Peer endpoints:
```text
POST /peer/transactions
POST /peer/blocks
GET  /peer/status
GET  /peer/blocks/{height}
```
Transaction gossip uses a JSON request containing:
```json
{
  "origin": "http://127.0.0.1:8081",
  "transaction": {}
}
```
Block gossip uses:
```json
{
  "origin": "http://127.0.0.1:8081",
  "block": {}
}
```
The `origin` field helps a receiving node avoid immediately forwarding the same item back to the peer it came from.
## 12. Transaction gossip
When a signed transaction is submitted to `/transactions`, the node validates the transaction and checks whether the transaction ID has already been seen. If it is new and valid, the transaction is added to the pending pool and forwarded to peers through `/peer/transactions`.
The experiment showed that submitting one signed transaction to node 1 resulted in:
```text
accepted = True
gossiped = 2
```
This means node 1 accepted the transaction and forwarded it to two peers in the three-node cluster. Checking the pending pool on all three nodes showed that the transaction was present on every node.
Submitting the same transaction again produced:
```text
accepted = False
gossiped = 0
```
This demonstrates deduplication. The node recognised the transaction ID as already seen and did not forward it again, preventing gossip loops.
## 13. Block gossip
When a node mines through `/mine`, it selects pending transactions, mines a valid proof-of-work block, appends the block locally, removes confirmed transactions from pending, and broadcasts the block to peers through `/peer/blocks`.
A receiving peer validates the block before appending it. The block is accepted only if it is valid and directly extends the local head. After mining on node 1, the experiment showed that all three nodes reached the same height and the same head hash, and pending transaction counts returned to zero. This demonstrates successful block propagation and mempool consistency after confirmation.
## 14. Chain synchronisation
Chain synchronisation supports new or lagging nodes. A node can call `/sync` with a peer URL:
```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8084/sync -Body '{"peer":"http://127.0.0.1:8081"}' -ContentType "application/json"
```
The syncing node first reads the peer status, compares heights, and downloads missing blocks one by one using `/peer/blocks/{height}`. Each downloaded block is validated before being appended.
The experiment used a new node with only the genesis block. After calling `/sync`, the node downloaded the missing blocks and reached the same height and head hash as the peer. This confirms that a lagging node can catch up without directly copying a state file.
## 15. Fork resolution and reorganisation
Fork resolution handles competing chains. If a node receives a peer block that does not extend its current head, the block may represent a competing branch. The node can download the peer chain and run fork resolution.
Manual fork resolution is triggered with:
```powershell
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8091/resolve-fork -Body '{"peer":"http://127.0.0.1:8092"}' -ContentType "application/json"
```
The node adopts the candidate only when:
- the candidate chain is valid,
- the candidate chain is longer than the local chain.
During reorganisation, transactions confirmed in replaced local blocks but absent from the adopted chain are treated as orphaned transactions. The node revalidates those orphaned transactions on top of the adopted chain. Valid orphaned transactions return to the pending pool, while invalid or conflicting transactions are dropped.
The fork experiment created two branches from a common funded chain. Node A had height `2`, while node B had height `3`. Before resolution, the nodes had different head hashes. After node A resolved against node B, the response showed:
```text
decision = adopt_candidate
adopted = True
local_height = 2
candidate_height = 3
after_height = 3
kept_pending = 1
dropped_pending = 0
```
After reorganisation, node A and node B had the same height and head hash. Node A also had one pending transaction, showing that the transaction from the replaced local branch was correctly returned to the pending pool.
## 16. Local cluster launcher
The local cluster launcher starts three connected node processes for demonstration:
```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start-cluster.ps1 -Reset
```
It builds the CLI, prepares node state files, starts each node in its own PowerShell window, writes process IDs to `.cluster/pids.json`, and performs a quick health check.
The expected health output is:
```text
Node 1: health=ok, height=0, pending=0, peers=2
Node 2: health=ok, height=0, pending=0, peers=2
Node 3: health=ok, height=0, pending=0, peers=2
```
The stop script terminates the recorded node processes:
```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\stop-cluster.ps1 -Clean
```
This provides a repeatable local test environment for transaction gossip, block gossip, synchronisation, and fork resolution.
## 17. Experimental verification
The final implementation was verified using:
```powershell
go test ./...
go test -race ./...
go vet ./...
go build -o toychain.exe ./cmd/toychain
```
The tests passed for:
- command-line behaviour,
- blockchain validation,
- wallet and signature rules,
- Merkle root checks,
- mining and retargeting,
- local fork resolution,
- networked node state wrapper,
- transaction gossip,
- block gossip,
- chain synchronisation,
- fork reorganisation,
- race detector execution.
The race detector result is important because the networked node can receive concurrent HTTP requests while also mining, gossiping, syncing, or reorganising. The mutex-protected `Node` wrapper is therefore a necessary design choice rather than an optional improvement.
## 18. Discussion
### 18.1 Finality
The simulator uses a simplified longest-valid-chain rule. This means finality is probabilistic rather than absolute. A transaction confirmed in a block can still be reorganised out if a longer valid competing chain appears. In practice, blockchain users wait for additional confirmations because each new block on top of a transaction makes replacement less likely.
### 18.2 51% attack discussion
In a proof-of-work network, an attacker with majority mining power can attempt to create a longer alternative chain. This can allow double-spending or reversal of recent transactions. This simulator does not model real mining economics or network-wide hash power, but its fork experiment demonstrates the structural idea: if a longer valid chain is presented, the node may adopt it and move transactions from the replaced branch back into pending.
### 18.3 Malicious peer behaviour
Digital signatures prevent a peer from forging transfers from another user's wallet. Merkle roots and block hashes prevent silent transaction tampering. Full-chain validation prevents a node from adopting a structurally invalid chain. However, malicious peers could still attempt denial-of-service behaviour by sending repeated invalid blocks, large requests, stale blocks, or conflicting data. Production systems would need peer scoring, rate limiting, banning, stronger authentication, and more careful resource controls.
### 18.4 Longest chain versus cumulative work
The simulator adopts a longer valid chain by block height. This is simple and understandable, but it is not the strongest production rule. A more accurate proof-of-work fork-choice rule would compare cumulative work so that a shorter chain with higher difficulty could potentially outweigh a longer low-difficulty chain. This is a future improvement.
## 19. Constraints and future improvements
This implementation is suitable for local blockchain learning and assessment demonstrations. It is not production cryptocurrency software.
Current constraints:
- peer networking is local HTTP-based and intended for localhost testing,
- peers are configured manually,
- no automatic peer discovery,
- no NAT traversal,
- no public network deployment,
- fork choice is based on block count rather than cumulative work,
- no transaction fees,
- no mining rewards,
- no smart contracts,
- no gas model,
- no persistent peer database,
- no automatic background sync loop,
- no advanced malicious-peer defence.
Future improvements:
1. Add automatic peer discovery.
2. Add periodic peer health checks.
3. Add automatic background synchronisation.
4. Replace block-count fork choice with cumulative-work fork choice.
5. Add transaction fees and mining rewards.
6. Add persistent mempool storage.
7. Add peer banning for repeated invalid data.
8. Add Docker Compose support for easier cluster demonstrations.
9. Add stronger API authentication and rate limiting.
10. Add a smart contract execution layer as a separate extension.
## 20. Conclusion
The project successfully demonstrates the main internal mechanisms of a blockchain node and extends them into a small local network. The blockchain core provides deterministic hashing, proof-of-work, Merkle-root validation, signed transactions, replay protection, ledger replay, tamper detection, and difficulty retargeting. The networked node layer adds transaction gossip, block gossip, chain synchronisation, fork resolution, reorganisation, orphaned transaction handling, and race-safe shared state.
The experiments show that a transaction submitted to one node propagates to peers, duplicate transactions are not re-gossiped, mined blocks propagate and clear pending pools, lagging nodes can synchronise from peers, and a node can converge to a longer valid peer chain while returning valid orphaned transactions to pending. Overall, the simulator provides a clear and testable model of blockchain behaviour while keeping the implementation small enough to understand and verify.
## References
- Nakamoto, S. (2008). *Bitcoin: A Peer-to-Peer Electronic Cash System*. https://bitcoin.org/bitcoin.pdf
- Decker, C., & Wattenhofer, R. (2013). *Information Propagation in the Bitcoin Network*. IEEE P2P 2013.
- Zheng, Z., Xie, S., Dai, H. N., Chen, X., & Wang, H. (2018). *Blockchain challenges and opportunities: A survey*. International Journal of Web and Grid Services, 14(4), 352-375.
- The Go Programming Language. *Data Race Detector*. https://go.dev/doc/articles/race_detector
- Go documentation: `crypto/ed25519`.
- Go documentation: `crypto/aes` and `crypto/cipher`.
- Go documentation: `crypto/sha256`.
- Go documentation: `context`.
- Go documentation: `net/http`.
