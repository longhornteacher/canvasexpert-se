package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestPullSummaryCountsObservedStatesAndSeparatesFetch(t *testing.T) {
	live := fixtureSubmission(5)
	live.Late = true
	missing := fixtureSubmission(6)
	missing.SubmittedAt = nil
	excused := fixtureSubmission(7)
	excused.Excused = true
	inactive := fixtureSubmission(8)
	unsupported := fixtureSubmission(9)
	unsupported.Type = "online_url"
	var origin string
	s, server := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/files/unreadable":
			_, _ = w.Write([]byte{0, 1, 2})
		case strings.HasSuffix(r.URL.Path, "/enrollments"):
			var enrollments []Enrollment
			for id := int64(5); id <= 9; id++ {
				state := "active"
				if id == 8 {
					state = "inactive"
				}
				enrollments = append(enrollments, Enrollment{UserID: id, Type: "StudentEnrollment", State: state, User: CanvasUser{ID: id, Name: "Synthetic Example"}})
			}
			_ = json.NewEncoder(w).Encode(enrollments)
		case strings.HasSuffix(r.URL.Path, "/submissions"):
			_ = json.NewEncoder(w).Encode([]Submission{live, missing, excused, inactive, unsupported})
		case strings.Contains(r.URL.Path, "/submissions/5"):
			live.Attachments = []Attachment{{ID: 500, Filename: "response.pdf", URL: origin + "/files/unreadable", ContentType: "application/pdf"}}
			_ = json.NewEncoder(w).Encode(live)
		default:
			_ = json.NewEncoder(w).Encode(fixtureAssignment())
		}
	})
	origin = server.URL
	report, e := s.PullWork(context.Background(), "TEST", "10")
	if e != nil || len(report.Assignments) != 1 {
		t.Fatalf("pull: %#v %v", report, e)
	}
	summary := report.Assignments[0]
	if summary.Course != "TEST" || summary.AssignmentID != 10 || summary.Students != 1 || summary.Late != 1 || summary.Missing != 1 || summary.Excused != 1 || summary.Inactive != 1 || summary.Unsupported != 1 || summary.UnreadableAttachments != 1 || len(summary.Packets) != 1 {
		t.Fatalf("incorrect summary: %#v", summary)
	}
	if report.DiscoveryRequests != 3 || report.FetchRequests != 3 || report.Requests != report.DiscoveryRequests+report.FetchRequests || report.DiscoveryElapsed <= 0 || report.FetchElapsed <= 0 {
		t.Fatalf("incorrect phase metrics: %#v", report)
	}
	// No actionable work needs no detailed history, rubric refetch or downloads.
	live.Excused = true
	report, e = s.PullWork(context.Background(), "TEST", "10")
	if e != nil || report.FetchRequests != 0 || report.DiscoveryRequests != 3 || report.Students != 0 {
		t.Fatalf("unnecessary fetch for skipped work: %#v %v", report, e)
	}
}
