# Research Report: Toy Blockchain and Ledger Simulator

## 1. Project scope and problem analysis

This project implements a local command-line blockchain and ledger simulator in Go. The system is intentionally small and self-contained: it does not connect to peers, use external blockchain services, or depend on third-party chain libraries. The main engineering problem is to represent an append-only chain where each block is linked to its predecessor, mined through a configurable proof-of-work rule, and validated deterministically after it is stored.

Correctness depends on two related properties. First, every block hash must be derived from the block's exact contents and its previous-hash link. Second, account balances must be reproducible from confirmed transaction history rather than treated as independent mutable state. If a transaction in an earlier block is changed, the recomputed hash must differ from the stored hash, and validation must report the first block where the inconsistency appears.

The ledger model supports simple transfers between named accounts. A reserved `FAUCET` sender introduces funds for testing and demonstration, avoiding the need for mining rewards, wallets, or digital signatures. This keeps the implementation focused on hashing, mining, validation, persistence, and Go code quality.

## 2. Architecture

The code is organised as a small Go module with a clear separation between command-line concerns and domain logic:

- `cmd/toychain`: command-line parsing, user-facing output, and orchestration.
- `internal/blockchain`: blocks, transactions, mining, validation, ledger replay, pending transactions, and persistence.

The main domain types are:

- `Transaction`: sender, recipient, integer amount, creation time, memo, and deterministic transaction ID.
- `Block`: height, Unix timestamp, transaction list, previous block hash, nonce, and own hash.
- `State`: confirmed chain plus pending transaction pool.
- `Balances`: derived account-balance map used when admitting transactions and validating the chain.

The CLI works with a JSON state file, so the chain survives between runs. Global flags allow the data path, difficulty, block size, worker count, and mining timeout to be configured without changing code.

## 3. Hashing scheme

A block hash is computed with SHA-256 over a canonical byte payload. The block's own `Hash` field is excluded, because including it would make the hash self-referential and impossible to reproduce directly.

The block fields are written into the hash payload in this exact order:

1. block height,
2. Unix timestamp,
3. previous block hash,
4. nonce,
5. transaction count,
6. for each transaction in order: transaction index, transaction ID, sender, recipient, amount, creation timestamp, and memo.

String values are length-prefixed before their contents are written. This prevents ambiguous serialisation cases, such as `ab` + `c` being indistinguishable from `a` + `bc` in a plain concatenated text stream.

This scheme gives deterministic hashing: the same block fields and nonce always produce the same SHA-256 hash, and a change to any hashed field produces a different recomputed hash with overwhelming probability.

## 4. Validation strategy

Validation scans the chain from the genesis block to the latest block and stops at the first detected error. For each block, it checks:

- the block height matches its position in the chain,
- the stored hash matches a recomputation of the block hash,
- the hash satisfies the configured proof-of-work target,
- the genesis block uses the fixed all-zero previous hash,
- every non-genesis block points to the previous block's stored hash,
- timestamps do not move backwards,
- every transaction is syntactically valid,
- every transaction ID matches its fields,
- every non-faucet transaction has sufficient sender balance when replayed in order.

The validation function returns a custom validation error containing the offending block height and the failed check. This makes failures easier to diagnose than a generic true/false result.

Ledger validation is performed by replaying transactions in chain order. Faucet transactions add funds to recipients. Normal transactions subtract from the sender and add to the recipient only after the sender's confirmed balance is checked. As a result, validation checks both chain integrity and ledger correctness.

## 5. Go feature choices

### Standard library focus

The implementation uses the Go standard library only. Packages such as `crypto/sha256`, `encoding/json`, `flag`, `context`, `sync`, `time`, and `os` cover the required hashing, persistence, CLI parsing, cancellation, concurrency, timing, and file handling needs. No third-party dependency is necessary for this scope.

### Interfaces

The core package uses concrete types rather than broad interfaces. This keeps the design simple and readable for a small command-line application. The CLI is still testable because its runner accepts `io.Writer` values for output, allowing tests to capture command results without mocking the entire application.

### Goroutines and channels

Mining is the only part of the program that benefits naturally from concurrency. The nonce space is divided across worker goroutines. Each worker searches a separate sequence of nonce values, and the first successful worker sends the mined block through a result channel. This reduces mining wall-clock time while keeping ledger and persistence operations single-path and race-free.

### Context cancellation

Mining accepts a `context.Context` so the command can stop cleanly when a timeout is reached or when one worker finds a valid nonce. Once a result is found, the context is cancelled and the remaining workers exit instead of continuing unnecessary hashing work.

### Error handling

The domain package returns errors instead of printing directly. Lower-level errors are wrapped with `%w`, allowing callers to preserve root causes while adding useful context. Validation failures use a custom `ValidationError` so the caller can report the block height and failed validation rule precisely.

## 6. Experiment 1: tamper-evidence

### Setup

Commands used:

```bash
./toychain -data tamper.json -difficulty 3 -workers 1 init -force
./toychain -data tamper.json -difficulty 3 -workers 1 faucet -to alice -amount 100
./toychain -data tamper.json -difficulty 3 -workers 1 mine
./toychain -data tamper.json -difficulty 3 validate
./toychain -data tamper.json -difficulty 3 tamper -height 1 -tx 0 -amount 999
./toychain -data tamper.json -difficulty 3 validate
```

### Observed output

Before tampering:

```text
mined block height=1 hash=0003405eafa524d3c23c8107b4ae15e91b256783e6ed0f62fa4678b09632b0d4 nonce=326 attempts=327 duration=2ms workers=1
VALID: 2 blocks checked at difficulty 3
```

After tampering:

```text
tampered block=1 tx=0 amount 100 -> 999; run validate to see detection
INVALID: block 1 failed hash check: stored hash does not match recomputed hash
```

### Explanation

The transaction amount is part of the block's canonical hash payload. Changing the amount from `100` to `999` changes the recomputed hash. The stored block hash remains the original mined hash, so validation fails at block 1 during the hash check. In this case, validation does not need to reach the previous-hash-link check because the first modified block is already invalid.

## 7. Experiment 2: difficulty versus effort

The proof-of-work target is a required number of leading zero hexadecimal digits. One hexadecimal digit has 16 possible values, so each additional required zero multiplies the expected search space by approximately 16. Individual runs can vary because mining is probabilistic.

Single-worker mining results from the implementation:

| Difficulty | Nonce found | Hash attempts | Duration |
|---:|---:|---:|---:|
| 1 | 11 | 12 | 0 ms |
| 2 | 107 | 108 | 1 ms |
| 3 | 6,828 | 6,829 | 28 ms |
| 4 | 10,251 | 10,252 | 41 ms |
| 5 | 558,651 | 558,652 | 1.857 s |

The trend is not linear in the difficulty number. Expected work grows exponentially because each extra leading zero adds another 1-in-16 condition. The difficulty-4 result finished earlier than the rough expectation because a valid nonce was found relatively quickly in that individual run. Over many runs, the average number of attempts would move closer to the exponential expectation.

## 8. Discussion

### Previous-hash links and tamper resistance

In this local implementation, a user with write access to the JSON file can alter history and, with enough computation, re-mine the modified block and every following block. In a real proof-of-work network, rewriting old history is far more difficult because the attacker must redo the work for the changed block and then catch up with the continuing work of the honest network. The previous-hash link ensures that a change to one old block invalidates that block and every descendant unless all affected proof-of-work is redone.

### Alternative to proof-of-work

One alternative is proof-of-stake. Instead of using brute-force hashing, validators lock economic value and can be penalised for dishonest behaviour. A major advantage is lower energy consumption because validators do not compete by continuously hashing. A drawback is increased protocol complexity: validator selection, penalties, finality, and governance must be designed carefully.

Another alternative for private or consortium systems is proof-of-authority. In proof-of-authority, known authorised validators are allowed to create blocks. Its advantage is speed and operational simplicity in trusted environments. Its drawback is centralisation, because users must trust the selected authority set.

### Differences from production blockchains

This implementation differs from a production blockchain in several important ways:

1. **No distributed consensus.** The program stores one local chain file. Production networks must handle many nodes, forks, propagation delay, and adversarial behaviour.
2. **No transaction signatures.** Transactions identify senders by plain text account names. Production blockchains require cryptographic signatures so only the owner of funds can authorise spending.
3. **No Merkle tree.** The block hash directly includes the transaction list. Production systems often use a Merkle root so transaction inclusion can be proven efficiently without downloading every transaction.
4. **No finality model.** A local file has no network-level rule for when a block should be considered irreversible.

If this project were extended, the first improvement would be transaction signatures. Each account could be represented by a public key. A transaction would include a signature over its canonical transaction payload, and validation would verify that signature before applying the transfer. This would prevent a user from creating a transaction that spends from another account name without authorisation.

## 9. Constraints and future improvements

The implementation satisfies the intended local simulator scope: command-line operation, deterministic hashing, proof-of-work mining, ledger replay, validation, tamper detection, JSON persistence, and automated tests. It is not designed to be secure financial software or a distributed blockchain network.

The main constraints are intentional: there is no peer-to-peer networking, no digital identity system, no Sybil resistance, no mempool policy, no fork-choice rule, and no production finality mechanism. Difficulty is capped at a practical level so the program remains easy to run on a laptop. These constraints keep the project focused on the required engineering goals while leaving clear paths for future work.

## References

- Go documentation: `crypto/sha256` package.
- Go documentation: `context` package.
- Satoshi Nakamoto, "Bitcoin: A Peer-to-Peer Electronic Cash System".
- Ethereum documentation: Proof-of-stake.
