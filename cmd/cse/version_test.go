package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	embedded "canvasexpertse"
)

func TestVersionIdentityAndHelpUseCanonicalVersion(t *testing.T) {
	var out, progress bytes.Buffer
	if code := run(context.Background(), []string{"version", "--json"}, &out, &progress); code != 0 {
		t.Fatalf("version exit %d: %s", code, progress.String())
	}
	var identity struct {
		Version   string `json:"version"`
		Revision  string `json:"revision"`
		BuiltAt   string `json:"built_at"`
		BuildKind string `json:"build_kind"`
	}
	if err := json.Unmarshal(out.Bytes(), &identity); err != nil {
		t.Fatal(err)
	}
	if identity.Version != embedded.ReleaseVersion() || identity.Revision == "" || identity.BuiltAt == "" || identity.BuildKind != "development" {
		t.Fatalf("incorrect identity: %+v", identity)
	}
	out.Reset()
	if code := run(context.Background(), []string{"help"}, &out, &progress); code != 0 || !strings.HasPrefix(out.String(), "CanvasExpert-SE "+identity.Version+" ") {
		t.Fatal("help and JSON version differ")
	}
}
