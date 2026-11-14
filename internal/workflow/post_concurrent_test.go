package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// TestConcurrentPostSavesEveryOutcomeAndNeverRepeats posts a class whose
// students run in parallel workers, with one uncertain comment write and one
// failed submission read among them. Every outcome must reach the saved file
// whatever order the workers finish in, and a rerun must write only what was
// held, never repeating an accepted or uncertain write.
func TestConcurrentPostSavesEveryOutcomeAndNeverRepeats(t *testing.T) {
	assignment := fixtureAssignment()
	assignment.Rubric, assignment.RubricAssociation = nil, nil
	assignment.RubricSettings.ID = 0
	const uncertain, unreadable = int64(103), int64(105)
	var mu sync.Mutex
	inFlight, maxInFlight := 0, 0
	puts := map[int64]int{}
	failedRead := false
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.Contains(r.URL.Path, "/submissions/") {
			_ = json.NewEncoder(w).Encode(assignment)
			return
		}
		var user int64
		_, _ = fmt.Sscanf(r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:], "%d", &user)
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		mu.Unlock()
		time.Sleep(15 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		inFlight--
		if r.Method == http.MethodGet {
			if user == unreadable && !failedRead {
				failedRead = true
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(fixtureSubmission(user))
			return
		}
		puts[user]++
		if user == uncertain {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	users := []int64{101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112}
	path := scoringFiles(t, s, users...)
	var doc domain.ResultsDocument
	if _, err := workspace.ReadDocument(path, &doc); err != nil {
		t.Fatal(err)
	}
	doc.Assignment.Rubric = nil
	doc.Interpretation = domain.Interpretation{}
	for i := range doc.Students {
		r := &doc.Students[i]
		r.Operation = "comment-only"
		r.CommentOnly = &domain.CommentOnlyOperation{Attempt: r.Grade.Attempt, Header: "none", Feedback: "Feedback for " + r.StudentRef}
		r.Grade = nil
	}
	packetPath := filepath.Join(s.Root, filepath.FromSlash(doc.Packet))
	var packet domain.PacketDocument
	if _, err := workspace.ReadDocument(packetPath, &packet); err != nil {
		t.Fatal(err)
	}
	packet.Assignment = doc.Assignment
	if err := workspace.WriteDocument(packetPath, packet, ""); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteDocument(path, doc, "results"); err != nil {
		t.Fatal(err)
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	userOf := map[string]int64{}
	for _, r := range doc.Students {
		st, e := store.Resolve(s.Client.Origin(), r.StudentRef)
		if e != nil {
			t.Fatal(e)
		}
		userOf[r.StudentRef] = st.UserID
	}

	if _, err = s.Post(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if maxInFlight < 2 {
		t.Fatalf("students were not posted concurrently: at most %d request in flight", maxInFlight)
	}
	saved := func() map[int64]string {
		var got domain.ResultsDocument
		if _, e := workspace.ReadDocument(path, &got); e != nil {
			t.Fatal(e)
		}
		out := map[int64]string{}
		for _, r := range got.Students {
			out[userOf[r.StudentRef]] = r.CommentOnly.Outcome.Status
			if r.CommentOnly.Outcome.Status == "succeeded" && r.CommentOnly.Outcome.PostedText != "Feedback for "+r.StudentRef {
				t.Fatalf("%s saved another student's text: %q", r.StudentRef, r.CommentOnly.Outcome.PostedText)
			}
		}
		return out
	}
	first := saved()
	for _, u := range users {
		want := "succeeded"
		switch u {
		case uncertain:
			want = "uncertain"
		case unreadable:
			want = "held"
		}
		if first[u] != want {
			t.Fatalf("user %d saved %q, want %q (all: %v)", u, first[u], want, first)
		}
	}
	mu.Lock()
	for _, u := range users {
		if want := map[bool]int{true: 0, false: 1}[u == unreadable]; puts[u] != want {
			t.Fatalf("user %d had %d comment writes on the first run, want %d", u, puts[u], want)
		}
	}
	mu.Unlock()

	if _, err = s.Post(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	second := saved()
	mu.Lock()
	defer mu.Unlock()
	for _, u := range users {
		if puts[u] != 1 {
			t.Fatalf("after the rerun user %d had %d comment writes, want exactly 1", u, puts[u])
		}
	}
	if second[unreadable] != "succeeded" || second[uncertain] != "uncertain" {
		t.Fatalf("rerun outcomes: %v", second)
	}
}
