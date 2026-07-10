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
