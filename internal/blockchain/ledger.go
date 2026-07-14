package blockchain

import (
	"fmt"
	"math"
)

// Balances maps account addresses to integer token balances.
type Balances map[string]int64

// Clone returns a defensive copy.
func (b Balances) Clone() Balances {
	out := make(Balances, len(b))
	for k, v := range b {
		out[k] = v
	}
	return out
}

// LedgerState is the replayed chain state used for balances, account nonces,
// and duplicate transaction detection.
type LedgerState struct {
	Balances Balances
	Nonces   map[string]uint64
	SeenTx   map[string]struct{}
}

func NewLedgerState() *LedgerState {
	return &LedgerState{Balances: make(Balances), Nonces: make(map[string]uint64), SeenTx: make(map[string]struct{})}
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

// ApplyTransaction validates tx against the current replayed ledger and mutates
// balances/nonces only if every validation step succeeds.
func (l *LedgerState) ApplyTransaction(tx Transaction) error {
	if _, exists := l.SeenTx[tx.ID]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateTx, tx.ID)
	}
	if err := tx.ValidateBasic(); err != nil {
		return err
	}

	if tx.IsFaucet() {
		if err := ensureCanCredit(l.Balances, tx.To, tx.Amount); err != nil {
			return err
		}
		l.Balances[tx.To] += tx.Amount
		l.SeenTx[tx.ID] = struct{}{}
		return nil
	}

	expectedNonce := l.Nonces[tx.From] + 1
	if tx.Nonce != expectedNonce {
		return fmt.Errorf("%w: %s expected %d got %d", ErrInvalidNonce, tx.From, expectedNonce, tx.Nonce)
	}

	fromBalance := l.Balances[tx.From]
	if fromBalance < tx.Amount {
		return fmt.Errorf("%w: %s has %d, needs %d", ErrInsufficientFunds, tx.From, fromBalance, tx.Amount)
	}
	if err := ensureCanCredit(l.Balances, tx.To, tx.Amount); err != nil {
		return err
	}

	l.Balances[tx.From] = fromBalance - tx.Amount
	l.Balances[tx.To] += tx.Amount
	l.Nonces[tx.From] = tx.Nonce
	l.SeenTx[tx.ID] = struct{}{}
	return nil
}

// ApplyTransaction validates tx against a standalone balance map. This helper is
// kept for focused tests; full chain replay uses LedgerState so nonce and
// duplicate transaction rules are enforced across the whole history.
func ApplyTransaction(balances Balances, tx Transaction) error {
	ledger := NewLedgerState()
	ledger.Balances = balances
	return ledger.ApplyTransaction(tx)
}

// LedgerFromChain derives the complete ledger state by replaying blocks in order.
func LedgerFromChain(chain []Block) (*LedgerState, error) {
	ledger := NewLedgerState()
	for _, block := range chain {
		for _, tx := range block.Transactions {
			if err := ledger.ApplyTransaction(tx); err != nil {
				return nil, newValidationError(block.Height, "ledger", err)
			}
		}
	}
	return ledger, nil
}

// BalancesFromChain derives account balances by replaying all transactions in block order.
func BalancesFromChain(chain []Block) (Balances, error) {
	ledger, err := LedgerFromChain(chain)
	if err != nil {
		return nil, err
	}
	return ledger.Balances, nil
}
