package blockchain

import "fmt"

// ValidateChain verifies hashes, links, per-block proof-of-work, heights,
// timestamps, the canonical genesis block, duplicate transactions, signatures,
// nonces, and ledger rules.
func ValidateChain(chain []Block, fallbackDifficulty int) error {
	if err := validateFallbackDifficulty(fallbackDifficulty); err != nil {
		return err
	}
	if err := validateChainNotEmpty(chain); err != nil {
		return err
	}

	ledger := NewLedgerState()
	for i, block := range chain {
		if err := validateBlockAtIndex(chain, i, block, ledger); err != nil {
			return err
		}
	}

	return nil
}

func validateFallbackDifficulty(fallbackDifficulty int) error {
	if fallbackDifficulty < MinDifficulty || fallbackDifficulty > MaxDifficulty {
		return fmt.Errorf(
			"%w: got %d, supported range is %d..%d",
			ErrInvalidDifficulty,
			fallbackDifficulty,
			MinDifficulty,
			MaxDifficulty,
		)
	}
	return nil
}

func validateChainNotEmpty(chain []Block) error {
	if len(chain) == 0 {
		return fmt.Errorf("%w: empty chain", ErrInvalidChain)
	}
	return nil
}

func validateBlockAtIndex(chain []Block, index int, block Block, ledger *LedgerState) error {
	if err := validateBlockIntegrity(block, index); err != nil {
		return err
	}

	if err := validateBlockPositionRules(chain, index, block); err != nil {
		return err
	}

	return validateBlockTransactions(block, ledger)
}

func validateBlockPositionRules(chain []Block, index int, block Block) error {
	if index == 0 {
		return validateCanonicalGenesis(block)
	}

	if err := validateBlockDifficulty(block); err != nil {
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
