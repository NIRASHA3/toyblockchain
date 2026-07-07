package blockchain

import "fmt"

// ValidateChain verifies hashes, links, proof-of-work, heights, timestamps, and ledger rules.
func ValidateChain(chain []Block, difficulty int) error {
	if difficulty < 0 || difficulty > MaxDifficulty {
		return fmt.Errorf("%w: got %d, supported range is 0..%d", ErrInvalidDifficulty, difficulty, MaxDifficulty)
	}
	if len(chain) == 0 {
		return fmt.Errorf("%w: empty chain", ErrInvalidChain)
	}
	balances := make(Balances)

	for i, block := range chain {
		if err := validateBlockIntegrity(block, i); err != nil {
			return err
		}
		if err := validateBlockDifficulty(block, difficulty); err != nil {
			return err
		}
		if err := validateBlockChaining(block, i, chain); err != nil {
			return err
		}
		if err := validateBlockTransactions(block, balances); err != nil {
			return err
		}
	}
	return nil
}

func validateBlockIntegrity(block Block, index int) error {
	if block.Height != index {
		return newValidationError(block.Height, "height", fmt.Errorf("expected %d got %d", index, block.Height))
	}
	if block.ComputeHash() != block.Hash {
		return newValidationError(block.Height, "hash", fmt.Errorf("stored hash does not match recomputed hash"))
	}
	return nil
}

func validateBlockDifficulty(block Block, difficulty int) error {
	if !MeetsDifficulty(block.Hash, difficulty) {
		return newValidationError(block.Height, "proof-of-work", fmt.Errorf("hash %s does not satisfy difficulty %d", block.Hash, difficulty))
	}
	return nil
}

func validateBlockChaining(block Block, index int, chain []Block) error {
	if index == 0 {
		if block.PrevHash != GenesisPreviousHash {
			return newValidationError(block.Height, "genesis previous hash", fmt.Errorf("expected %s got %s", GenesisPreviousHash, block.PrevHash))
		}
		if block.Timestamp != 0 {
			return newValidationError(block.Height, "genesis timestamp", fmt.Errorf("expected 0 got %d", block.Timestamp))
		}
	} else {
		prev := chain[index-1]
		if block.PrevHash != prev.Hash {
			return newValidationError(block.Height, "previous-hash link", fmt.Errorf("expected %s got %s", prev.Hash, block.PrevHash))
		}
		if block.Timestamp < prev.Timestamp {
			return newValidationError(block.Height, "timestamp", fmt.Errorf("timestamp %d is before previous timestamp %d", block.Timestamp, prev.Timestamp))
		}
	}
	return nil
}

func validateBlockTransactions(block Block, balances Balances) error {
	for _, tx := range block.Transactions {
		if err := ApplyTransaction(balances, tx); err != nil {
			return newValidationError(block.Height, "ledger", err)
		}
	}
	return nil
}
