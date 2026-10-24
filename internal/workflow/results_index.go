package workflow

import (
	"bytes"
	"path/filepath"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
	"gopkg.in/yaml.v3"
)

type resultsAssignmentKey struct {
	instance           string
	course, assignment int64
}

type indexedResult struct {
	student                                     string
	attempt                                     domain.AttemptRef
	academic, comment                           bool
	timing                                      *domain.LateTiming
	timingRecord, timingOrigin, timingCompleted bool
}

// resultsIndex contains only completion/provenance facts. Build it once for an
// operation, then share it across assignment reads without further disk reads.
type resultsIndex struct {
	assignments map[resultsAssignmentKey][]indexedResult
}

func (s *Service) withResultsIndex() *Service {
	copy := *s
	copy.resultsIndex = s.indexedResults()
	return &copy
}

func (s *Service) indexedResults() *resultsIndex {
	if s.resultsIndex != nil {
		return s.resultsIndex
	}
	return loadResultsIndex(s.Root)
}

func loadResultsIndex(root string) *resultsIndex {
	index := &resultsIndex{assignments: map[resultsAssignmentKey][]indexedResult{}}
	// Shared results identify member attempts through their source packet. Cache
	// that packet/work read as well, including missing/unreadable sources.
	packets := map[string]map[int64]map[string]domain.AttemptRef{}
	_ = walkResultFiles(root, func(path string) error {
		var raw map[string]any
		if _, err := workspace.ReadDocument(path, &raw); err != nil || yamlInt(raw["schema"]) != 1 {
			return nil
		}
		kind := yamlString(raw["kind"])
		if kind != "results" && kind != "group-results" {
			return nil
		}
		a := yamlMap(raw["assignment"])
		key := resultsAssignmentKey{yamlString(a["instance"]), yamlInt64(a["course_id"]), yamlInt64(a["assignment_id"])}
		encoded, err := yaml.Marshal(raw)
		if err != nil {
			return nil
		}
		if kind == "results" {
			var doc domain.ResultsDocument
			if decodeIndexedDocument(encoded, &doc) {
				for _, row := range doc.Students {
					index.addStudent(key, row, true, true)
				}
			}
			return nil
		}
		packetPath := yamlString(raw["packet"])
		attempts, known := packets[packetPath]
		if !known {
			attempts = indexedGroupAttempts(root, packetPath)
			packets[packetPath] = attempts
		}
		// Academic group completion intentionally parses each grade independently:
		// an unrelated authored group action cannot undo an accepted grade.
		for _, rawGroup := range yamlSlice(raw["groups"]) {
			group := yamlMap(rawGroup)
			if yamlString(group["operation"]) == "grade" {
				var grade domain.GroupGradeOperation
				if decodeIndexedValue(group["grade"], &grade) && definite(grade.Outcomes.Comment) && definite(grade.Outcomes.Rubric) && definite(grade.Outcomes.Grade) {
					for _, ref := range yamlStrings(group["member_refs"]) {
						if attempt, ok := attempts[yamlInt64(group["group_id"])][ref]; ok {
							index.assignments[key] = append(index.assignments[key], indexedResult{student: ref, attempt: attempt, academic: true})
						}
					}
				}
			}
			for _, rawStudent := range yamlSlice(group["students"]) {
				row := yamlMap(rawStudent)
				if yamlString(row["operation"]) == "grade" {
					var grade domain.GradeOperation
					if decodeIndexedValue(row["grade"], &grade) && academicGradeComplete(&grade) {
						index.assignments[key] = append(index.assignments[key], indexedResult{student: yamlString(row["student_ref"]), attempt: grade.Attempt, academic: true})
					}
				}
			}
		}
		var groups domain.GroupResultsDocument
		if !decodeIndexedDocument(encoded, &groups) {
			return nil
		}
		for _, group := range groups.Groups {
			for _, ref := range group.MemberRefs {
				attempt, ok := attempts[group.GroupID][ref]
				if !ok {
					continue
				}
				var timing *domain.LateTiming
				var provenance domain.SchoolTimeProvenance
				var hasProvenance, record bool
				if group.Operation == "late-only" && group.LateOnly != nil && group.LateOnly.Outcome.Status == "succeeded" {
					timing = &group.LateOnly.LateTiming
					provenance, hasProvenance = group.LateOnly.MemberSchoolTime[ref]
					record = true
				} else if group.Operation == "grade" && group.Grade != nil && group.Grade.LateTiming != nil && group.Grade.Outcomes.Late.Status == "succeeded" {
					timing = group.Grade.LateTiming
					provenance, hasProvenance = group.Grade.MemberSchoolTime[ref]
					record = provenance.Attempt.Attempt > 0
				}
				if timing != nil {
					row := indexedResult{student: ref, attempt: attempt, timingRecord: record}
					if hasProvenance {
						memberTiming := *timing
						memberTiming.SchoolTime = &provenance
						row.timing = &memberTiming
						row.timingOrigin = true
						row.timingCompleted = provenance.Status == timing.Status && provenance.SecondsLate == timing.SecondsLate
					}
					index.assignments[key] = append(index.assignments[key], row)
				}
			}
			for _, row := range group.Students {
				// Legacy timing-origin lookup requires membership in the result's
				// audience; timing completion can still use an individual row.
				index.addStudent(key, row, false, containsString(group.MemberRefs, row.StudentRef))
			}
		}
		return nil
	})
	return index
}

func decodeIndexedValue(value, target any) bool {
	encoded, err := yaml.Marshal(value)
	return err == nil && yaml.Unmarshal(encoded, target) == nil
}

func decodeIndexedDocument(encoded []byte, target any) bool {
	decoder := yaml.NewDecoder(bytes.NewReader(encoded))
	decoder.KnownFields(true)
	return decoder.Decode(target) == nil
}

func (index *resultsIndex) addStudent(key resultsAssignmentKey, row domain.StudentResult, academic, timingAudience bool) {
	item := indexedResult{student: row.StudentRef}
	switch {
	case row.Operation == "grade" && row.Grade != nil:
		item.attempt = row.Grade.Attempt
		item.academic = academic && academicGradeComplete(row.Grade)
		if row.Grade.Outcomes.Late.Status == "succeeded" {
			item.timing = row.Grade.LateTiming
		}
	case row.Operation == "late-only" && row.LateOnly != nil:
		item.attempt = row.LateOnly.Attempt
		if row.LateOnly.Outcome.Status == "succeeded" {
			item.timing = &row.LateOnly.LateTiming
		}
	case row.Operation == "comment-only" && row.CommentOnly != nil:
		item.attempt = row.CommentOnly.Attempt
		item.comment = row.CommentOnly.Outcome.Status == "succeeded" && !row.CommentOnly.FeedbackOnly()
	default:
		return
	}
	item.timingCompleted = item.timing != nil
	item.timingRecord = timingAudience && item.timing != nil
	item.timingOrigin = item.timingRecord
	index.assignments[key] = append(index.assignments[key], item)
}

func indexedGroupAttempts(root, packetPath string) map[int64]map[string]domain.AttemptRef {
	groups := map[int64]map[string]domain.AttemptRef{}
	if !workspacePathUnderCourses(packetPath) {
		return groups
	}
	path, err := workspace.ConstrainPath(root, filepath.FromSlash(packetPath))
	if err != nil {
		return groups
	}
	var packet domain.GroupPacketDocument
	if _, err := workspace.ReadDocument(path, &packet); err != nil {
		return groups
	}
	for _, entry := range packet.Groups {
		if _, seen := groups[entry.GroupID]; seen {
			continue
		}
		groups[entry.GroupID] = map[string]domain.AttemptRef{}
		if !workspacePathUnderCourses(entry.Path) {
			continue
		}
		workPath, err := workspace.ConstrainPath(root, filepath.FromSlash(entry.Path))
		if err != nil {
			continue
		}
		var work domain.GroupWorkDocument
		if _, err := workspace.ReadDocument(workPath, &work); err != nil {
			continue
		}
		for _, member := range work.Members {
			if _, seen := groups[entry.GroupID][member.StudentRef]; !seen {
				groups[entry.GroupID][member.StudentRef] = domain.AttemptRef{Attempt: member.Attempt, SubmittedAt: member.SubmittedAt}
			}
		}
	}
	return groups
}

func (index *resultsIndex) completedHint(id int64, ref string, sub Submission, snapshot domain.AssignmentSnapshot) bool {
	for _, row := range index.assignments[resultsAssignmentKey{snapshot.Instance, snapshot.CourseID, id}] {
		if row.student == ref && row.academic && sameAttempt(row.attempt, attemptRef(sub)) {
			return true
		}
	}
	return false
}

func (index *resultsIndex) completedCommentOnlyHint(id int64, ref string, sub Submission, snapshot domain.AssignmentSnapshot) bool {
	for _, row := range index.assignments[resultsAssignmentKey{snapshot.Instance, snapshot.CourseID, id}] {
		if row.student == ref && row.comment && sameAttempt(row.attempt, attemptRef(sub)) {
			return true
		}
	}
	return false
}

func (index *resultsIndex) completedTimingHint(instance string, assignmentID, courseID int64, ref string, sub Submission, a Assignment, calendar *workspace.SchoolCalendar) bool {
	if ref == "" || !sub.LateKnown || !sub.SecondsLateKnown || !sub.LatePolicyStatusKnown {
		return false
	}
	due, known := submissionEffectiveDue(sub, a)
	if !known || due == "" {
		return false
	}
	for _, row := range index.assignments[resultsAssignmentKey{instance, courseID, assignmentID}] {
		if row.student == ref && row.timingCompleted && sameAttempt(row.attempt, attemptRef(sub)) && timingSourcePresent(*row.timing, sub) && timingStillMatches(*row.timing, due, sub, calendar) {
			return true
		}
	}
	return false
}

func (index *resultsIndex) acceptedTimingRecordExists(instance string, courseID, assignmentID int64, ref string, current domain.AttemptRef) bool {
	for _, row := range index.assignments[resultsAssignmentKey{instance, courseID, assignmentID}] {
		if row.student == ref && row.timingRecord && sameAttempt(row.attempt, current) {
			return true
		}
	}
	return false
}

func (index *resultsIndex) acceptedTimingOrigin(instance string, courseID, assignmentID int64, ref string, current, source domain.AttemptRef, observed domain.ObservedTiming) bool {
	for _, row := range index.assignments[resultsAssignmentKey{instance, courseID, assignmentID}] {
		if row.student == ref && row.timingOrigin && sameAttempt(row.attempt, current) && acceptedTimingMatches(*row.timing, source, observed) {
			return true
		}
	}
	return false
}
