package blockchain

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidAmount       = errors.New("invalid transaction amount")
	ErrInvalidTransaction  = errors.New("invalid transaction")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrBalanceOverflow     = errors.New("balance overflow")
	ErrInvalidDifficulty   = errors.New("invalid difficulty")
	ErrInvalidChain        = errors.New("invalid chain")
	ErrInvalidGenesis      = errors.New("invalid genesis block")
	ErrNoPendingTx         = errors.New("no pending transactions")
	ErrBlockSizeOutOfRange = errors.New("block size out of range")
)

// ValidationError identifies the first block and validation check that failed.
type ValidationError struct {
	Height int
	Check  string
	Err    error
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("block %d failed %s check: %v", e.Height, e.Check, e.Err)
}

func (e *ValidationError) Unwrap() error { return e.Err }

func newValidationError(height int, check string, err error) *ValidationError {
	return &ValidationError{Height: height, Check: check, Err: err}
}
