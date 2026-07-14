package blockchain

import (
	"testing"
	"time"
)

func testWallet(t *testing.T) Wallet {
	t.Helper()
	wallet, err := NewWallet()
	if err != nil {
		t.Fatalf("create test wallet: %v", err)
	}
	return wallet
}

func signedTestTransfer(t *testing.T, wallet Wallet, to string, amount int64, nonce uint64, memo string, now time.Time) Transaction {
	t.Helper()
	tx, err := NewSignedTransfer(wallet, to, amount, nonce, memo, now)
	if err != nil {
		t.Fatalf("create signed tx: %v", err)
	}
	return tx
}
