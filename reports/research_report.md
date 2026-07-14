# Research Report: Toy Blockchain and Ledger Simulator

## 1. Project scope and problem analysis

This project implements a local command-line blockchain and ledger simulator in pure Go. The goal is to demonstrate the internal behaviour of a small blockchain, including deterministic block hashing, proof-of-work mining, ledger replay, tamper detection, JSON persistence, and transaction validation.

The latest improvement adds wallet-based signed transactions. Earlier versions used simple account names, which proved ledger behaviour but did not prove transaction ownership. The updated version derives account addresses from Ed25519 public keys and requires normal transfers to be signed by the sender wallet.

## 2. Architecture

The project is organised as a small Go module:

- `cmd/toychain`: command-line interface and output formatting.
- `internal/blockchain`: domain logic for blocks, transactions, wallets, mining, validation, balances, and persistence.

The main types are:

- `Wallet`: Ed25519 key pair and derived address.
- `Transaction`: sender, recipient, amount, creation time, memo, nonce, public key, signature, and deterministic ID.
- `Block`: height, Unix timestamp, difficulty, transaction list, previous block hash, nonce, and own hash.
- `State`: confirmed chain plus pending transaction pool.
- `LedgerState`: replayed balances, sender nonces, and seen transaction IDs.

## 3. Hashing scheme

A block hash is computed using SHA-256 over a canonical byte payload. The block's own `Hash` field is excluded. The field order is:

1. block height,
2. Unix timestamp,
3. difficulty,
4. previous block hash,
5. nonce,
6. transaction count,
7. for each transaction in order: transaction index, transaction ID, sender, recipient, amount, creation timestamp, memo, transaction nonce, public key, and signature.

String values are length-prefixed before their content is written. This avoids ambiguity such as `ab` + `c` producing the same textual stream as `a` + `bc`.

The transaction ID is also deterministic. It is computed from the transaction signing payload plus the signature. The signing payload excludes the transaction ID and signature, preventing circular hashing.

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
- recomputed hash equals stored hash,
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

Because validation replays the ledger while checking the chain, it detects both structural tampering and business-rule violations.

## 7. Go feature choices

### Interfaces

The core domain package avoids unnecessary interfaces. Concrete types are clearer for this small program. The CLI accepts `io.Writer` values in `run`, making CLI tests possible without a fake framework.

### Goroutines and channels

Mining is the naturally concurrent part of the program. The nonce space is split among workers. A buffered result channel returns the first valid block, and context cancellation stops the remaining workers.

### Context

`context.Context` is used in mining to support cancellation and CLI timeouts. When one worker finds a valid nonce, the context is cancelled and the remaining workers stop cleanly.

### Error handling

Errors are returned rather than printed in the domain package. Lower-level errors are wrapped with `%w`, and chain validation returns a custom `ValidationError` containing the block height and failed check.

## 8. Experiment 1: tamper-evidence

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
INVALID: block 1 failed hash check: stored hash does not match recomputed hash
```

### Explanation

The transaction amount is part of the block's canonical hash payload. Changing the amount changes the recomputed hash. The stored block hash remains the old mined hash, so validation fails at the first altered block during the hash check.

## 9. Experiment 2: difficulty versus effort

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

## 10. Discussion

### Why previous-hash links make old tampering impractical in real chains

In this local toy, a user can edit the JSON file and, with enough time, re-mine the changed block and every following block. In a real chain, old tampering is impractical because the attacker must redo the proof-of-work for the modified block and then catch up with and overtake the honest network's continuing work.

### Alternative to proof-of-work

One alternative is proof-of-stake. Instead of expending hashing work, validators lock economic value and can be penalised if they behave dishonestly. One advantage is lower energy use because validators do not compete by brute-force hashing. One drawback is extra protocol complexity because validator selection, slashing, and finality rules must be designed carefully.

Another simple private-network alternative is proof-of-authority, where known authorised validators may create blocks. Its advantage is simplicity and speed for trusted organisations. Its drawback is centralisation.

### Three ways this toy differs from production blockchains

1. **No distributed consensus.** This program has one local chain file. Production blockchains must handle many nodes, forks, propagation delays, and adversarial participants.
2. **No Merkle tree.** The block hashes the full transaction list directly. Production chains usually store a Merkle root so transaction inclusion can be verified efficiently.
3. **No peer-to-peer network.** Transactions and blocks are not propagated between nodes.

If extending one area next, a Merkle tree would be useful. Each transaction would be hashed into a leaf, pairs of hashes would be combined, and the final Merkle root would be included in the block hash. This would allow transaction inclusion proofs.

## 11. Constraints and future improvements

This implementation is suitable for a local CLI blockchain learning project. It is not production money software.

Current constraints:

- no peer-to-peer network,
- no distributed consensus,
- no Merkle tree,
- no fork choice rule,
- no transaction fees,
- no smart contracts,
- passphrases are supplied through CLI flags,
- the standard-library-only wallet KDF is educational and weaker than Argon2id or scrypt.

Future improvements:

1. Use interactive hidden passphrase input.
2. Replace the educational KDF with Argon2id or scrypt.
3. Add Merkle roots and Merkle proof verification.
4. Add a REST API for block and transaction lookup.
5. Add peer-to-peer node communication.
6. Add proof-of-authority or fork-resolution logic.
7. Add difficulty retargeting.

## References

- Go documentation: `crypto/ed25519` package.
- Go documentation: `crypto/aes` and `crypto/cipher` packages.
- Go documentation: `crypto/sha256` package.
- Go documentation: `context` package.
- Satoshi Nakamoto, "Bitcoin: A Peer-to-Peer Electronic Cash System".
- Ethereum documentation: Proof-of-stake.
