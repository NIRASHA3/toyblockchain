package blockchain

import "fmt"

// ValidateChain verifies hashes, links, per-block proof-of-work, heights,
// timestamps, the canonical genesis block, duplicate transactions, signatures,
// nonces, and ledger rules. Retargeting is disabled in this compatibility helper;
// use ValidateChainWithConfig when validating retargeted chains.
func ValidateChain(chain []Block, fallbackDifficulty int) error {
	cfg := Config{Difficulty: fallbackDifficulty, MaxBlockTx: DefaultMaxBlockTx, RetargetInterval: 0}
	return ValidateChainWithConfig(chain, cfg)
}

// ValidateChainWithConfig verifies the chain and also checks that each non-genesis
// block uses the difficulty expected by the configured retargeting rules.
func ValidateChainWithConfig(chain []Block, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := validateChainNotEmpty(chain); err != nil {
		return err
	}

	ledger := NewLedgerState()
	for i, block := range chain {
		if err := validateBlockAtIndex(chain, i, block, ledger, cfg); err != nil {
			return err
		}
	}

	return nil
}

func validateChainNotEmpty(chain []Block) error {
	if len(chain) == 0 {
		return fmt.Errorf("%w: empty chain", ErrInvalidChain)
	}
	return nil
}

func validateBlockAtIndex(chain []Block, index int, block Block, ledger *LedgerState, cfg Config) error {
	if err := validateBlockIntegrity(block, index); err != nil {
		return err
	}

	if err := validateBlockPositionRules(chain, index, block, cfg); err != nil {
		return err
	}

	return validateBlockTransactions(block, ledger)
}

func validateBlockPositionRules(chain []Block, index int, block Block, cfg Config) error {
	if index == 0 {
		return validateCanonicalGenesis(block)
	}

	if err := validateBlockDifficulty(block); err != nil {
		return err
	}

	if err := validateExpectedDifficulty(chain[:index], block, cfg); err != nil {
		return err
	}

	return validateBlockChaining(block, index, chain)
}

func validateBlockIntegrity(block Block, index int) error {
	if block.Height != index {
		return newValidationError(
			block.Height,
			"height",
			fmt.Errorf("expected %d got %d", index, block.Height),
		)
	}

	computedMerkleRoot := ComputeMerkleRoot(block.Transactions)
	if block.MerkleRoot != computedMerkleRoot {
		return newValidationError(
			block.Height,
			"merkle root",
			fmt.Errorf("stored merkle root does not match recomputed root"),
		)
	}

	if block.ComputeHash() != block.Hash {
		return newValidationError(
			block.Height,
			"hash",
			fmt.Errorf("stored hash does not match recomputed hash"),
		)
	}

	return nil
}

func validateCanonicalGenesis(block Block) error {
	expected := NewGenesisBlock()
	if !isCanonicalGenesis(block, expected) {
		return newValidationError(block.Height, "genesis", ErrInvalidGenesis)
	}

	if !MeetsDifficulty(block.Hash, block.Difficulty) {
		return newValidationError(
			block.Height,
			"proof-of-work",
			fmt.Errorf("genesis hash %s does not satisfy difficulty %d", block.Hash, block.Difficulty),
		)
	}

	return nil
}

func isCanonicalGenesis(block Block, expected Block) bool {
	return block.Height == expected.Height &&
		block.Timestamp == expected.Timestamp &&
		block.Difficulty == expected.Difficulty &&
		block.PrevHash == expected.PrevHash &&
		block.MerkleRoot == expected.MerkleRoot &&
		block.Nonce == expected.Nonce &&
		block.Hash == expected.Hash &&
		len(block.Transactions) == 0
}

func validateBlockDifficulty(block Block) error {
	if block.Difficulty < MinDifficulty || block.Difficulty > MaxDifficulty {
		return newValidationError(
			block.Height,
			"difficulty",
			fmt.Errorf(
				"%w: got %d, supported range is %d..%d",
				ErrInvalidDifficulty,
				block.Difficulty,
				MinDifficulty,
				MaxDifficulty,
			),
		)
	}

	if !MeetsDifficulty(block.Hash, block.Difficulty) {
		return newValidationError(
			block.Height,
			"proof-of-work",
			fmt.Errorf("hash %s does not satisfy stored difficulty %d", block.Hash, block.Difficulty),
		)
	}

	return nil
}

func validateExpectedDifficulty(previousChain []Block, block Block, cfg Config) error {
	// The first non-genesis block establishes the starting runtime difficulty.
	// This lets validation use the difficulty stored in the chain instead of
	// requiring callers to remember the exact CLI -difficulty used for block 1.
	if block.Height == 1 {
		return nil
	}
	expected, err := cfg.NextDifficulty(previousChain)
	if err != nil {
		return newValidationError(block.Height, "difficulty retarget", err)
	}
	if block.Difficulty != expected {
		return newValidationError(
			block.Height,
			"difficulty retarget",
			fmt.Errorf("expected difficulty %d got %d", expected, block.Difficulty),
		)
	}
	return nil
}

func validateBlockChaining(block Block, index int, chain []Block) error {
	prev := chain[index-1]

	if block.PrevHash != prev.Hash {
		return newValidationError(
			block.Height,
			"previous-hash link",
			fmt.Errorf("expected %s got %s", prev.Hash, block.PrevHash),
		)
	}

	if block.Timestamp < prev.Timestamp {
		return newValidationError(
			block.Height,
			"timestamp",
			fmt.Errorf("timestamp %d is before previous timestamp %d", block.Timestamp, prev.Timestamp),
		)
	}

	return nil
}

func validateBlockTransactions(block Block, ledger *LedgerState) error {
	for _, tx := range block.Transactions {
		if err := ledger.ApplyTransaction(tx); err != nil {
			return newValidationError(block.Height, "ledger", err)
		}
	}

	return nil
}
