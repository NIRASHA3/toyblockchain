package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := run([]string{"help"}, &out, &errOut); err != nil {
		t.Fatalf("run help: %v", err)
	}
	if !strings.Contains(out.String(), "toychain") {
		t.Fatalf("help output missing program name: %q", out.String())
	}
}

func TestParseAmount(t *testing.T) {
	amount, err := parseAmount("42")
	if err != nil {
		t.Fatalf("parse amount: %v", err)
	}
	if amount != 42 {
		t.Fatalf("amount = %d, want 42", amount)
	}
	if _, err := parseAmount("0"); err == nil {
		t.Fatal("zero amount unexpectedly accepted")
	}
}
