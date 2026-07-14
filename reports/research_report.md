# Research Report: Toy Blockchain and Ledger Simulator

## 1. Project scope and problem analysis

This project implements a local command-line blockchain and ledger simulator in pure Go. The goal is to demonstrate the internal behaviour of a small blockchain, including deterministic block hashing, proof-of-work mining, ledger replay, tamper detection, JSON persistence, transaction validation, wallet-based signatures, Merkle-root-based transaction commitment, and read-only REST API access.

The latest improvement adds a read-only REST API for blockchain exploration. Earlier versions were CLI-only. The updated version can expose chain data, blocks, balances, transactions, validation status, and Merkle proofs over local HTTP endpoints without accepting wallet passphrases or changing blockchain state.

## 2. Architecture

The project is organised as a small Go module:

- `cmd/toychain`: command-line interface, read-only HTTP API, and output formatting.
- `internal/blockchain`: domain logic for blocks, transactions, wallets, Merkle roots, mining, validation, balances, and persistence.

The main types are:

- `Wallet`: Ed25519 key pair and derived address.
- `Transaction`: sender, recipient, amount, creation time, memo, nonce, public key, signature, and deterministic ID.
- `Block`: height, Unix timestamp, difficulty, transaction list, Merkle root, previous block hash, nonce, and own hash.
- `State`: confirmed chain plus pending transaction pool.
- `LedgerState`: replayed balances, sender nonces, and seen transaction IDs.
- `MerkleProofStep`: one sibling hash and its left/right position in a Merkle proof path.
- `apiServer`: local read-only HTTP server for chain exploration endpoints.

## 3. Hashing scheme

A block hash is computed using SHA-256 over a canonical block-header byte payload. The block's own `Hash` field is excluded. The field order is:

1. block height,
2. Unix timestamp,
3. difficulty,
4. previous block hash,
5. Merkle root,
6. nonce.

The transaction list is no longer directly serialised into the block hash payload. Instead, every transaction is hashed into a Merkle leaf. Each leaf commits to the transaction ID, sender, recipient, amount, creation timestamp, memo, transaction nonce, public key, and signature. Leaf hashes are paired and hashed upward until one Merkle root remains. If a level has an odd number of hashes, the final hash is duplicated for that level.

The transaction ID is also deterministic. It is computed from the transaction signing payload plus the signature. The signing payload excludes the transaction ID and signature, preventing circular hashing.

Merkle proof generation starts at a selected transaction leaf and records the sibling hash at every tree level. During verification, the transaction hash is combined with each sibling according to its left/right position until a root is reconstructed. The reconstructed root must equal the block's stored Merkle root.

## 4. Wallets and digital signatures

Wallets use Ed25519 keys from the Go standard library. The wallet address is derived by hashing the public key and taking a short address prefix. A transfer transaction includes the sender address, recipient address, amount, nonce, sender public key, and signature.

During validation, the system checks that:

- the sender address matches the public key,
- the signature verifies against the transaction signing payload,
- the transaction ID matches the transaction contents,
- the sender nonce is the next expected nonce,
- the transaction ID has not already appeared in the chain,
- the sender has enough balance.

This improves security because a user can no longer spend from an address just by typing the address string. The transaction must be authorised by the matching private key.

## 5. Encrypted wallet storage

Wallet private keys are encrypted before being written to disk. The implementation uses AES-256-GCM from the Go standard library. AES-GCM provides authenticated encryption, so an incorrect passphrase or modified ciphertext is rejected during wallet loading.

Because the project keeps to the standard library, the passphrase-derived encryption key is created using an iterative SHA-256 process with a random salt. This is acceptable for an educational pure-Go exercise, but a production wallet should use a memory-hard KDF such as Argon2id or scrypt.

## 6. Validation strategy

Validation scans the chain from block 0 to the tip and fails fast on the first offending block. It checks:

- height equals the block's position in the slice,
- recomputed Merkle root equals the stored Merkle root,
- recomputed block hash equals the stored block hash,
- hash satisfies the block's stored proof-of-work difficulty,
- genesis block matches the fixed canonical genesis block,
- every later block points to the previous block's stored hash,
- timestamps do not move backwards,
- every transaction is syntactically valid,
- every transaction ID matches its fields,
- every non-faucet transaction has a valid signature,
- every sender nonce is in the correct sequence,
- duplicate transaction IDs are rejected,
- every non-faucet transaction has sufficient sender balance,
- balance overflow is prevented before mutating balances.
- Merkle proofs can be generated and locally verified for selected transactions.

Because validation recomputes the Merkle root and replays the ledger, it detects both structural tampering and business-rule violations.

## 7. Read-only REST API design

Phase 3A adds a local HTTP server using the Go standard library `net/http` package. The server is started with:

```bash
./toychain -data demo.json -difficulty 3 serve -addr :8080
```

The read-only endpoints are:

- `GET /health`,
- `GET /chain`,
- `GET /blocks`,
- `GET /blocks/{height}`,
- `GET /balances`,
- `GET /transactions/{id}`,
- `GET /merkle-proof?height=2&tx=0`,
- `GET /validate`.

Normal read endpoints load the JSON state and validate the chain before returning data. This prevents the API from presenting hand-edited invalid chain data as if it were valid. The `/validate` endpoint is slightly different because it must be able to report invalid chains instead of refusing to load them silently.

The API is intentionally read-only. It does not receive wallet file paths, private keys, or wallet passphrases. This is a better standard design because wallet secrets should remain on the client side. Future write endpoints should accept already-signed transactions and verify them server-side rather than asking the server to decrypt a wallet.

## 8. Go feature choices

### Interfaces

The core domain package avoids unnecessary interfaces. Concrete types are clearer for this small program. The CLI accepts `io.Writer` values in `run`, making CLI tests possible without a fake framework.

### Goroutines and channels

Mining is the naturally concurrent part of the program. The nonce space is split among workers. A buffered result channel returns the first valid block, and context cancellation stops the remaining workers.

### Context

`context.Context` is used in mining to support cancellation and CLI timeouts. When one worker finds a valid nonce, the context is cancelled and the remaining workers stop cleanly.

### Error handling

Errors are returned rather than printed in the domain package. Lower-level errors are wrapped with `%w`, and chain validation returns a custom `ValidationError` containing the block height and failed check.

## 9. Experiment 1: tamper-evidence

### Setup

Commands used:

```bash
./toychain -data tamper.json -difficulty 3 init -force
./toychain -data tamper.json -difficulty 3 faucet -to ALICE_ADDRESS -amount 100
./toychain -data tamper.json -difficulty 3 mine
./toychain -data tamper.json -difficulty 3 validate
./toychain -data tamper.json tamper -height 1 -tx 0 -amount 999
./toychain -data tamper.json -difficulty 3 validate
```

### Observed output

Before tampering:

```text
mined block height=1 difficulty=3 hash=000... nonce=... attempts=... duration=... workers=...
VALID: 2 blocks checked
```

After tampering:

```text
tampered block=1 tx=0 amount 100 -> 999; run validate to see detection
INVALID: block 1 failed merkle root check: stored merkle root does not match recomputed root
```

### Explanation

The transaction amount is part of the transaction hash leaf. Changing the amount changes the transaction hash, which changes the recomputed Merkle root. The stored Merkle root remains the old value, so validation fails at the Merkle root check before the block hash check.

## 10. Experiment 2: difficulty versus effort

The proof-of-work target is a required number of leading zero hexadecimal digits. One hexadecimal digit has 16 possible values, so adding one required leading zero multiplies the expected search space by about 16. Individual runs can vary because hashing is probabilistic.

Example single-worker mining trend:

| Difficulty | Expected trend |
|---:|---|
| 1 | Usually very fast |
| 2 | More attempts than difficulty 1 |
| 3 | Noticeably more attempts |
| 4 | Can vary but average effort is much higher |
| 5 | Highest supported difficulty for this simulator |

The trend is not linear in the difficulty number. The expected work grows exponentially because each extra zero hex digit adds another 1-in-16 condition.

## 11. Experiment 3: Merkle proof generation

### Setup

Commands used after creating a chain with at least one transaction in block 2:

```bash
./toychain -data demo.json -difficulty 3 merkle-proof -height 2 -tx 0
```

### Observed output

The command prints JSON similar to:

```json
{
  "block_height": 2,
  "transaction_index": 0,
  "transaction_id": "...",
  "transaction_hash": "...",
  "merkle_root": "...",
  "proof": [
    {
      "position": "right",
      "hash": "..."
    }
  ],
  "valid": true
}
```

### Explanation

The proof contains only the sibling hashes needed to reconstruct the Merkle root for the selected transaction. If the transaction hash, proof path, or Merkle root is changed, verification returns false. This demonstrates how block membership can be checked without re-hashing every transaction in the block.

## 12. Experiment 4: read-only REST API

### Setup

After creating and mining a demo chain, the API server was started:

```bash
./toychain -data demo.json -difficulty 3 serve -addr :8080
```

Example API checks:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/blocks/2
curl http://localhost:8080/balances
curl "http://localhost:8080/merkle-proof?height=2&tx=0"
curl http://localhost:8080/validate
```

### Expected result

The server returns JSON responses. `/validate` returns `valid: true` for a correct chain, `/blocks/{height}` returns block details including the Merkle root, `/balances` returns replayed confirmed balances, and `/merkle-proof` returns a proof with `valid: true`.

### Explanation

This demonstrates how the CLI blockchain can be exposed through a backend-style interface without changing the underlying chain state. It also preserves the safer wallet model because the API does not receive passphrases or private keys.

## 12. Discussion

### Why previous-hash links make old tampering impractical in real chains

In this local toy, a user can edit the JSON file and, with enough time, recompute the Merkle root and re-mine the changed block and every following block. In a real chain, old tampering is impractical because the attacker must redo the proof-of-work for the modified block and then catch up with and overtake the honest network's continuing work.

### Alternative to proof-of-work

One alternative is proof-of-stake. Instead of expending hashing work, validators lock economic value and can be penalised if they behave dishonestly. One advantage is lower energy use because validators do not compete by brute-force hashing. One drawback is extra protocol complexity because validator selection, slashing, and finality rules must be designed carefully.

Another simple private-network alternative is proof-of-authority, where known authorised validators may create blocks. Its advantage is simplicity and speed for trusted organisations. Its drawback is centralisation.

### Three ways this toy differs from production blockchains

1. **No distributed consensus.** This program has one local chain file. Production blockchains must handle many nodes, forks, propagation delays, and adversarial participants.
2. **No peer-to-peer network.** Transactions and blocks are not propagated between nodes.
3. **No fork choice rule.** If two different valid histories are created locally, the program does not implement a network rule to choose the canonical one.

The project now includes a Merkle root and a proof command, but it still stores the full transactions inside each local block file.

## 13. Constraints and future improvements

This implementation is suitable for a local CLI blockchain learning project. It is not production money software.

Current constraints:

- no write REST API endpoints yet,
- no peer-to-peer network,
- no distributed consensus,
- no fork choice rule,
- no transaction fees,
- no smart contracts,
- passphrases are supplied through CLI flags,
- the standard-library-only wallet KDF is educational and weaker than Argon2id or scrypt.

Future improvements:

1. Use interactive hidden passphrase input.
2. Replace the educational KDF with Argon2id or scrypt.
3. Add write API support that accepts already-signed transactions.
4. Add peer-to-peer node communication.
5. Add proof-of-authority or fork-resolution logic.
6. Add difficulty retargeting.

## References

- Go documentation: `crypto/ed25519` package.
- Go documentation: `crypto/aes` and `crypto/cipher` packages.
- Go documentation: `crypto/sha256` package.
- Go documentation: `context` package.
- Go documentation: `net/http` package.
- Satoshi Nakamoto, "Bitcoin: A Peer-to-Peer Electronic Cash System".
- Ethereum documentation: Proof-of-stake.
