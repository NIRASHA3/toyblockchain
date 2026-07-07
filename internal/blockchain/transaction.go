package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Transaction represents either a regular transfer or a faucet funding operation.
// Faucet transactions use From == FaucetAccount and are the only way this toy chain
// introduces new funds.
type Transaction struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Amount    int64  `json:"amount"`
	CreatedAt int64  `json:"created_at"`
	Memo      string `json:"memo,omitempty"`
}

// NewTransfer creates a spend transaction from one account to another.
func NewTransfer(from, to string, amount int64, memo string, now time.Time) (Transaction, error) {
	tx := Transaction{From: strings.TrimSpace(from), To: strings.TrimSpace(to), Amount: amount, CreatedAt: now.UnixNano(), Memo: strings.TrimSpace(memo)}
	tx.ID = tx.ComputeID()
	if err := tx.ValidateBasic(); err != nil {
		return Transaction{}, err
	}
	return tx, nil
}

// NewFaucet creates a funding transaction from the special faucet account.
func NewFaucet(to string, amount int64, memo string, now time.Time) (Transaction, error) {
	tx := Transaction{From: FaucetAccount, To: strings.TrimSpace(to), Amount: amount, CreatedAt: now.UnixNano(), Memo: strings.TrimSpace(memo)}
	tx.ID = tx.ComputeID()
	if err := tx.ValidateBasic(); err != nil {
		return Transaction{}, err
	}
	return tx, nil
}

// ComputeID deterministically derives a transaction ID from all transaction fields except ID.
func (tx Transaction) ComputeID() string {
	payload := fmt.Sprintf("from=%d:%s\nto=%d:%s\namount=%d\ncreated_at=%d\nmemo=%d:%s\n",
		len(tx.From), tx.From, len(tx.To), tx.To, tx.Amount, tx.CreatedAt, len(tx.Memo), tx.Memo)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

// ValidateBasic checks transaction syntax and ID consistency. Balance checks are performed
// by ApplyTransaction because they depend on ledger state.
func (tx Transaction) ValidateBasic() error {
	if strings.TrimSpace(tx.To) == "" {
		return fmt.Errorf("%w: recipient is required", ErrInvalidTransaction)
	}
	if strings.TrimSpace(tx.From) == "" {
		return fmt.Errorf("%w: sender is required", ErrInvalidTransaction)
	}
	if tx.Amount <= 0 {
		return fmt.Errorf("%w: amount must be positive", ErrInvalidAmount)
	}
	if strings.TrimSpace(tx.ID) == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidTransaction)
	}
	if expected := tx.ComputeID(); tx.ID != expected {
		return fmt.Errorf("%w: transaction id mismatch: expected %s got %s", ErrInvalidTransaction, expected, tx.ID)
	}
	return nil
}

// IsFaucet reports whether tx introduces new funds.
func (tx Transaction) IsFaucet() bool { return tx.From == FaucetAccount }
