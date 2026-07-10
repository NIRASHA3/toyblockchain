package blockchain

import (
	"fmt"
	"math"
)

// Balances maps account names to integer token balances.
type Balances map[string]int64

// Clone returns a defensive copy.
func (b Balances) Clone() Balances {
	out := make(Balances, len(b))
	for k, v := range b {
		out[k] = v
	}
	return out
}

func ensureCanCredit(balances Balances, account string, amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("%w: amount must be positive", ErrInvalidAmount)
	}
	current := balances[account]
	if current > math.MaxInt64-amount {
		return fmt.Errorf("%w: crediting %s by %d would exceed maximum balance", ErrBalanceOverflow, account, amount)
	}
	return nil
}

// ApplyTransaction validates tx against the current ledger and mutates balances if valid.
func ApplyTransaction(balances Balances, tx Transaction) error {
	if err := tx.ValidateBasic(); err != nil {
		return err
	}

	if tx.IsFaucet() {
		if err := ensureCanCredit(balances, tx.To, tx.Amount); err != nil {
			return err
		}
		balances[tx.To] += tx.Amount
		return nil
	}

	fromBalance := balances[tx.From]
	if fromBalance < tx.Amount {
		return fmt.Errorf("%w: %s has %d, needs %d", ErrInsufficientFunds, tx.From, fromBalance, tx.Amount)
	}
	if err := ensureCanCredit(balances, tx.To, tx.Amount); err != nil {
		return err
	}

	// Mutate only after every validation step succeeds, so failed transactions
	// cannot partially debit the sender.
	balances[tx.From] = fromBalance - tx.Amount
	balances[tx.To] += tx.Amount
	return nil
}

// BalancesFromChain derives account balances by replaying all transactions in block order.
func BalancesFromChain(chain []Block) (Balances, error) {
	balances := make(Balances)
	for _, block := range chain {
		for _, tx := range block.Transactions {
			if err := ApplyTransaction(balances, tx); err != nil {
				return nil, newValidationError(block.Height, "ledger", err)
			}
		}
	}
	return balances, nil
}
