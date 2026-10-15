// Package installlog stores a small, sanitized operational history in UserInstall.
package installlog

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const maxBytes = 1 << 20

// Entry is intentionally an allowlist. Never add raw errors, responses, or config here.
type Entry struct {
	Timestamp     string `json:"timestamp"`
	Build         string `json:"build"`
	Step          string `json:"step"`
	Client        string `json:"client,omitempty"`
	Outcome       string `json:"outcome"`
	ErrorCategory string `json:"error_category,omitempty"`
	InstallPath   string `json:"install_path,omitempty"`
	RulesPath     string `json:"rules_path,omitempty"`
}

func Append(root, build string, e Entry) error {
	e.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	e.Build = build
	if !oneOf(e.Step, "hub", "browser", "registration", "cowork_extension", "local_mcp_check", "credential_validation", "credential_save", "origin_configuration") || !oneOf(e.Outcome, "started", "reused", "launch_requested", "file_revealed", "launch_failed", "prepared", "succeeded", "failed", "updated", "unchanged") || !oneOf(e.Client, "", "work", "cowork", "antigravity", "auto") || !oneOf(e.ErrorCategory, "", "startup", "registration", "local_check", "validation", "credential_store", "configuration", "browser", "file_io") {
		return errors.New("install log entry contains an unsupported field value")
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	p := filepath.Join(root, "install-log.jsonl")
	if st, err := os.Stat(p); err == nil && st.Size()+int64(len(b)) > maxBytes {
		_ = os.Remove(p + ".1")
		if err := os.Rename(p, p+".1"); err != nil {
			return err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(b)
	return err
}

func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}
