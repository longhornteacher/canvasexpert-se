package workflow

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// pageDateDetails is the complete date-details response for one course Page.
// Pages have availability dates and no graded due date.
type pageDateDetails struct {
	ID                     int64  `json:"id"`
	OnlyVisibleToOverrides *bool  `json:"only_visible_to_overrides"`
	VisibleToEveryone      *bool  `json:"visible_to_everyone"`
	UnlockAt               *string `json:"unlock_at"`
	LockAt                 *string `json:"lock_at"`
	Overrides              []pageDateOverride `json:"overrides"`
}

type pageDateOverride struct {
	ID              int64   `json:"id"`
	Title           string  `json:"title"`
	GroupID         int64   `json:"group_id"`
	CourseSectionID int64   `json:"course_section_id"`
	StudentIDs      []int64 `json:"student_ids"`
	UnlockAt        *string `json:"unlock_at"`
	LockAt          *string `json:"lock_at"`
}

func pageDateDetailsPath(courseID, pageID int64) string {
	// Force page_id:ID; a bare number is interpreted as a slug first.
	return nativePagePath(courseID, pageID) + "/date_details"
}

// readPageDates reads the scalar envelope and then the complete paginated
// override collection. A partial read is an error: a collection write replaces
// every row, so an incomplete baseline could delete unrelated access.
func (s *Service) readPageDates(ctx context.Context, courseID, pageID int64) (pageDateDetails, error) {
	var d pageDateDetails
	if err := s.Client.Get(ctx, pageDateDetailsPath(courseID, pageID), &d); err != nil {
		return d, err
	}
	if d.OnlyVisibleToOverrides == nil {
		return d, fmt.Errorf("Canvas did not report who can see Page %d", pageID)
	}
	var overrides []pageDateOverride
	if err := s.Client.ListField(ctx, pageDateDetailsPath(courseID, pageID), "overrides", &overrides); err != nil {
		return d, fmt.Errorf("Page %d override collection could not be read completely: %w", pageID, err)
	}
	d.Overrides = overrides
	return d, nil
}

// pageDatesSnapshot converts a live read into the document baseline. Student
// targets are mapped through the existing private store; an unmapped student
// makes the baseline incomplete rather than silently dropping access.
func (s *Service) pageDatesSnapshot(d pageDateDetails, store *workspace.StudentStore) (domain.PageDatesSnapshot, error) {
	snap := domain.PageDatesSnapshot{OnlyVisibleToOverrides: d.OnlyVisibleToOverrides, VisibleToEveryone: d.VisibleToEveryone, UnlockAt: d.UnlockAt, LockAt: d.LockAt, Overrides: []domain.PageOverrideRow{}}
	for _, row := range d.Overrides {
		if row.ID < 0 {
			return snap, fmt.Errorf("Page override with a negative ID cannot be preserved")
		}
		out := domain.PageOverrideRow{ID: row.ID, Title: row.Title, GroupID: row.GroupID, CourseSectionID: row.CourseSectionID, UnlockAt: row.UnlockAt, LockAt: row.LockAt}
		if len(row.StudentIDs) > 0 {
			for _, id := range row.StudentIDs {
				ref := ""
				for _, student := range store.Rows {
					if student.Instance == s.Client.Origin() && student.UserID == id {
						ref = student.Ref
						break
					}
				}
				if ref == "" {
					return snap, fmt.Errorf("Page override %d targets a student outside the private mapping; its access cannot be preserved", row.ID)
				}
				out.StudentRefs = append(out.StudentRefs, ref)
			}
			sort.Strings(out.StudentRefs)
		}
		targets := 0
		if out.GroupID > 0 {
			targets++
		}
		if out.CourseSectionID > 0 {
			targets++
		}
		if len(out.StudentRefs) > 0 {
			targets++
		}
		if targets != 1 {
			return snap, fmt.Errorf("Page override %d has an unsupported target shape; its access cannot be preserved", row.ID)
		}
		snap.Overrides = append(snap.Overrides, out)
	}
	sort.Slice(snap.Overrides, func(i, j int) bool { return snap.Overrides[i].ID < snap.Overrides[j].ID })
	return snap, nil
}

// pageDatesFingerprint identifies the exact observed collection so a later
// write can refuse to replace rows the teacher changed. Empty and absent
// collections hash identically so a document round-trip cannot look changed.
func pageDatesFingerprint(snap domain.PageDatesSnapshot) string {
	normalized := snap
	if len(normalized.Overrides) == 0 {
		normalized.Overrides = nil
	}
	return audienceHash(normalized)
}

// pageOverridePayload builds one override entry for a collection write. An
// existing row keeps its exact ID; a new row omits it.
func pageOverridePayload(row domain.PageOverrideRow, studentIDs []int64) map[string]any {
	fields := map[string]any{}
	if row.ID > 0 {
		fields["id"] = row.ID
	}
	if row.Title != "" {
		fields["title"] = row.Title
	}
	if row.GroupID > 0 {
		fields["group_id"] = row.GroupID
	}
	if row.CourseSectionID > 0 {
		fields["course_section_id"] = row.CourseSectionID
	}
	if len(studentIDs) > 0 {
		fields["student_ids"] = studentIDs
	}
	if row.UnlockAt != nil {
		fields["unlock_at"] = *row.UnlockAt
	}
	if row.LockAt != nil {
		fields["lock_at"] = *row.LockAt
	}
	return fields
}

// writePageDates replaces the complete collection. Canvas documents 204 with no
// body, so success is accepted without requiring returned override IDs.
func (s *Service) writePageDates(ctx context.Context, courseID, pageID int64, onlyVisible bool, unlockAt, lockAt *string, overrides []map[string]any) error {
	body := map[string]any{"only_visible_to_overrides": onlyVisible, "assignment_overrides": overrides}
	if unlockAt != nil {
		body["unlock_at"] = *unlockAt
	}
	if lockAt != nil {
		body["lock_at"] = *lockAt
	}
	return s.Client.WriteAccepted(ctx, http.MethodPut, pageDateDetailsPath(courseID, pageID), body, nil)
}

// pageAudienceRows returns the observed rows that this audience does not own.
// They are preserved verbatim in every collection write.
func pageAudienceRows(snap domain.PageDatesSnapshot, ownedIDs map[int64]bool) []domain.PageOverrideRow {
	rows := []domain.PageOverrideRow{}
	for _, row := range snap.Overrides {
		if !ownedIDs[row.ID] {
			rows = append(rows, row)
		}
	}
	return rows
}

// pageAudienceExceptions describes access beyond the selected clubs so review
// never claims exclusive access.
func pageAudienceExceptions(snap domain.PageDatesSnapshot, ownedIDs map[int64]bool, desired map[string]bool) []string {
	exceptions := []string{}
	for _, row := range snap.Overrides {
		if ownedIDs[row.ID] {
			continue
		}
		switch {
		case row.GroupID > 0:
			exceptions = append(exceptions, fmt.Sprintf("Override %d preserves additional group access", row.ID))
		case row.CourseSectionID > 0:
			exceptions = append(exceptions, fmt.Sprintf("Override %d preserves additional section access", row.ID))
		default:
			extra := false
			for _, ref := range row.StudentRefs {
				if !desired[ref] {
					extra = true
				}
			}
			if extra {
				exceptions = append(exceptions, fmt.Sprintf("Override %d preserves student access beyond the selected clubs; exclusive club access is not achieved", row.ID))
			}
		}
	}
	return exceptions
}

// pageStudentIDs resolves private refs to Canvas user IDs for a collection write.
func (s *Service) pageStudentIDs(store *workspace.StudentStore, refs []string) ([]int64, error) {
	ids := []int64{}
	for _, ref := range refs {
		student, err := store.Resolve(s.Client.Origin(), ref)
		if err != nil {
			return nil, err
		}
		ids = append(ids, student.UserID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

// pageAudienceOwnedIDs lists the exact rows this audience created and accepted.
func pageAudienceOwnedIDs(a *domain.Audience) map[int64]bool {
	ids := map[int64]bool{}
	if a == nil || a.State == nil {
		return ids
	}
	for _, row := range a.State.Owned {
		if row.ID > 0 && definite(row.Outcome) {
			ids[row.ID] = true
		}
	}
	return ids
}

// pageAudienceRefs returns the desired refs that are not already covered by a
// preserved manual student row.
func pageAudienceRefs(st *domain.AudienceState, snap domain.PageDatesSnapshot, ownedIDs map[int64]bool) ([]string, []string) {
	manual := map[string]bool{}
	exceptions := []string{}
	for _, row := range snap.Overrides {
		if ownedIDs[row.ID] {
			continue
		}
		for _, ref := range row.StudentRefs {
			manual[ref] = true
		}
	}
	refs := []string{}
	for _, ref := range st.StudentRefs {
		if manual[ref] {
			exceptions = append(exceptions, ref+" retains an existing student exception")
			continue
		}
		refs = append(refs, ref)
	}
	return refs, exceptions
}

// pageDatesChanged reports whether the requested availability dates differ
// from the observed baseline.
func pageDatesChanged(d *domain.PageDocument) bool {
	if d.ObservedDates == nil {
		return false
	}
	if d.UnlockAt != nil && !optionalDateMatches(d.UnlockAt, d.ObservedDates.UnlockAt) {
		return true
	}
	if d.LockAt != nil && !optionalDateMatches(d.LockAt, d.ObservedDates.LockAt) {
		return true
	}
	return false
}

func optionalDateMatches(d *domain.OptionalDate, observed *string) bool {
	if d == nil || !d.Set {
		return true
	}
	if d.Null {
		return observed == nil || *observed == ""
	}
	return observed != nil && *observed == d.Value
}

// pageDateValue returns the value to send for one availability date.
func pageDateValue(d *domain.OptionalDate, observed *string) *string {
	if d == nil || !d.Set {
		return observed
	}
	if d.Null {
		return nil
	}
	value := d.Value
	return &value
}

// pageDatesSummary describes the availability-date part of a Page action.
func pageDatesSummary(d *domain.PageDocument) string {
	parts := []string{}
	if d.UnlockAt != nil {
		parts = append(parts, "unlock "+reviewOptionalDate(d.UnlockAt))
	}
	if d.LockAt != nil {
		parts = append(parts, "lock "+reviewOptionalDate(d.LockAt))
	}
	return strings.Join(parts, "; ")
}

func reviewOptionalDate(d *domain.OptionalDate) string {
	if d == nil || !d.Set {
		return "preserve"
	}
	if d.Null {
		return "clear"
	}
	return d.Value
}
