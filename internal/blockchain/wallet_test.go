package blockchain

import (
	"errors"
	"os"
	"testing"
)

func TestWalletRoundTripEncryptedFile(t *testing.T) {
	wallet := testWallet(t)
	path := t.TempDir() + "/alice.wallet.json"
	passphrase := "correct horse battery staple"

	if err := SaveEncryptedWallet(path, wallet, passphrase); err != nil {
		t.Fatalf("save wallet: %v", err)
	}
	metadata, err := ReadWalletMetadata(path)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if metadata.Address != wallet.Address {
		t.Fatalf("metadata address = %s, want %s", metadata.Address, wallet.Address)
	}
	loaded, err := LoadEncryptedWallet(path, passphrase)
	if err != nil {
		t.Fatalf("load wallet: %v", err)
	}
	if loaded.Address != wallet.Address || !loaded.PublicKey.Equal(wallet.PublicKey) || !loaded.PrivateKey.Equal(wallet.PrivateKey) {
		t.Fatal("loaded wallet does not match saved wallet")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read wallet file: %v", err)
	}
	if string(data) == "" || containsPrivateKeyHex(string(data), wallet) {
		t.Fatal("wallet file appears to expose plaintext private key")
	}
}

func TestWalletRejectsWrongPassphrase(t *testing.T) {
	wallet := testWallet(t)
	path := t.TempDir() + "/alice.wallet.json"
	if err := SaveEncryptedWallet(path, wallet, "right passphrase"); err != nil {
		t.Fatalf("save wallet: %v", err)
	}
	if _, err := LoadEncryptedWallet(path, "wrong passphrase"); !errors.Is(err, ErrInvalidWallet) {
		t.Fatalf("error = %v, want ErrInvalidWallet", err)
	}
}

func containsPrivateKeyHex(content string, wallet Wallet) bool {
	return len(wallet.PrivateKey) > 0 && len(content) > 0 && string(wallet.PrivateKey) == content
}
