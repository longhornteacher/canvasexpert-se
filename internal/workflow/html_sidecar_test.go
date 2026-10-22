package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// The agent reads packets and student files; raw HTML moves to side files so
// those reads carry readable text once instead of markup twice.
func TestPullKeepsReadableTextAndMovesHTMLToSideFiles(t *testing.T) {
	assignment := fixtureAssignment()
	assignment.Description = `<p>Fix the code.</p><pre>def f():
    return 1</pre>`
	live := fixtureSubmission(5)
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/enrollments"):
			_ = json.NewEncoder(w).Encode([]Enrollment{{UserID: 5, Type: "StudentEnrollment", State: "active", User: live.User}})
		case strings.HasSuffix(r.URL.Path, "/submissions"):
			_ = json.NewEncoder(w).Encode([]Submission{live})
		case strings.Contains(r.URL.Path, "/submissions/5"):
			_ = json.NewEncoder(w).Encode(live)
		default:
			_ = json.NewEncoder(w).Encode(assignment)
		}
	})
	report, err := s.PullWork(context.Background(), "TEST", "10")
	if err != nil || len(report.Packets) != 1 {
		t.Fatalf("pull: %#v %v", report, err)
	}
	var packet domain.PacketDocument
	body, err := workspace.ReadDocument(report.Packets[0], &packet)
	if err != nil {
		t.Fatal(err)
	}
	if packet.Assignment.DescriptionHTML != "" || !strings.Contains(body, "Fix the code.\n```\ndef f():\n    return 1\n```") || strings.Contains(body, "<p>") {
		t.Fatalf("packet should carry readable directions only: %q / %q", packet.Assignment.DescriptionHTML, body)
	}
	if html, e := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(packet.DirectionsHTML))); e != nil || string(html) != assignment.Description {
		t.Fatalf("directions side file: %q %v", html, e)
	}
	var work domain.StudentWorkDocument
	if _, err = workspace.ReadDocument(filepath.Join(s.Root, filepath.FromSlash(packet.Students[0])), &work); err != nil {
		t.Fatal(err)
	}
	attempts := append([]domain.AttemptWork{work.Current}, work.Prior...)
	if len(attempts) != 2 {
		t.Fatalf("attempts: %d", len(attempts))
	}
	for _, attempt := range attempts {
		if attempt.BodyHTML != "" || attempt.ReadableText == "" || !strings.HasSuffix(attempt.BodyHTMLPath, "/attempt-"+strconv.Itoa(attempt.Ref.Attempt)+".html") {
			t.Fatalf("attempt %d kept inline HTML or lost its side file: %+v", attempt.Ref.Attempt, attempt)
		}
		if html, e := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(attempt.BodyHTMLPath))); e != nil || !strings.HasPrefix(string(html), "<p>") {
			t.Fatalf("attempt side file: %q %v", html, e)
		}
	}
}
