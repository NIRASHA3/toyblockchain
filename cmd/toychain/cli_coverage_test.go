package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLIPrintShowsChainDetails(t *testing.T) {
	path := t.TempDir() + "/cli-print.json"
	var out bytes.Buffer
	var errOut bytes.Buffer

	commands := [][]string{
		{"-data", path, "init", "-force"},
		{"-data", path, "-difficulty", "1", "faucet", "-to", "alice", "-amount", "100"},
		{"-data", path, "-difficulty", "1", "mine"},
	}

	for _, args := range commands {
		if err := run(args, &out, &errOut); err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
	}

	out.Reset()
	if err := run([]string{"-data", path, "-difficulty", "1", "print"}, &out, &errOut); err != nil {
		t.Fatalf("print: %v", err)
	}

	output := out.String()
	checks := []string{
		"Block 0",
		"Block 1",
		"hash:",
		"tx_count:   1",
		"[0] FAUCET -> alice amount=100",
		"memo=\"faucet funding\"",
	}

	for _, want := range checks {
		if !strings.Contains(output, want) {
			t.Fatalf("print output missing %q:\n%s", want, output)
		}
	}
}

func TestCLITamperRejectsOutOfRangeHeightAndTx(t *testing.T) {
	path := t.TempDir() + "/cli-tamper.json"
	var out bytes.Buffer
	var errOut bytes.Buffer

	commands := [][]string{
		{"-data", path, "init", "-force"},
		{"-data", path, "-difficulty", "1", "faucet", "-to", "alice", "-amount", "100"},
		{"-data", path, "-difficulty", "1", "mine"},
	}

	for _, args := range commands {
		if err := run(args, &out, &errOut); err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
	}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "height",
			args: []string{"-data", path, "tamper", "-height", "2", "-tx", "0", "-amount", "999"},
			want: "height 2 out of range",
		},
		{
			name: "tx-index",
			args: []string{"-data", path, "tamper", "-height", "1", "-tx", "1", "-amount", "999"},
			want: "transaction index 1 out of range for block 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(tt.args, &out, &errOut)
			if err == nil {
				t.Fatalf("run %v: expected error", tt.args)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}
