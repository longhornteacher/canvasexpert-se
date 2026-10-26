package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestFocusedReferencesNeedNoWorkspaceOrCanvas(t *testing.T) {
	for _, section := range []string{"assignment", "page", "module", "quiz", "inbox"} {
		var out, progress bytes.Buffer
		code := run(context.Background(), []string{"help", "formats-" + section}, &out, &progress)
		if code != 0 || !strings.HasPrefix(out.String(), "# "+section+" format") || strings.Contains(out.String(), "### Student operations") {
			t.Fatalf("%s: exit=%d output=%s error=%s", section, code, out.String(), progress.String())
		}
	}
}
