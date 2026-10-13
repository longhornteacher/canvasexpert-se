package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestInstallUIFailureEmitsWarningAndReturnsError(t *testing.T) {
	var out, progress bytes.Buffer
	var steps []string
	log := func(step, outcome, category, client string) { steps = append(steps, step+":"+outcome+":"+category) }
	fail := func(context.Context, string, string) (installUIReady, error) {
		return installUIReady{}, errors.New("private WebUI did not report startup")
	}
	if err := reportInstallUI(context.Background(), "root", "work", fail, &out, &progress, log); err == nil {
		t.Fatal("UI failure must still be returned so the exit status reflects it")
	}
	got := out.String()
	if !strings.Contains(got, `"ui_warning"`) || !strings.Contains(got, "webui.cmd") || strings.Contains(got, "http") {
		t.Fatalf("warning lacks fallback or invents a URL: %s", got)
	}
	if len(steps) != 1 || steps[0] != "hub:failed:startup" {
		t.Fatalf("log steps = %v", steps)
	}
	out.Reset()
	ok := func(context.Context, string, string) (installUIReady, error) {
		return installUIReady{URL: "http://127.0.0.1:1/setup", InstallRoot: "root", Reused: true, BrowserOutcome: "launch_requested"}, nil
	}
	if err := reportInstallUI(context.Background(), "root", "work", ok, &out, &progress, log); err != nil || !strings.Contains(out.String(), "http://127.0.0.1:1/setup") {
		t.Fatalf("success output changed: %v %s", err, out.String())
	}
}
