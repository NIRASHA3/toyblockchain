package blockchain

import "fmt"

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

// ApplyTransaction validates tx against the current ledger and mutates balances if valid.
func ApplyTransaction(balances Balances, tx Transaction) error {
	if err := tx.ValidateBasic(); err != nil {
		return err
	}
	if tx.IsFaucet() {
		balances[tx.To] += tx.Amount
		return nil
	}
	if balances[tx.From] < tx.Amount {
		return fmt.Errorf("%w: %s has %d, needs %d", ErrInsufficientFunds, tx.From, balances[tx.From], tx.Amount)
	}
	balances[tx.From] -= tx.Amount
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
