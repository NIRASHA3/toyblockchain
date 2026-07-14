package blockchain

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Transaction represents either a regular signed transfer or a faucet funding
// operation. Faucet transactions use From == FaucetAccount and are the only way
// this toy chain introduces new funds.
type Transaction struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Amount    int64  `json:"amount"`
	CreatedAt int64  `json:"created_at"`
	Memo      string `json:"memo,omitempty"`
	Nonce     uint64 `json:"nonce,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
	Signature string `json:"signature,omitempty"`
}

// NewSignedTransfer creates a signed spend transaction from a wallet.
func NewSignedTransfer(wallet Wallet, to string, amount int64, nonce uint64, memo string, now time.Time) (Transaction, error) {
	tx := Transaction{
		From:      wallet.Address,
		To:        strings.TrimSpace(to),
		Amount:    amount,
		CreatedAt: now.UnixNano(),
		Memo:      strings.TrimSpace(memo),
		Nonce:     nonce,
		PublicKey: hex.EncodeToString(wallet.PublicKey),
	}
	if err := tx.Sign(wallet.PrivateKey); err != nil {
		return Transaction{}, err
	}
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

// SigningPayload is the exact canonical payload signed by the sender's wallet.
func (tx Transaction) SigningPayload() []byte {
	return []byte(fmt.Sprintf("from=%d:%s\nto=%d:%s\namount=%d\ncreated_at=%d\nmemo=%d:%s\nnonce=%d\npublic_key=%d:%s\n",
		len(tx.From), tx.From,
		len(tx.To), tx.To,
		tx.Amount,
		tx.CreatedAt,
		len(tx.Memo), tx.Memo,
		tx.Nonce,
		len(tx.PublicKey), tx.PublicKey,
	))
}

// Sign signs the transaction payload with an Ed25519 private key and recomputes the ID.
func (tx *Transaction) Sign(privateKey ed25519.PrivateKey) error {
	if len(privateKey) != ed25519.PrivateKeySize {
		return fmt.Errorf("%w: private key length %d", ErrInvalidWallet, len(privateKey))
	}
	tx.Signature = hex.EncodeToString(ed25519.Sign(privateKey, tx.SigningPayload()))
	tx.ID = tx.ComputeID()
	return nil
}

// ComputeID deterministically derives a transaction ID from all transaction fields except ID.
func (tx Transaction) ComputeID() string {
	payload := fmt.Sprintf("%s\nsignature=%d:%s\n", tx.SigningPayload(), len(tx.Signature), tx.Signature)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

// ValidateBasic checks transaction syntax, ID consistency, and transfer signature.
// Balance and nonce sequence checks are performed by LedgerState.ApplyTransaction
// because they depend on ledger state.
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

	if tx.IsFaucet() {
		if tx.Nonce != 0 || tx.PublicKey != "" || tx.Signature != "" {
			return fmt.Errorf("%w: faucet transaction must not include nonce, public key, or signature", ErrInvalidTransaction)
		}
		return nil
	}

	return tx.ValidateSignature()
}

// ValidateSignature verifies that a non-faucet transaction was signed by the
// private key belonging to the sender address.
func (tx Transaction) ValidateSignature() error {
	if tx.Nonce == 0 {
		return fmt.Errorf("%w: nonce must start from 1", ErrInvalidNonce)
	}
	publicKey, err := PublicKeyFromHex(tx.PublicKey)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}
	if expected := AddressFromPublicKey(publicKey); tx.From != expected {
		return fmt.Errorf("%w: sender address does not match public key", ErrInvalidSignature)
	}
	signature, err := hex.DecodeString(strings.TrimSpace(tx.Signature))
	if err != nil {
		return fmt.Errorf("%w: decode signature: %v", ErrInvalidSignature, err)
	}
	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("%w: signature length %d", ErrInvalidSignature, len(signature))
	}
	if !ed25519.Verify(publicKey, tx.SigningPayload(), signature) {
		return fmt.Errorf("%w: verification failed", ErrInvalidSignature)
	}
	return nil
}

// IsFaucet reports whether tx introduces new funds.
func (tx Transaction) IsFaucet() bool { return tx.From == FaucetAccount }
