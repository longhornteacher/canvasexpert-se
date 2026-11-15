package mcpserver

import (
	"fmt"
	"strings"
)

// resultText renders a tool result as plain text. The body is unchanged; any
// populated metadata (state, outcome, review and session IDs, URL, result path,
// revision notes, setup outcomes) comes first as "key: value" lines, then a
// blank line. Explicit false values are kept: a setup step that was not done
// must read as not done, not as missing. A result with no metadata is just its
// body.
func resultText(out TextOutput) string {
	var meta []string
	add := func(key, value string) {
		if value != "" {
			meta = append(meta, key+": "+value)
		}
	}
	addBool := func(key string, value *bool) {
		if value != nil {
			meta = append(meta, fmt.Sprintf("%s: %t", key, *value))
		}
	}
	add("state", out.State)
	add("outcome", out.Outcome)
	add("review_id", out.ReviewID)
	add("session_id", out.SessionID)
	add("url", out.URL)
	add("result_path", out.ResultPath)
	for _, note := range out.RevisionNotes {
		meta = append(meta, fmt.Sprintf("revision_note: %s: %s", note.Ref, note.Note))
	}
	if out.SetupStatusAvailable {
		meta = append(meta, "setup_status_available: true")
	}
	addBool("credential_saved", out.CredentialSaved)
	addBool("validation_succeeded", out.ValidationSucceeded)
	addBool("connection_tested", out.ConnectionTested)
	add("setup_outcome", out.SetupOutcome)
	if len(meta) == 0 {
		return out.Text
	}
	return strings.Join(meta, "\n") + "\n\n" + out.Text
}
