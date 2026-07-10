package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"toyblockchain/internal/blockchain"
)

func TestCLIRejectsInvalidFaucetAmounts(t *testing.T) {
	path := t.TempDir() + "/cli-invalid.json"
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
			err := run([]string{"-data", path, "faucet", "-to", "alice", "-amount", tt.amount}, &out, &errOut)
			if !errors.Is(err, blockchain.ErrInvalidAmount) {
				t.Fatalf("error = %v, want ErrInvalidAmount", err)
			}
		})
	}
}

func TestCLIRejectsOverspendAndKeepsPendingClean(t *testing.T) {
	path := t.TempDir() + "/cli-overspend.json"
	var out bytes.Buffer
	var errOut bytes.Buffer

	commands := [][]string{
		{"-data", path, "-difficulty", "1", "init", "-force"},
		{"-data", path, "-difficulty", "1", "faucet", "-to", "alice", "-amount", "100"},
		{"-data", path, "-difficulty", "1", "mine"},
	}

	for _, args := range commands {
		if err := run(args, &out, &errOut); err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
	}

	err := run([]string{"-data", path, "-difficulty", "1", "tx", "-from", "alice", "-to", "bob", "-amount", "150"}, &out, &errOut)
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
	var out bytes.Buffer
	var errOut bytes.Buffer

	commands := [][]string{
		{"-data", path, "-difficulty", "1", "init", "-force"},
		{"-data", path, "-difficulty", "1", "faucet", "-to", "alice", "-amount", "100"},
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
