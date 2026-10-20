package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// pageAudienceFixture is a stateful course Page with date-details overrides.
type pageAudienceFixture struct {
	Service *Service
	Page    NativePage
	Dates   pageDateDetails
	Writes  []string
	// Members is the live club membership the handler reports.
	Members map[int64][]int64
	// BodyPuts counts content updates; an audience-only change must not send one.
	BodyPuts int
	// FailDates makes the collection write fail with the given status.
	FailDates int
}

func newPageAudienceFixture(t *testing.T, members map[int64][]int64) *pageAudienceFixture {
	t.Helper()
	f := &pageAudienceFixture{Page: NativePage{PageID: 7, Slug: "ghost-calendar", Title: "Ghost calendar", Body: "<p>Calendar</p>"}, Dates: pageDateDetails{ID: 7, OnlyVisibleToOverrides: boolPtr(false), VisibleToEveryone: boolPtr(true), Overrides: []pageDateOverride{}}, Members: members}
	f.Service, _ = workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch {
		case r.URL.Path == "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "name": "Book Club"})
		case strings.HasPrefix(r.URL.Path, "/api/v1/groups/") && strings.HasSuffix(r.URL.Path, "/memberships"):
			id := int64(60)
			if strings.Contains(r.URL.Path, "/61/") {
				id = 61
			}
			rows := []any{}
			for _, user := range f.Members[id] {
				rows = append(rows, map[string]any{"user_id": user, "group_id": id, "workflow_state": "accepted"})
			}
			write(rows)
		case strings.HasPrefix(r.URL.Path, "/api/v1/groups/"):
			id := int64(60)
			if strings.Contains(r.URL.Path, "/61") {
				id = 61
			}
			write(map[string]any{"id": id, "course_id": 1, "group_category_id": 50, "context_type": "Course", "name": "Ghost"})
		case r.URL.Path == "/api/v1/courses/1/enrollments":
			rows := []any{}
			for _, users := range f.Members {
				for _, user := range users {
					rows = append(rows, map[string]any{"user_id": user, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": user, "name": "Private"}})
				}
			}
			write(rows)
		case r.URL.Path == "/api/v1/courses/1/pages/page_id:7/date_details":
			if r.Method == http.MethodPut {
				f.Writes = append(f.Writes, "dates")
				if f.FailDates > 0 {
					w.WriteHeader(f.FailDates)
					return
				}
				var payload struct {
					OnlyVisible *bool            `json:"only_visible_to_overrides"`
					UnlockAt    *string          `json:"unlock_at"`
					LockAt      *string          `json:"lock_at"`
					Overrides   []map[string]any `json:"assignment_overrides"`
				}
				_ = json.NewDecoder(r.Body).Decode(&payload)
				f.Dates.OnlyVisibleToOverrides = payload.OnlyVisible
				f.Dates.VisibleToEveryone = boolPtr(payload.OnlyVisible == nil || !*payload.OnlyVisible)
				f.Dates.UnlockAt, f.Dates.LockAt = payload.UnlockAt, payload.LockAt
				next := []pageDateOverride{}
				for _, row := range payload.Overrides {
					out := pageDateOverride{}
					if v, ok := row["id"].(float64); ok {
						out.ID = int64(v)
					}
					if v, ok := row["title"].(string); ok {
						out.Title = v
					}
					if v, ok := row["group_id"].(float64); ok {
						out.GroupID = int64(v)
					}
					if v, ok := row["course_section_id"].(float64); ok {
						out.CourseSectionID = int64(v)
					}
					if v, ok := row["unlock_at"].(string); ok {
						out.UnlockAt = &v
					}
					if v, ok := row["lock_at"].(string); ok {
						out.LockAt = &v
					}
					if ids, ok := row["student_ids"].([]any); ok {
						for _, id := range ids {
							out.StudentIDs = append(out.StudentIDs, int64(id.(float64)))
						}
					}
					next = append(next, out)
				}
				f.Dates.Overrides = next
				w.WriteHeader(http.StatusNoContent)
				return
			}
			write(f.Dates)
		case r.URL.Path == "/api/v1/courses/1/pages/page_id:7":
			if r.Method == http.MethodPut {
				f.Writes = append(f.Writes, "page")
				f.BodyPuts++
				var payload struct {
					Page map[string]any `json:"wiki_page"`
				}
				_ = json.NewDecoder(r.Body).Decode(&payload)
				if v, ok := payload.Page["body"].(string); ok {
					f.Page.Body = v
				}
				if v, ok := payload.Page["title"].(string); ok {
					f.Page.Title = v
				}
				if v, ok := payload.Page["published"].(bool); ok {
					f.Page.Published = v
				}
			}
			write(f.Page)
		case r.URL.Path == "/api/v1/courses/1/pages":
			f.Writes = append(f.Writes, "create")
			var payload struct {
				Page struct {
					Title     string `json:"title"`
					Body      string `json:"body"`
					Published bool   `json:"published"`
				} `json:"wiki_page"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload.Page.Published {
				t.Error("audience Page was created published")
			}
			f.Page = NativePage{PageID: 7, Slug: "ghost-calendar", Title: payload.Page.Title, Body: payload.Page.Body}
			write(f.Page)
		case r.URL.Path == "/api/v1/courses/1/modules/4/items/8":
			write(map[string]any{"id": 8, "type": "Page", "page_url": f.Page.Slug})
		case strings.HasPrefix(r.URL.Path, "/api/v1/courses/1/modules/4/items"):
			f.Writes = append(f.Writes, "module")
			write(map[string]any{"id": 8, "type": "Page", "page_url": f.Page.Slug})
		case r.URL.Path == "/api/v1/courses/1/modules/4":
			write(map[string]any{"id": 4})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	return f
}

func (f *pageAudienceFixture) seedStudents(t *testing.T, users ...int64) {
	t.Helper()
	store, err := workspace.OpenStudents(f.Service.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		if _, err = store.Upsert(f.Service.Client.Origin(), user, "Private", "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
}

func (f *pageAudienceFixture) ref(t *testing.T, user int64) string {
	t.Helper()
	store, err := workspace.OpenStudents(f.Service.Root)
	if err != nil {
		t.Fatal(err)
	}
	student, err := store.Resolve(f.Service.Client.Origin(), "")
	_ = student
	for _, row := range store.Rows {
		if row.UserID == user {
			return row.Ref
		}
	}
	t.Fatalf("no ref for %d (%v)", user, err)
	return ""
}

func pageAudienceDraft() domain.PageDocument {
	return domain.PageDocument{Schema: 1, Kind: "page", Operation: "create", Course: "TEST", Title: "Ghost calendar", BodyFormat: "html", Audience: &domain.Audience{Mode: "individual", Policy: "snapshot", Source: domain.AudienceSource{GroupCategoryID: 50, GroupIDs: []int64{60}}}}
}

func TestPageAudienceCreateRestrictsBeforePublicationAndPreservesRows(t *testing.T) {
	f := newPageAudienceFixture(t, map[int64][]int64{60: {9001, 9002}})
	f.seedStudents(t, 9001, 9002, 9003)
	// An unrelated section row and availability dates must survive replacement.
	unlock := "2026-10-20T12:00:00Z"
	f.Dates.UnlockAt = &unlock
	f.Dates.Overrides = []pageDateOverride{{ID: 90, Title: "Section A", CourseSectionID: 5, LockAt: &unlock}}
	path := filepath.Join(f.Service.Root, "courses", "TEST", "drafts", "ghost.md")
	d := pageAudienceDraft()
	d.Published = true
	d.ModuleID = 4
	if err := workspace.WriteDocument(path, &d, "<p>Calendar</p>"); err != nil {
		t.Fatal(err)
	}
	state, err := f.Service.PrepareAudience(context.Background(), "TEST", path, false)
	if err != nil || len(state.StudentRefs) != 2 || !state.Complete {
		t.Fatalf("prepare %+v %v", state, err)
	}
	review, err := BuildReview(f.Service.Root, path)
	if err != nil || len(review.Cards) != 1 || review.Cards[0].Hold != "" {
		t.Fatalf("review %+v %v", review, err)
	}
	report, err := f.Service.PushPage(context.Background(), path)
	if err != nil || report.Status != "completed" {
		t.Fatalf("push %+v %v", report, err)
	}
	if strings.Join(f.Writes, ",") != "create,dates,module,page" {
		t.Fatalf("audience must precede placement and publication: %v", f.Writes)
	}
	if f.Dates.OnlyVisibleToOverrides == nil || !*f.Dates.OnlyVisibleToOverrides {
		t.Fatal("Page was not restricted to overrides")
	}
	if len(f.Dates.Overrides) != 2 {
		t.Fatalf("expected preserved section row plus managed row: %+v", f.Dates.Overrides)
	}
	if f.Dates.Overrides[0].ID != 90 || f.Dates.Overrides[0].CourseSectionID != 5 {
		t.Fatalf("unrelated row was not preserved: %+v", f.Dates.Overrides[0])
	}
	if f.Dates.UnlockAt == nil || *f.Dates.UnlockAt != unlock {
		t.Fatal("availability date was not preserved")
	}
	if !f.Page.Published {
		t.Fatal("Page was not published after the accepted restriction")
	}
}

func TestPageAudienceFailureNeverPublishes(t *testing.T) {
	f := newPageAudienceFixture(t, map[int64][]int64{60: {9001}})
	f.seedStudents(t, 9001)
	f.FailDates = http.StatusForbidden
	path := filepath.Join(f.Service.Root, "courses", "TEST", "drafts", "ghost.md")
	d := pageAudienceDraft()
	d.Published = true
	if err := workspace.WriteDocument(path, &d, "<p>Calendar</p>"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Service.PrepareAudience(context.Background(), "TEST", path, false); err != nil {
		t.Fatal(err)
	}
	report, err := f.Service.PushPage(context.Background(), path)
	if err != nil || report.Status != "held" {
		t.Fatalf("push %+v %v", report, err)
	}
	for _, write := range f.Writes {
		if write == "page" {
			t.Fatal("Page was published despite a refused audience")
		}
	}
	var saved domain.PageDocument
	if _, err = workspace.ReadDocument(path, &saved); err != nil || saved.Created == nil || saved.Created.PageID != 7 {
		t.Fatalf("Page identity was not retained: %+v %v", saved.Created, err)
	}
	if saved.AudienceOutcome.Status != "failed" {
		t.Fatalf("refused audience outcome not saved: %+v", saved.AudienceOutcome)
	}
}

func TestPageAudienceOnlyUpdateSendsNoBodyPut(t *testing.T) {
	f := newPageAudienceFixture(t, map[int64][]int64{60: {9001}})
	f.seedStudents(t, 9001)
	path, err := f.Service.PullPage(context.Background(), "TEST", "7")
	if err != nil {
		t.Fatal(err)
	}
	var d domain.PageDocument
	body, err := workspace.ReadDocument(path, &d)
	if err != nil {
		t.Fatal(err)
	}
	if d.ObservedDates == nil {
		t.Fatal("pull did not record the date-details baseline")
	}
	// The teacher edits the live body after the pull.
	f.Page.Body = "<p>Teacher live edit</p>"
	d.Audience = pageAudienceDraft().Audience
	if err = workspace.WriteDocument(path, &d, body); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.PrepareAudience(context.Background(), "TEST", path, false); err != nil {
		t.Fatal(err)
	}
	report, err := f.Service.PushPage(context.Background(), path)
	if err != nil || report.Status != "completed" {
		t.Fatalf("push %+v %v", report, err)
	}
	if f.BodyPuts != 0 {
		t.Fatalf("audience-only update sent %d body PUTs", f.BodyPuts)
	}
	if f.Page.Body != "<p>Teacher live edit</p>" {
		t.Fatal("teacher's live body edit was overwritten")
	}
	if f.Dates.OnlyVisibleToOverrides == nil || !*f.Dates.OnlyVisibleToOverrides {
		t.Fatal("audience was not applied")
	}
}

func TestPageAudienceRefreshAddsAndRemovesWithoutTouchingManualRows(t *testing.T) {
	f := newPageAudienceFixture(t, map[int64][]int64{60: {9001, 9002}})
	f.seedStudents(t, 9001, 9002, 9003)
	path := filepath.Join(f.Service.Root, "courses", "TEST", "drafts", "ghost.md")
	d := pageAudienceDraft()
	if err := workspace.WriteDocument(path, &d, "<p>Calendar</p>"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Service.PrepareAudience(context.Background(), "TEST", path, false); err != nil {
		t.Fatal(err)
	}
	if report, err := f.Service.PushPage(context.Background(), path); err != nil || report.Status != "completed" {
		t.Fatalf("initial push %+v %v", report, err)
	}
	// A teacher adds a manual student exception after the accepted restriction.
	f.Dates.Overrides = append(f.Dates.Overrides, pageDateOverride{ID: 91, Title: "Manual", StudentIDs: []int64{9003}})
	// Membership changes: 9001 leaves, 9003 joins the club.
	f.Members[60] = []int64{9002, 9003}
	state, err := f.Service.PrepareAudience(context.Background(), "TEST", path, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Refresh.Add) != 0 || len(state.Refresh.Remove) != 1 {
		t.Fatalf("refresh plan %+v", state.Refresh)
	}
	action := filepath.Join(f.Service.Root, filepath.FromSlash(state.PreparedPath))
	report, err := f.Service.PushPage(context.Background(), action)
	if err != nil || report.Status != "completed" {
		t.Fatalf("refresh push %+v %v", report, err)
	}
	if len(f.Dates.Overrides) != 2 {
		t.Fatalf("manual row was not preserved: %+v", f.Dates.Overrides)
	}
	found := false
	for _, row := range f.Dates.Overrides {
		if row.ID == 91 && len(row.StudentIDs) == 1 && row.StudentIDs[0] == 9003 {
			found = true
		}
	}
	if !found {
		t.Fatalf("manual exception changed: %+v", f.Dates.Overrides)
	}
}

func TestPageAudienceRefreshHoldsChangedManagedRow(t *testing.T) {
	f := newPageAudienceFixture(t, map[int64][]int64{60: {9001, 9002}})
	f.seedStudents(t, 9001, 9002)
	path := filepath.Join(f.Service.Root, "courses", "TEST", "drafts", "ghost.md")
	d := pageAudienceDraft()
	if err := workspace.WriteDocument(path, &d, "<p>Calendar</p>"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Service.PrepareAudience(context.Background(), "TEST", path, false); err != nil {
		t.Fatal(err)
	}
	if report, err := f.Service.PushPage(context.Background(), path); err != nil || report.Status != "completed" {
		t.Fatalf("initial push %+v %v", report, err)
	}
	// The teacher edits the managed row in Canvas.
	f.Dates.Overrides[0].StudentIDs = []int64{9001}
	state, err := f.Service.PrepareAudience(context.Background(), "TEST", path, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Refresh.Unknown) == 0 {
		t.Fatalf("changed managed row was not held: %+v", state.Refresh)
	}
	action := filepath.Join(f.Service.Root, filepath.FromSlash(state.PreparedPath))
	report, err := f.Service.PushPage(context.Background(), action)
	if err != nil || report.Status != "held" {
		t.Fatalf("refresh push %+v %v", report, err)
	}
	if len(f.Dates.Overrides) != 1 || len(f.Dates.Overrides[0].StudentIDs) != 1 || f.Dates.Overrides[0].StudentIDs[0] != 9001 {
		t.Fatalf("teacher edit was overwritten: %+v", f.Dates.Overrides)
	}
}

func TestPageAudienceRejectsGroupPage(t *testing.T) {
	d := pageAudienceDraft()
	d.GroupID = 70
	if err := d.ValidateBody("<p>Body</p>"); err == nil {
		t.Fatal("group Page accepted an individual audience")
	}
}
