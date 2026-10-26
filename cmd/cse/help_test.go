package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestHelpUnknownTopicExplainsAndExitsZero(t *testing.T) {
	var out, progress bytes.Buffer
	if code := run(context.Background(), []string{"help", "setup"}, &out, &progress); code != 0 || progress.Len() != 0 {
		t.Fatalf("exit %d, stderr %q", code, progress.String())
	}
	if !strings.Contains(out.String(), "formats, formats-assignment, formats-page, formats-module, formats-quiz, formats-inbox, fields, examples") || !strings.Contains(out.String(), "Usage:") {
		t.Fatal("missing topic guidance or command list")
	}
}
