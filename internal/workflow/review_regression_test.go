package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestRegressionPartialUncertainGroupTiming(t *testing.T) {
	env := newGroupPostEnv(t, false)
	prepareGroupLateOnly(t, env)
	row := &env.doc.Groups[0]
	row.LateOnly.Outcome = domain.ActionOutcome{Status: "uncertain"}
	row.Representative = chooseRepresentative(*row)
	store, err := workspace.OpenStudents(env.s.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, student := range store.Rows {
		if student.Ref == row.Representative {
			env.timing[student.UserID] = map[string]any{"late": true, "late_policy_status": "late", "seconds_late": 3600}
		}
	}
	env.save(t)
	report, err := env.s.PostGroupResults(context.Background(), env.results)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("only representative matches; outcome=%s; writes=%d", report.Groups[0].Outcomes.Late.Status, len(env.puts))
	if report.Groups[0].Outcomes.Late.Status != "uncertain" || groupComplete(env.reload(t)) || len(env.puts) != 0 {
		t.Fatalf("mixed timing must remain uncertain without writes: %+v, puts=%v", report.Groups[0], env.puts)
	}
}

func TestRegressionFreshSharedTimingStillWritesWhenOnlyRepresentativeMatches(t *testing.T) {
	env := newGroupPostEnv(t, false)
	prepareGroupLateOnly(t, env)
	row := &env.doc.Groups[0]
	rep := chooseRepresentative(*row)
	store, err := workspace.OpenStudents(env.s.Root)
	if err != nil {
		t.Fatal(err)
	}
	student, err := store.Resolve(env.s.Client.Origin(), rep)
	if err != nil {
		t.Fatal(err)
	}
	// The representative already had the requested timing in the reviewed
	// baseline; the two other members still require this new shared action.
	baseline := row.LateOnly.ObservedMembers[rep]
	status := "late"
	baseline.Late, baseline.LatePolicyStatus, baseline.SecondsLate = true, &status, 3600
	row.LateOnly.ObservedMembers[rep] = baseline
	env.timing[student.UserID] = map[string]any{"late": true, "late_policy_status": "late", "seconds_late": 3600}
	var packet domain.GroupPacketDocument
	if _, err := workspace.ReadDocument(filepath.Join(env.s.Root, filepath.FromSlash(env.doc.Packet)), &packet); err != nil {
		t.Fatal(err)
	}
	workPath := filepath.Join(env.s.Root, filepath.FromSlash(packet.Groups[0].Path))
	var work domain.GroupWorkDocument
	if _, err := workspace.ReadDocument(workPath, &work); err != nil {
		t.Fatal(err)
	}
	for i := range work.Members {
		if work.Members[i].StudentRef == rep {
			work.Members[i].ObservedTiming = &baseline
		}
	}
	if err := workspace.WriteDocument(workPath, &work, ""); err != nil {
		t.Fatal(err)
	}
	env.save(t)
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	if len(env.puts) != 1 || env.reload(t).LateOnly.Outcome.Status != "succeeded" {
		t.Fatalf("new full-audience timing was suppressed by representative: %+v puts=%v", env.reload(t), env.puts)
	}
}

func TestRegressionAcademicGroupCompletionFollowsExactAttemptInLocalDraft(t *testing.T) {
	env := newGroupPostEnv(t, false)
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	next := filepath.Join(env.s.Root, "drafts", "accepted.yaml")
	if err := os.MkdirAll(filepath.Dir(next), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(env.results, next); err != nil {
		t.Fatal(err)
	}
	ref := env.doc.Groups[0].MemberRefs[0]
	if !env.s.completedHint(10, ref, Submission{Attempt: 1}, env.doc.Assignment) {
		t.Fatal("accepted shared grade lost from YAML outside courses")
	}
	if env.s.completedHint(10, ref, Submission{Attempt: 2}, env.doc.Assignment) {
		t.Fatal("accepted shared grade suppressed a new member attempt")
	}
}

func TestRegressionUnreleasedFeedbackWithoutScore(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-10-18T12:00:00-05:00")
	loc, _ := time.LoadLocation("America/Chicago")
	a := Assignment{ID: 1, Name: "Private feedback", Published: true, DueAt: str("2026-10-17T12:00:00-05:00"), Points: "10", SubmissionTypes: []string{"online_text_entry"}}
	var sub Submission
	if err := json.Unmarshal([]byte(`{"user_id":9,"assignment_id":1,"assignment_visible":true,"attempt":1,"submitted_at":"2026-10-17T11:00:00-05:00","score":null,"entered_score":null,"graded_at":null,"posted_at":null,"submission_comments":[{"author_id":77,"comment":"PRIVATE FEEDBACK"}]}`), &sub); err != nil {
		t.Fatal(err)
	}
	course := ParentReportCourse{WorkDue: []ParentWorkDueItem{}}
	buildStudentReportScoped(&course, []Submission{sub}, nil, map[int64]Assignment{1: a}, now, loc, now.AddDate(0, 0, -14), now.AddDate(0, 0, 7))
	if len(course.WorkDue) != 1 || course.WorkDue[0].Status != "awaiting_grading" || len(course.Corrections) != 1 {
		t.Fatalf("ungraded submission must retain its inventory and corrections availability: %+v", course)
	}
	if len(course.Corrections) > 0 && len(course.Corrections[0].Comments) > 0 {
		t.Fatalf("BUG: posted_at:null with no score exposed feedback: %+v", course.Corrections[0].Comments)
	}
	for _, posted := range []string{`,"posted_at":"2026-10-17T12:00:00-05:00"`, ""} {
		var released Submission
		if err := json.Unmarshal([]byte(`{"user_id":9,"assignment_id":1,"assignment_visible":true,"attempt":1,"submitted_at":"2026-10-17T11:00:00-05:00","score":null,"entered_score":null,"graded_at":null,"submission_comments":[{"author_id":77,"comment":"RELEASED FEEDBACK"}]`+posted+`}`), &released); err != nil {
			t.Fatal(err)
		}
		control := ParentReportCourse{}
		buildStudentReportScoped(&control, []Submission{released}, nil, map[int64]Assignment{1: a}, now, loc, now.AddDate(0, 0, -14), now.AddDate(0, 0, 7))
		if len(control.Corrections) != 1 || len(control.Corrections[0].Comments) != 1 {
			t.Fatalf("released or legacy omitted timestamp lost ordinary feedback: %+v", control)
		}
	}
}

func TestRegressionTimingProvenanceFileExtension(t *testing.T) {
	env := newGroupPostEnv(t, false)
	prepareGroupLateOnly(t, env)
	env.doc.Groups[0].LateOnly.Outcome = domain.ActionOutcome{Status: "succeeded"}
	env.save(t)
	var work domain.GroupWorkDocument
	var packet domain.GroupPacketDocument
	if _, err := workspace.ReadDocument(filepath.Join(env.s.Root, filepath.FromSlash(env.doc.Packet)), &packet); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.ReadDocument(filepath.Join(env.s.Root, filepath.FromSlash(packet.Groups[0].Path)), &work); err != nil {
		t.Fatal(err)
	}
	ref := work.Members[0].StudentRef
	current := domain.AttemptRef{Attempt: work.Members[0].Attempt, SubmittedAt: work.Members[0].SubmittedAt}
	if !env.s.acceptedTimingRecordExists(1, 10, ref, current) {
		t.Fatal("fixture .md not recognized")
	}
	previous := env.results
	for _, name := range []string{"results.yaml", "results.yml", "results.txt", "results.json", "results"} {
		next := filepath.Join(env.s.Root, "drafts", name)
		if err := os.MkdirAll(filepath.Dir(next), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(previous, next); err != nil {
			t.Fatal(err)
		}
		previous = next
		if !env.s.acceptedTimingRecordExists(1, 10, ref, current) {
			t.Fatalf("accepted timing lost from local draft %s", name)
		}
		if env.s.acceptedTimingRecordExists(2, 10, ref, current) || env.s.acceptedTimingRecordExists(1, 11, ref, current) || env.s.acceptedTimingRecordExists(1, 10, "other", current) || env.s.acceptedTimingRecordExists(1, 10, ref, domain.AttemptRef{Attempt: current.Attempt + 1}) {
			t.Fatalf("unrelated identity inherited timing provenance from %s", name)
		}
	}
}

func TestRegressionGroupPreparedSchoolTimePostsOnce(t *testing.T) {
	env := newGroupPostEnv(t, false)
	calendar := workspace.SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-01", EndDate: "2027-05-31"}
	if err := workspace.WriteTeacherDocument(env.s.Root, "context/school-calendar.md", calendar, "Synthetic calendar"); err != nil {
		t.Fatal(err)
	}
	var packet domain.GroupPacketDocument
	if _, err := workspace.ReadDocument(filepath.Join(env.s.Root, filepath.FromSlash(env.doc.Packet)), &packet); err != nil {
		t.Fatal(err)
	}
	var work domain.GroupWorkDocument
	if _, err := workspace.ReadDocument(filepath.Join(env.s.Root, filepath.FromSlash(packet.Groups[0].Path)), &work); err != nil {
		t.Fatal(err)
	}
	decision := LateTimingDecisionsDocument{Schema: 1, Kind: "late-timing-decisions", Packet: env.doc.Packet, Groups: []LateTimingGroupDecision{{GroupID: 60}}}
	for _, member := range work.Members {
		decision.Groups[0].Members = append(decision.Groups[0].Members, LateTimingStudentDecision{StudentRef: member.StudentRef, FirstGenuine: domain.AttemptRef{Attempt: member.Attempt, SubmittedAt: member.SubmittedAt}})
	}
	path := filepath.Join(env.s.Root, "courses", "TEST", "timing-decisions.md")
	if err := workspace.WriteDocument(path, &decision, ""); err != nil {
		t.Fatal(err)
	}
	prepared, err := env.s.PrepareLateTiming(mustRel(t, env.s.Root, path))
	if err != nil || len(prepared.Groups) != 1 || prepared.Groups[0].Status != "proposed" || prepared.Groups[0].Timing == nil || len(prepared.Groups[0].MemberSchoolTime) != 3 {
		t.Fatalf("group school-time preparation: %+v err=%v", prepared, err)
	}
	prepareGroupLateOnly(t, env)
	row := &env.doc.Groups[0]
	p := prepared.Groups[0]
	row.LateOnly = &domain.GroupLateOnly{LateTiming: *p.Timing, ObservedMembers: p.ObservedMembers, MemberSchoolTime: p.MemberSchoolTime}
	env.save(t)
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	if len(env.puts) != 1 || env.reload(t).LateOnly.Outcome.Status != "succeeded" {
		t.Fatalf("prepared group timing must use one write: %+v puts=%v", env.reload(t), env.puts)
	}
	fields := env.puts[0].body["submission"].(map[string]any)
	if fields["late_policy_status"] != p.Timing.Status || fields["seconds_late_override"] != float64(p.Timing.SecondsLate) {
		t.Fatalf("prepared timing was not posted honestly: %v", fields)
	}
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil || len(env.puts) != 1 {
		t.Fatalf("accepted prepared timing replayed: puts=%v err=%v", env.puts, err)
	}
}

func TestRegressionIndividualModeChangesBetweenRubricAndGrade(t *testing.T) {
	env := newGroupEnv(t, true, true)
	previous := env.server.Config.Handler
	flipped := false
	env.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		previous.ServeHTTP(w, r)
		if r.Method == http.MethodPut && !flipped {
			shared := false
			env.shared = &shared
			flipped = true
		}
	})
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	for _, p := range env.puts {
		if _, grade := p.body["submission"]; grade {
			t.Fatalf("individual grade sent after mode changed to shared: user=%d body=%v", p.user, p.body)
		}
	}
	row := env.reload(t)
	if len(env.puts) != 1 || row.Students[0].Grade.Outcomes.Rubric.Status != "succeeded" || row.Students[0].Grade.Outcomes.Grade.Status != "held" {
		t.Fatalf("accepted rubric must be retained and later writes held: puts=%v row=%+v", env.puts, row)
	}
}

func TestRegressionIndividualUncertainCommentIsNotReplayed(t *testing.T) {
	env := newGroupEnv(t, false, true)
	row := &env.doc.Groups[0]
	st := &row.Students[0]
	st.Operation = "comment-only"
	st.CommentOnly = &domain.CommentOnlyOperation{
		Attempt: st.Grade.Attempt, Components: st.Grade.Components, Feedback: st.Grade.Feedback,
	}
	st.Grade = nil
	row.Students = row.Students[:1]
	env.save(t)
	previous := env.server.Config.Handler
	env.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			http.Error(w, "unclear write", http.StatusInternalServerError)
			return
		}
		previous.ServeHTTP(w, r)
	})
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	if saved := env.reload(t); saved.Students[0].CommentOnly.Outcome.Status != "uncertain" {
		t.Fatalf("expected an uncertain first write, got %v", saved)
	}
	env.server.Config.Handler = previous
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	if len(env.puts) > 0 {
		t.Fatalf("uncertain comment replayed: %v", env.puts)
	}
	saved := env.reload(t)
	original := saved.Students[0].CommentOnly.Outcome.PostedText
	saved.Students[0].CommentOnly.Feedback = "Edited after the uncertain original write."
	env.doc.Groups[0] = saved
	store, err := workspace.OpenStudents(env.s.Root)
	if err != nil {
		t.Fatal(err)
	}
	student, err := store.Resolve(env.s.Client.Origin(), saved.Students[0].StudentRef)
	if err != nil {
		t.Fatal(err)
	}
	env.timing[student.UserID] = map[string]any{"submission_comments": []any{map[string]any{"id": 99, "attempt": 1, "comment": original}}}
	env.save(t)
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	out := env.reload(t).Students[0].CommentOnly.Outcome
	if len(env.puts) != 0 || out.Status != "preserved" || out.PostedText != original {
		t.Fatalf("original text should reconcile despite edited feedback: %+v puts=%v", out, env.puts)
	}
}

func TestRegressionCommentOnlyRecoveryUsesSavedAttemptAndText(t *testing.T) {
	for _, tc := range []struct {
		name, status, saved, comment string
		commentAttempt               int
		want                         string
		writes                       int
	}{
		{"absent", "uncertain", "Original", "", 2, "uncertain", 0},
		{"original present", "pending", "Original", "Original", 2, "preserved", 0},
		{"wrong attempt", "uncertain", "Original", "Original", 1, "uncertain", 0},
		{"legacy without text", "uncertain", "", "Edited", 2, "uncertain", 0},
		{"known rejection", "failed", "Original", "", 2, "succeeded", 1},
		{"accepted", "succeeded", "Original", "", 2, "succeeded", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes := 0
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				writes++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if len(body) != 1 || body["comment"] == nil {
					t.Errorf("non-comment payload: %v", body)
				}
				w.WriteHeader(http.StatusNoContent)
			})
			live := fixtureSubmission(5)
			live.Comments = []SubmissionComment{{ID: 88, Attempt: &tc.commentAttempt, Comment: tc.comment}}
			a, err := Snapshot(fixtureAssignment(), s.Client.Origin())
			if err != nil {
				t.Fatal(err)
			}
			doc := &domain.ResultsDocument{Assignment: a, Interpretation: domain.Interpretation{Mapping: "repeated", Rationale: "Observed response", Components: []domain.Component{{Key: "answer", Label: "Answer", Weight: domain.MustDecimal("1")}}}}
			row := &domain.StudentResult{Operation: "comment-only", CommentOnly: &domain.CommentOnlyOperation{Attempt: attemptRef(live), Feedback: "Edited", Components: []domain.ComponentScore{{Key: "answer", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "c1", Points: domain.MustDecimal("80")}}}}, Outcome: domain.ActionOutcome{Status: tc.status, PostedText: tc.saved}}}
			if err := s.postCommentOnly(context.Background(), doc, row, domain.ValidatedStudent{Interpretation: doc.Interpretation}, postSource{student: workspace.Student{UserID: 5}}, live, func() error { return nil }); err != nil {
				t.Fatal(err)
			}
			if writes != tc.writes || row.CommentOnly.Outcome.Status != tc.want {
				t.Fatalf("writes=%d outcome=%+v", writes, row.CommentOnly.Outcome)
			}
			if tc.writes == 0 && row.CommentOnly.Outcome.PostedText != tc.saved {
				t.Fatalf("original text replaced: %+v", row.CommentOnly.Outcome)
			}
		})
	}
}

func TestRegressionIncompleteLiveRepliesCannotConfirmMissingWork(t *testing.T) {
	env, packet, rel := newDiscussionEnv(t)
	path, _ := env.results(t, packet, rel, []string{"100", "60"})
	var doc domain.ResultsDocument
	if _, err := workspace.ReadDocument(path, &doc); err != nil {
		t.Fatal(err)
	}
	doc.Interpretation.Components = append(doc.Interpretation.Components, domain.Component{Key: "reply", Label: "Required reply", Weight: domain.MustDecimal("1")})
	for i := range doc.Students {
		doc.Students[i].Grade.Assessment = domain.Decimal{}
		doc.Students[i].Grade.Components = append(doc.Students[i].Grade.Components, domain.ComponentScore{Key: "reply", Evidence: "missing", Criteria: []domain.CriterionScore{{CriterionID: "c1", Points: domain.MustDecimal("0")}}})
	}
	if err := workspace.WriteDocument(path, &doc, ""); err != nil {
		t.Fatal(err)
	}
	previous := env.server.Config.Handler
	env.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/view") {
			http.Error(w, "unavailable", http.StatusForbidden)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/entries") {
			w.Header().Set("Content-Type", "application/json")
			recent := []any{}
			for n := 0; n < 10; n++ {
				recent = append(recent, map[string]any{"id": 600 + n, "user_id": 6, "message": "Recent reply", "created_at": "2026-10-10T12:00:00Z"})
			}
			_ = json.NewEncoder(w).Encode([]any{
				map[string]any{"id": 501, "user_id": 5, "message": "<p>The first student's claim</p>", "created_at": "2026-10-10T10:00:00Z", "updated_at": env.updatedAt[501]},
				map[string]any{"id": 502, "user_id": 6, "message": "<p>The second student replies</p>", "created_at": "2026-10-10T11:00:00Z", "updated_at": env.updatedAt[502], "has_more_replies": true, "recent_replies": recent},
			})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/replies") {
			http.Error(w, "unavailable", http.StatusForbidden)
			return
		}
		previous.ServeHTTP(w, r)
	})
	if _, err := env.s.Post(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	for _, p := range env.puts {
		if _, grade := p.body["submission"]; grade {
			t.Fatalf("missing-response score posted despite incomplete live replies: %v", p.body)
		}
	}
}

func TestRegressionDiscussionCoverageHoldsOnlyTheIncompleteClub(t *testing.T) {
	for _, incompleteAtPull := range []bool{false, true} {
		t.Run(fmt.Sprintf("incomplete-at-pull-%t", incompleteAtPull), func(t *testing.T) {
			env, _, _ := newDiscussionEnv(t)
			previous := env.server.Config.Handler
			incomplete := incompleteAtPull
			env.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				write := func(value any) { _ = json.NewEncoder(w).Encode(value) }
				switch r.URL.Path {
				case "/api/v1/courses/1/discussion_topics/77":
					write(map[string]any{"id": 77, "assignment_id": 10, "group_category_id": 50, "group_topic_children": []any{map[string]any{"group_id": 60, "id": 701}, map[string]any{"group_id": 61, "id": 702}}})
				case "/api/v1/group_categories/50":
					write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "non_collaborative": false})
				case "/api/v1/group_categories/50/groups":
					write([]any{map[string]any{"id": 60, "course_id": 1, "context_type": "Course", "group_category_id": 50}, map[string]any{"id": 61, "course_id": 1, "context_type": "Course", "group_category_id": 50}})
				case "/api/v1/groups/60/discussion_topics/701":
					write(map[string]any{"id": 701})
				case "/api/v1/groups/61/discussion_topics/702":
					write(map[string]any{"id": 702})
				case "/api/v1/groups/60/discussion_topics/701/view", "/api/v1/groups/60/discussion_topics/701/entries":
					if incomplete && strings.HasSuffix(r.URL.Path, "/view") {
						http.Error(w, "unavailable", http.StatusForbidden)
						return
					}
					entries := []any{map[string]any{"id": 501, "user_id": 5, "message": "Claim in first club", "created_at": "2026-10-10T10:00:00Z", "has_more_replies": incomplete}}
					if strings.HasSuffix(r.URL.Path, "/view") {
						write(map[string]any{"view": entries})
					} else {
						write(entries)
					}
				case "/api/v1/groups/60/discussion_topics/701/entries/501/replies":
					http.Error(w, "required reply page unavailable", http.StatusForbidden)
				case "/api/v1/groups/61/discussion_topics/702/view":
					write(map[string]any{"view": []any{map[string]any{"id": 502, "user_id": 6, "message": "Claim in independent club", "created_at": "2026-10-10T11:00:00Z"}}})
				default:
					previous.ServeHTTP(w, r)
				}
			})
			pulled, err := env.s.PullWork(context.Background(), "TEST", "10")
			if err != nil || len(pulled.Packets) != 1 {
				t.Fatalf("pull=%+v err=%v", pulled, err)
			}
			var packet domain.PacketDocument
			if _, err := workspace.ReadDocument(pulled.Packets[0], &packet); err != nil {
				t.Fatal(err)
			}
			path, _ := env.results(t, packet, mustRel(t, env.s.Root, pulled.Packets[0]), []string{"100", "60"})
			var doc domain.ResultsDocument
			if _, err := workspace.ReadDocument(path, &doc); err != nil {
				t.Fatal(err)
			}
			doc.Interpretation.Components = append(doc.Interpretation.Components, domain.Component{Key: "reply", Label: "Reply", Weight: domain.MustDecimal("1")})
			for i := range doc.Students {
				doc.Students[i].Grade.Assessment = domain.Decimal{}
				doc.Students[i].Grade.Components = append(doc.Students[i].Grade.Components, domain.ComponentScore{Key: "reply", Evidence: "missing", Criteria: []domain.CriterionScore{{CriterionID: "c1", Points: domain.MustDecimal("0")}}})
			}
			if err := workspace.WriteDocument(path, &doc, ""); err != nil {
				t.Fatal(err)
			}
			incomplete = true
			if _, err := env.s.Post(context.Background(), path); err != nil {
				t.Fatal(err)
			}
			independent := false
			for _, put := range env.puts {
				if put.user == 5 {
					t.Fatalf("incomplete club posted unsupported missing assessment: %+v", put)
				}
				if put.user == 6 {
					independent = true
				}
			}
			if !independent {
				t.Fatal("complete independent club was held by another source's coverage")
			}
		})
	}
}

func TestRegressionCitedContextCannotNarrowUnattributedStudentsCoverage(t *testing.T) {
	context := domain.DiscussionEntry{ContextType: "Group", ContextID: 60, TopicID: 701, EntryID: 1, AuthorKind: "student", StudentRef: "other"}
	p := domain.DiscussionPacket{Status: "partial", Sources: []domain.DiscussionSource{{ContextType: "Group", ContextID: 60, TopicID: 701, Status: "complete"}, {ContextType: "Group", ContextID: 61, TopicID: 702, Status: "partial", Reason: "unread replies"}}, Entries: []domain.DiscussionEntry{context}}
	keys := discussionRelevantSources(p, "unattributed-student", []string{domain.DiscussionEntryRef(context)})
	if status, _ := discussionCoverage(p, keys); status == "complete" {
		t.Fatal("citing another student's complete club cannot establish an unattributed student's absence")
	}
}

func TestRegressionSetRenamePreservesUnchangedSettings(t *testing.T) {
	var patch map[string]any
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut {
			_ = json.NewDecoder(r.Body).Decode(&patch)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "name": "Clubs", "self_signup": "enabled", "non_collaborative": false})
	})
	doc := domain.GroupSetDocument{Schema: 1, Kind: "group-set", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, Name: "Renamed clubs",
		SelfSignup: domain.OptionalString{Set: true, Value: "disabled"},
		Observed:   &domain.GroupSetSnapshot{Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, Name: "Clubs", SelfSignup: "disabled", AutoLeader: "none"},
	}
	path := filepath.Join(s.Root, "courses", "TEST", "set.md")
	if err := workspace.WriteDocument(path, &doc, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PushGroupSet(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if _, changed := patch["self_signup"]; changed {
		t.Fatalf("rename overwrote unrelated live self-signup setting: %v", patch)
	}
}

func TestRegressionGroupSetPatchUsesOnlyRequestedDifferences(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		change      func(*domain.GroupSetDocument)
		liveChange  bool
		want        any
		writes      int
	}{
		{"unchanged draft", "", func(d *domain.GroupSetDocument) {}, true, nil, 0},
		{"rename with unrelated live settings", "name", func(d *domain.GroupSetDocument) { d.Name = "Renamed" }, true, "Renamed", 1},
		{"signup clear", "self_signup", func(d *domain.GroupSetDocument) { d.SelfSignup = domain.OptionalString{Set: true, Null: true} }, false, nil, 1},
		{"limit clear", "group_limit", func(d *domain.GroupSetDocument) { d.GroupLimit = domain.OptionalInt{Set: true, Null: true} }, false, nil, 1},
		{"leader clear", "auto_leader", func(d *domain.GroupSetDocument) { d.AutoLeader = domain.OptionalString{Set: true, Null: true} }, false, nil, 1},
		{"signup change", "self_signup", func(d *domain.GroupSetDocument) { d.SelfSignup.Value = "restricted" }, false, "restricted", 1},
		{"limit change", "group_limit", func(d *domain.GroupSetDocument) { d.GroupLimit.Value = 5 }, false, float64(5), 1},
		{"leader change", "auto_leader", func(d *domain.GroupSetDocument) { d.AutoLeader.Value = "random" }, false, "random", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var patches []map[string]any
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPut {
					var patch map[string]any
					if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
						t.Error(err)
					}
					patches = append(patches, patch)
				}
				live := map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "name": "Clubs", "self_signup": "enabled", "group_limit": 4, "auto_leader": "first", "non_collaborative": false}
				if tc.name == "signup clear" {
					live["group_limit"] = nil
				}
				if tc.liveChange {
					live["self_signup"], live["group_limit"], live["auto_leader"] = "restricted", 8, "random"
				}
				_ = json.NewEncoder(w).Encode(live)
			})
			limit := 4
			doc := domain.GroupSetDocument{Schema: 1, Kind: "group-set", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, Name: "Clubs", SelfSignup: domain.OptionalString{Set: true, Value: "enabled"}, GroupLimit: domain.OptionalInt{Set: true, Value: 4}, AutoLeader: domain.OptionalString{Set: true, Value: "first"}, Observed: &domain.GroupSetSnapshot{Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, Name: "Clubs", SelfSignup: "enabled", GroupLimit: &limit, AutoLeader: "first"}}
			if tc.name == "signup clear" {
				doc.GroupLimit, doc.Observed.GroupLimit = domain.OptionalInt{Set: true, Null: true}, nil
			}
			tc.change(&doc)
			path := filepath.Join(s.Root, "courses", "TEST", "set.md")
			if err := workspace.WriteDocument(path, &doc, ""); err != nil {
				t.Fatal(err)
			}
			if _, err := s.PushGroupSet(context.Background(), path); err != nil {
				t.Fatal(err)
			}
			if len(patches) != tc.writes {
				t.Fatalf("patches=%v", patches)
			}
			if tc.writes > 0 && (len(patches[0]) != 1 || patches[0][tc.field] != tc.want) {
				t.Fatalf("unexpected fields: %v", patches)
			}
		})
	}
}

func TestRegressionFallbackIncludesDocumentedRecentReplies(t *testing.T) {
	env, _, _ := newDiscussionEnv(t)
	previous := env.server.Config.Handler
	env.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/view") {
			http.Error(w, "unavailable", http.StatusForbidden)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/entries") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]any{
				map[string]any{"id": 501, "user_id": 5, "message": "Root", "recent_replies": []any{map[string]any{"id": 503, "user_id": 6, "message": "Actual reply", "created_at": "2026-10-10T12:00:00Z"}}, "has_more_replies": false},
			})
			return
		}
		previous.ServeHTTP(w, r)
	})
	packet, err := env.s.PullDiscussionEvidence(context.Background(), "TEST", NativeDiscussion{ID: 77, AssignmentID: 10}, env.s.Config.Courses[0], 0, 0, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Entries) != 2 {
		t.Fatalf("documented fallback silently dropped reply: status=%s entries=%v", packet.Status, packet.Entries)
	}
	root, reply := packet.Entries[0], packet.Entries[1]
	if root.EntryID != 501 || reply.EntryID != 503 || reply.ParentEntryID != 501 || root.StudentRef == "" || reply.StudentRef == "" || root.StudentRef == reply.StudentRef || reply.AuthorKind != "student" || reply.ContextType != "Course" || reply.ContextID != 1 || reply.TopicID != 77 || packet.Status != "complete" {
		t.Fatalf("fallback lost source-qualified reply attribution: %+v", packet)
	}
}

func TestRegressionUnsubmittedPlaceholderDoesNotBlockModeEdit(t *testing.T) {
	a := fixtureAssignment()
	setID, mode := int64(50), false
	a.GroupCategoryID, a.GradeGroupStudentsIndividually = &setID, &mode
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/group_categories/50":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "non_collaborative": false})
		case "/api/v1/courses/1/assignments/10/submissions":
			_ = json.NewEncoder(w).Encode([]any{map[string]any{"user_id": 5, "assignment_id": 10, "attempt": 0, "submitted_at": nil, "workflow_state": "unsubmitted"}})
		case "/api/v1/courses/1/assignments/10":
			if r.Method == http.MethodPut {
				writes++
				individual := true
				a.GradeGroupStudentsIndividually = &individual
			}
			_ = json.NewEncoder(w).Encode(a)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	path, err := s.PullItem(context.Background(), "TEST", "10")
	if err != nil {
		t.Fatal(err)
	}
	var doc domain.AssignmentDocument
	body, err := workspace.ReadDocument(path, &doc)
	if err != nil {
		t.Fatal(err)
	}
	individual := true
	doc.GradeGroupStudentsIndividually = &individual
	if err := workspace.WriteDocument(path, &doc, body); err != nil {
		t.Fatal(err)
	}
	report, err := s.Push(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if writes == 0 {
		t.Fatalf("mode edit blocked by unsubmitted placeholder: status=%s message=%s", report.Status, report.Message)
	}
}

func TestRegressionSubmittedWorkOnLaterPageHoldsOnlyStructure(t *testing.T) {
	a := fixtureAssignment()
	setID, mode := int64(50), false
	a.GroupCategoryID, a.GradeGroupStudentsIndividually = &setID, &mode
	var patch map[string]any
	pages := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/group_categories/50":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "non_collaborative": false})
		case "/api/v1/courses/1/assignments/10/submissions":
			pages++
			if r.URL.Query().Get("page") == "2" {
				_, _ = w.Write([]byte(`[{"user_id":6,"assignment_id":10,"attempt":1,"submitted_at":"2026-10-16T18:00:00Z","workflow_state":"submitted"}]`))
			} else {
				w.Header().Set("Link", fmt.Sprintf(`<https://%s%s?page=2>; rel="next"`, r.Host, r.URL.Path))
				_, _ = w.Write([]byte(`[{"user_id":5,"assignment_id":10,"attempt":0,"submitted_at":null,"workflow_state":"unsubmitted"}]`))
			}
		case "/api/v1/courses/1/assignments/10":
			if r.Method == http.MethodPut {
				var payload map[string]map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				patch = payload["assignment"]
				if title, ok := patch["name"].(string); ok {
					a.Name = title
				}
			}
			_ = json.NewEncoder(w).Encode(a)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	path, err := s.PullItem(context.Background(), "TEST", "10")
	if err != nil {
		t.Fatal(err)
	}
	var doc domain.AssignmentDocument
	body, err := workspace.ReadDocument(path, &doc)
	if err != nil {
		t.Fatal(err)
	}
	doc.Title = "Updated title"
	doc.GradeGroupStudentsIndividually = boolPtr(true)
	if err := workspace.WriteDocument(path, &doc, body); err != nil {
		t.Fatal(err)
	}
	report, err := s.Push(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 || report.Status != "partial" || len(patch) != 1 || patch["name"] != "Updated title" || *a.GradeGroupStudentsIndividually {
		t.Fatalf("later submitted work must protect structure and allow title: pages=%d report=%+v patch=%v", pages, report, patch)
	}
}
