package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"toyblockchain/internal/blockchain"
)

func createCLIWallet(t *testing.T, passphrase string) (string, string) {
	t.Helper()
	wallet, err := blockchain.NewWallet()
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}
	path := t.TempDir() + "/wallet.json"
	if err := blockchain.SaveEncryptedWallet(path, wallet, passphrase); err != nil {
		t.Fatalf("save wallet: %v", err)
	}
	return path, wallet.Address
}

func TestCLIRejectsInvalidFaucetAmounts(t *testing.T) {
	path := t.TempDir() + "/cli-invalid.json"
	_, alice := createCLIWallet(t, "alice-pass")
	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := run([]string{"-data", path, "init", "-force"}, &out, &errOut); err != nil {
		t.Fatalf("init: %v", err)
	}

	tests := []struct {
		name   string
		amount string
	}{
		{name: "zero", amount: "0"},
		{name: "negative", amount: "-5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run([]string{"-data", path, "faucet", "-to", alice, "-amount", tt.amount}, &out, &errOut)
			if !errors.Is(err, blockchain.ErrInvalidAmount) {
				t.Fatalf("error = %v, want ErrInvalidAmount", err)
			}
		})
	}
}

func TestCLIRejectsOverspendAndKeepsPendingClean(t *testing.T) {
	path := t.TempDir() + "/cli-overspend.json"
	aliceWallet, alice := createCLIWallet(t, "alice-pass")
	_, bob := createCLIWallet(t, "bob-pass")
	var out bytes.Buffer
	var errOut bytes.Buffer

	commands := [][]string{
		{"-data", path, "-difficulty", "1", "init", "-force"},
		{"-data", path, "-difficulty", "1", "faucet", "-to", alice, "-amount", "100"},
		{"-data", path, "-difficulty", "1", "mine"},
	}

	for _, args := range commands {
		if err := run(args, &out, &errOut); err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
	}

	err := run([]string{"-data", path, "-difficulty", "1", "tx", "-wallet", aliceWallet, "-passphrase", "alice-pass", "-to", bob, "-amount", "150"}, &out, &errOut)
	if !errors.Is(err, blockchain.ErrInsufficientFunds) {
		t.Fatalf("error = %v, want ErrInsufficientFunds", err)
	}

	out.Reset()
	if err := run([]string{"-data", path, "-difficulty", "1", "pending"}, &out, &errOut); err != nil {
		t.Fatalf("pending: %v", err)
	}

	if !strings.Contains(out.String(), "no pending transactions") {
		t.Fatalf("pending output = %q, want no pending transactions", out.String())
	}
}

func TestCLIValidateReturnsErrorForInvalidChain(t *testing.T) {
	path := t.TempDir() + "/cli-invalid-chain.json"
	_, alice := createCLIWallet(t, "alice-pass")
	var out bytes.Buffer
	var errOut bytes.Buffer

	commands := [][]string{
		{"-data", path, "-difficulty", "1", "init", "-force"},
		{"-data", path, "-difficulty", "1", "faucet", "-to", alice, "-amount", "100"},
		{"-data", path, "-difficulty", "1", "mine"},
		{"-data", path, "tamper", "-height", "1", "-tx", "0", "-amount", "999"},
	}

	for _, args := range commands {
		if err := run(args, &out, &errOut); err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
	}

	out.Reset()
	err := run([]string{"-data", path, "-difficulty", "1", "validate"}, &out, &errOut)
	if !errors.Is(err, errValidationFailed) {
		t.Fatalf("validate error = %v, want errValidationFailed", err)
	}

	if !strings.Contains(out.String(), "INVALID:") {
		t.Fatalf("validate output = %q, want INVALID", out.String())
	}
}

func TestCLIWalletNewShowAndSignedTransfer(t *testing.T) {
	base := t.TempDir()
	aliceWallet := base + "/alice.wallet.json"
	bobWallet := base + "/bob.wallet.json"
	chainPath := base + "/chain.json"
	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := run([]string{"wallet", "new", "-out", aliceWallet, "-passphrase", "alice-pass"}, &out, &errOut); err != nil {
		t.Fatalf("wallet new alice: %v", err)
	}
	out.Reset()
	if err := run([]string{"wallet", "new", "-out", bobWallet, "-passphrase", "bob-pass"}, &out, &errOut); err != nil {
		t.Fatalf("wallet new bob: %v", err)
	}

	aliceMeta, err := blockchain.ReadWalletMetadata(aliceWallet)
	if err != nil {
		t.Fatalf("read alice metadata: %v", err)
	}
	bobMeta, err := blockchain.ReadWalletMetadata(bobWallet)
	if err != nil {
		t.Fatalf("read bob metadata: %v", err)
	}

	commands := [][]string{
		{"-data", chainPath, "-difficulty", "1", "init", "-force"},
		{"-data", chainPath, "-difficulty", "1", "faucet", "-to", aliceMeta.Address, "-amount", "100"},
		{"-data", chainPath, "-difficulty", "1", "mine"},
		{"-data", chainPath, "-difficulty", "1", "tx", "-wallet", aliceWallet, "-passphrase", "alice-pass", "-to", bobMeta.Address, "-amount", "40"},
		{"-data", chainPath, "-difficulty", "1", "mine"},
		{"-data", chainPath, "-difficulty", "1", "validate"},
	}
	for _, args := range commands {
		if err := run(args, &out, &errOut); err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
	}
	out.Reset()
	if err := run([]string{"-data", chainPath, "-difficulty", "1", "balances"}, &out, &errOut); err != nil {
		t.Fatalf("balances: %v", err)
	}
	if !strings.Contains(out.String(), aliceMeta.Address) || !strings.Contains(out.String(), bobMeta.Address) {
		t.Fatalf("balances output missing wallet addresses: %q", out.String())
	}
}
