package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// LateTimingDecisionsDocument references one pulled packet and records the
// agent's evidence-based first-genuine attempt and any established deadline.
type LateTimingDecisionsDocument struct {
	Schema   int                         `yaml:"schema" json:"schema"`
	Kind     string                      `yaml:"kind" json:"kind"`
	Packet   string                      `yaml:"packet" json:"packet"`
	Students []LateTimingStudentDecision `yaml:"students,omitempty" json:"students,omitempty"`
	Groups   []LateTimingGroupDecision   `yaml:"groups,omitempty" json:"groups,omitempty"`
}

type LateTimingStudentDecision struct {
	StudentRef           string            `yaml:"student_ref" json:"student_ref"`
	FirstGenuine         domain.AttemptRef `yaml:"first_genuine" json:"first_genuine"`
	AccommodatedDeadline string            `yaml:"accommodated_deadline,omitempty" json:"accommodated_deadline,omitempty"`
	AccommodationBasis   string            `yaml:"accommodation_basis,omitempty" json:"accommodation_basis,omitempty"`
}

type LateTimingGroupDecision struct {
	GroupID int64                       `yaml:"group_id" json:"group_id"`
	Members []LateTimingStudentDecision `yaml:"members" json:"members"`
}

type LateTimingPreparation struct {
	Schema   int                       `json:"schema"`
	Kind     string                    `json:"kind"`
	Packet   string                    `json:"packet"`
	Students []PreparedLateTiming      `json:"students,omitempty"`
	Groups   []PreparedGroupLateTiming `json:"groups,omitempty"`
}

type PreparedLateTiming struct {
	StudentRef string             `json:"student_ref"`
	GroupID    int64              `json:"group_id,omitempty"`
	Status     string             `json:"status"` // proposed, unchanged, held
	Reason     string             `json:"reason,omitempty"`
	Timing     *domain.LateTiming `json:"late_timing,omitempty"`
}

type PreparedGroupLateTiming struct {
	GroupID          int64                                  `json:"group_id"`
	Status           string                                 `json:"status"` // proposed, unchanged, held
	Reason           string                                 `json:"reason,omitempty"`
	Timing           *domain.LateTiming                     `json:"late_timing,omitempty"`
	ObservedMembers  map[string]domain.ObservedTiming       `json:"observed_members,omitempty"`
	MemberSchoolTime map[string]domain.SchoolTimeProvenance `json:"member_school_time,omitempty"`
	Members          []PreparedLateTiming                   `json:"members,omitempty"`
}

func (d LateTimingDecisionsDocument) Validate() error {
	if d.Schema != 1 || d.Kind != "late-timing-decisions" || !workspacePathUnderCourses(d.Packet) {
		return fmt.Errorf("late-timing-decisions needs schema 1, kind, and a packet path under courses/")
	}
	if len(d.Students) == 0 && len(d.Groups) == 0 {
		return fmt.Errorf("at least one student or group timing decision is required")
	}
	seenStudents, seenGroups := map[string]bool{}, map[int64]bool{}
	for i, row := range d.Students {
		if err := validateLateTimingDecision(row); err != nil {
			return fmt.Errorf("students[%d]: %w", i, err)
		}
		if seenStudents[row.StudentRef] {
			return fmt.Errorf("student %q has duplicate timing choices", row.StudentRef)
		}
		seenStudents[row.StudentRef] = true
	}
	for i, group := range d.Groups {
		if group.GroupID <= 0 || seenGroups[group.GroupID] || len(group.Members) == 0 {
			return fmt.Errorf("groups[%d] needs a unique positive group_id and member choices", i)
		}
		seenGroups[group.GroupID] = true
		seen := map[string]bool{}
		for j, row := range group.Members {
			if err := validateLateTimingDecision(row); err != nil {
				return fmt.Errorf("groups[%d].members[%d]: %w", i, j, err)
			}
			if seen[row.StudentRef] {
				return fmt.Errorf("groups[%d] has duplicate member choices for %q", i, row.StudentRef)
			}
			seen[row.StudentRef] = true
		}
	}
	return nil
}

func validateLateTimingDecision(row LateTimingStudentDecision) error {
	if strings.TrimSpace(row.StudentRef) == "" || row.FirstGenuine.Attempt <= 0 {
		return fmt.Errorf("student_ref and positive first_genuine.attempt are required")
	}
	if err := validateDateTime(row.FirstGenuine.SubmittedAt, "first_genuine.submitted_at"); err != nil {
		return err
	}
	if row.AccommodatedDeadline != "" {
		if err := validateDateTime(row.AccommodatedDeadline, "accommodated_deadline"); err != nil {
			return err
		}
		if strings.TrimSpace(row.AccommodationBasis) == "" {
			return fmt.Errorf("accommodation_basis is required with accommodated_deadline")
		}
	} else if row.AccommodationBasis != "" {
		return fmt.Errorf("accommodation_basis requires accommodated_deadline")
	}
	return nil
}

func workspacePathUnderCourses(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	return !filepath.IsAbs(path) && clean != "courses" && strings.HasPrefix(clean, "courses/") && !strings.Contains(clean, "../")
}

// PrepareLateTiming reads a decisions file under courses/, resolves every
// identity and attempt from its referenced packet, and returns proposals only.
// It never writes to Canvas or edits the source packet/results.
func (s *Service) PrepareLateTiming(path string) (LateTimingPreparation, error) {
	absPath, err := workspace.ConstrainPath(s.Root, filepath.FromSlash(path))
	if err != nil {
		return LateTimingPreparation{}, err
	}
	if !pathUnderRootCourses(s.Root, absPath) {
		return LateTimingPreparation{}, fmt.Errorf("late-timing-decisions file must be under courses/")
	}
	var decisions LateTimingDecisionsDocument
	if _, err := workspace.ReadDocument(absPath, &decisions); err != nil {
		return LateTimingPreparation{}, err
	}
	if err := decisions.Validate(); err != nil {
		return LateTimingPreparation{}, err
	}
	packetPath, err := workspace.ConstrainPath(s.Root, filepath.FromSlash(decisions.Packet))
	if err != nil {
		return LateTimingPreparation{}, err
	}
	packetBytes, err := os.ReadFile(packetPath)
	if err != nil {
		return LateTimingPreparation{}, err
	}
	kind, err := workspace.DocumentKind(packetBytes)
	if err != nil {
		return LateTimingPreparation{}, err
	}
	return s.prepareLateTiming(decisions, packetBytes, kind)
}

func (s *Service) prepareLateTiming(decisions LateTimingDecisionsDocument, packetBytes []byte, kind string) (LateTimingPreparation, error) {
	result := LateTimingPreparation{Schema: 1, Kind: "late-timing-preparation", Packet: filepath.ToSlash(decisions.Packet)}
	switch kind {
	case "packet":
		if len(decisions.Groups) > 0 {
			return LateTimingPreparation{}, fmt.Errorf("individual packet cannot accept group timing choices")
		}
		var packet domain.PacketDocument
		if _, err := workspace.ParseDocument(packetBytes, &packet); err != nil {
			return LateTimingPreparation{}, err
		}
		if err := s.validateTimingPacketIdentity(packet.Course, packet.Assignment); err != nil || packet.Schema != 1 || packet.Kind != "packet" {
			if err == nil {
				err = fmt.Errorf("referenced packet schema or kind is invalid")
			}
			return LateTimingPreparation{}, err
		}
		studentFiles := make(map[string]string, len(packet.Students))
		for _, rel := range packet.Students {
			if !workspacePathUnderCourses(rel) {
				return LateTimingPreparation{}, fmt.Errorf("packet contains a student path outside courses/")
			}
			file, pathErr := workspace.ConstrainPath(s.Root, filepath.FromSlash(rel))
			if pathErr != nil {
				return LateTimingPreparation{}, pathErr
			}
			var work domain.StudentWorkDocument
			if _, pathErr = workspace.ReadDocument(file, &work); pathErr != nil {
				return LateTimingPreparation{}, pathErr
			}
			if work.AssignmentID != packet.Assignment.AssignmentID || work.StudentRef == "" || work.Kind != "student-work" {
				return LateTimingPreparation{}, fmt.Errorf("student-work document does not match the referenced packet assignment")
			}
			studentFiles[work.StudentRef] = file
		}
		for _, choice := range decisions.Students {
			file := studentFiles[choice.StudentRef]
			if file == "" {
				result.Students = append(result.Students, PreparedLateTiming{StudentRef: choice.StudentRef, Status: "held", Reason: "student reference is not in the referenced packet"})
				continue
			}
			var work domain.StudentWorkDocument
			if _, err := workspace.ReadDocument(file, &work); err != nil {
				return LateTimingPreparation{}, err
			}
			result.Students = append(result.Students, s.prepareStudentTiming(work, choice, packet.Assignment.CourseID))
		}
	case "group-packet":
		if len(decisions.Students) > 0 {
			return LateTimingPreparation{}, fmt.Errorf("group packet cannot accept ordinary student timing choices")
		}
		var packet domain.GroupPacketDocument
		if _, err := workspace.ParseDocument(packetBytes, &packet); err != nil {
			return LateTimingPreparation{}, err
		}
		if err := packet.Validate(); err != nil {
			return LateTimingPreparation{}, err
		}
		if err := s.validateTimingPacketIdentity(packet.Course, packet.Assignment); err != nil {
			return LateTimingPreparation{}, err
		}
		for _, groupChoice := range decisions.Groups {
			entry := findGroupPacketEntry(packet, groupChoice.GroupID)
			if entry == nil || !workspacePathUnderCourses(entry.Path) {
				result.Groups = append(result.Groups, PreparedGroupLateTiming{GroupID: groupChoice.GroupID, Status: "held", Reason: "group is not in the referenced packet"})
				continue
			}
			file, pathErr := workspace.ConstrainPath(s.Root, filepath.FromSlash(entry.Path))
			if pathErr != nil {
				return LateTimingPreparation{}, pathErr
			}
			var work domain.GroupWorkDocument
			if _, pathErr = workspace.ReadDocument(file, &work); pathErr != nil {
				return LateTimingPreparation{}, pathErr
			}
			if work.Kind != "group-work" || work.AssignmentID != packet.Assignment.AssignmentID || work.GroupID != groupChoice.GroupID || work.CourseID != packet.Assignment.CourseID || work.Instance != packet.Assignment.Instance {
				return LateTimingPreparation{}, fmt.Errorf("group-work document does not match the referenced packet assignment and group")
			}
			result.Groups = append(result.Groups, s.prepareGroupTiming(work, groupChoice))
		}
	default:
		return LateTimingPreparation{}, fmt.Errorf("referenced packet must be kind packet or group-packet")
	}
	return result, nil
}

func (s *Service) validateTimingPacketIdentity(courseName string, assignment domain.AssignmentSnapshot) error {
	course, err := s.Course(courseName)
	if err != nil {
		return err
	}
	if assignment.Instance == "" || assignment.Instance != s.Config.Instance || assignment.CourseID != course.ID || assignment.AssignmentID <= 0 {
		return fmt.Errorf("referenced packet does not match the configured Canvas instance, course, and assignment")
	}
	return nil
}

// PrepareLateTimingDecisions exposes the same typed preparation path used by
// the CLI file contract for MCP callers that provide the decision batch inline.
func (s *Service) PrepareLateTimingDecisions(decisions LateTimingDecisionsDocument) (LateTimingPreparation, error) {
	if err := decisions.Validate(); err != nil {
		return LateTimingPreparation{}, err
	}
	packetPath, err := workspace.ConstrainPath(s.Root, filepath.FromSlash(decisions.Packet))
	if err != nil {
		return LateTimingPreparation{}, err
	}
	packetBytes, err := os.ReadFile(packetPath)
	if err != nil {
		return LateTimingPreparation{}, err
	}
	kind, err := workspace.DocumentKind(packetBytes)
	if err != nil {
		return LateTimingPreparation{}, err
	}
	return s.prepareLateTiming(decisions, packetBytes, kind)
}

func pathUnderRootCourses(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && workspacePathUnderCourses(rel)
}

func findGroupPacketEntry(packet domain.GroupPacketDocument, groupID int64) *domain.GroupPacketEntry {
	for i := range packet.Groups {
		if packet.Groups[i].GroupID == groupID {
			return &packet.Groups[i]
		}
	}
	return nil
}

func (s *Service) prepareStudentTiming(work domain.StudentWorkDocument, choice LateTimingStudentDecision, courseID int64) PreparedLateTiming {
	row := PreparedLateTiming{StudentRef: work.StudentRef}
	if work.AssignmentVisible != nil && !*work.AssignmentVisible {
		row.Status, row.Reason = "held", "assignment is not currently visible to this student"
		return row
	}
	if work.Excused {
		row.Status, row.Reason = "held", "excused work is excluded from timing changes"
		return row
	}
	if work.Current.ObservedTiming == nil || !work.Current.ObservedTiming.StatusKnown || !work.Current.ObservedTiming.SecondsLateKnown || !work.Current.ObservedTiming.LateKnown {
		row.Status, row.Reason = "held", "current Canvas timing is incomplete; pull the assignment again"
		return row
	}
	if work.Current.ObservedTiming.HasCustomStatus {
		row.Status, row.Reason = "held", "a custom Canvas grade status would be cleared by a timing change"
		return row
	}
	status := ""
	if work.Current.ObservedTiming.LatePolicyStatus != nil {
		status = *work.Current.ObservedTiming.LatePolicyStatus
	}
	accepted := s.acceptedTimingOrigin(courseID, work.AssignmentID, work.StudentRef, work.Current.Ref, choice.FirstGenuine, *work.Current.ObservedTiming)
	if status != "" && status != "late" && !accepted {
		row.Status, row.Reason = "unchanged", "an explicit Canvas timing status is preserved"
		return row
	}
	if status == "late" && !accepted {
		row.Status, row.Reason = "held", "explicit Canvas late status has no matching accepted CE-SE timing provenance"
		return row
	}
	if work.Current.EffectiveDueAt == "" {
		row.Status, row.Reason = "held", "the student's effective Canvas due date is unavailable"
		return row
	}
	timing, reason := s.calculateTiming(choice, work.Current.EffectiveDueAt, work.Current.ObservedTiming, attemptsForStudent(work), work.Current.Ref)
	if reason != "" {
		row.Status, row.Reason = "held", reason
		return row
	}
	row.Timing = timing
	if timingMatchesObserved(*timing, *work.Current.ObservedTiming) {
		row.Status = "unchanged"
	} else {
		row.Status = "proposed"
	}
	return row
}

func attemptsForStudent(work domain.StudentWorkDocument) []domain.AttemptWork {
	return append([]domain.AttemptWork{work.Current}, work.Prior...)
}

func (s *Service) calculateTiming(choice LateTimingStudentDecision, canvasDue string, observed *domain.ObservedTiming, attempts []domain.AttemptWork, current domain.AttemptRef) (*domain.LateTiming, string) {
	var source *domain.AttemptWork
	for i := range attempts {
		if attempts[i].Ref.Attempt == choice.FirstGenuine.Attempt && attempts[i].Ref.SubmittedAt == choice.FirstGenuine.SubmittedAt {
			source = &attempts[i]
			break
		}
	}
	if source == nil {
		return nil, "selected first genuine attempt is not in the referenced packet"
	}
	due, err := time.Parse(time.RFC3339Nano, canvasDue)
	if err != nil {
		return nil, "effective Canvas due date is malformed"
	}
	deadline := due
	basis := "First genuine submission compared with the effective Canvas due date."
	if choice.AccommodatedDeadline != "" {
		deadline, err = time.Parse(time.RFC3339Nano, choice.AccommodatedDeadline)
		if err != nil {
			return nil, "accommodated deadline is malformed"
		}
		basis = strings.TrimSpace(choice.AccommodationBasis)
	}
	submitted, err := time.Parse(time.RFC3339Nano, source.Ref.SubmittedAt)
	if err != nil {
		return nil, "selected first genuine submission time is malformed"
	}
	calendar := workspace.SchoolCalendar{}
	if submitted.After(deadline) {
		calendar, err = workspace.ReadSchoolCalendar(s.Root)
		if err != nil {
			return nil, "school calendar is unavailable for a late interval: " + err.Error()
		}
	}
	counted, err := calendar.CountSchoolTime(deadline, submitted)
	if err != nil {
		return nil, "school time cannot be calculated: " + err.Error()
	}
	if !observed.StatusKnown || !observed.SecondsLateKnown || !observed.LateKnown {
		return nil, "current Canvas timing was not fully observed; pull the assignment again"
	}
	provenance := &domain.SchoolTimeProvenance{Attempt: source.Ref, SubmittedAt: source.Ref.SubmittedAt, Deadline: deadline.Format(time.RFC3339), Status: counted.Status, SecondsLate: counted.SecondsLate, SkippedDates: append([]string(nil), counted.SkippedDates...)}
	timing := &domain.LateTiming{Status: counted.Status, SecondsLate: counted.SecondsLate, DueAt: due.Format(time.RFC3339), Basis: basis, SchoolTime: provenance, Observed: observed}
	return timing, ""
}

func timingMatchesObserved(timing domain.LateTiming, observed domain.ObservedTiming) bool {
	if observed.HasCustomStatus {
		return false
	}
	if timing.Status == "none" {
		return !observed.Late && (observed.LatePolicyStatus == nil || *observed.LatePolicyStatus == "none")
	}
	return timing.Status == "late" && observed.Late && observed.SecondsLate == timing.SecondsLate && (observed.LatePolicyStatus == nil || *observed.LatePolicyStatus == "late")
}

func (s *Service) prepareGroupTiming(work domain.GroupWorkDocument, group LateTimingGroupDecision) PreparedGroupLateTiming {
	result := PreparedGroupLateTiming{GroupID: work.GroupID}
	individual := work.Assignment.GradeGroupStudentsIndividually != nil && *work.Assignment.GradeGroupStudentsIndividually
	if !work.RosterComplete || !work.PropagationAudienceComplete || (!individual && work.SharedAssessmentStatus == "held") {
		result.Status, result.Reason = "held", "group audience is incomplete or changed; timing was not prepared"
		return result
	}
	members := map[string]domain.GroupWorkMember{}
	for _, member := range work.Members {
		members[member.StudentRef] = member
	}
	choices := map[string]LateTimingStudentDecision{}
	for _, choice := range group.Members {
		choices[choice.StudentRef] = choice
	}
	if work.Assignment.GradeGroupStudentsIndividually != nil && !*work.Assignment.GradeGroupStudentsIndividually {
		if len(choices) != len(work.PropagationAudience) {
			result.Status, result.Reason = "held", "shared timing requires a first-genuine choice for every affected member"
			return result
		}
		result.ObservedMembers = map[string]domain.ObservedTiming{}
		result.MemberSchoolTime = map[string]domain.SchoolTimeProvenance{}
		var common *domain.LateTiming
		for _, ref := range work.PropagationAudience {
			member, ok := members[ref]
			choice, hasChoice := choices[ref]
			if !ok || !hasChoice {
				result.Status, result.Reason = "held", "shared timing requires a first-genuine choice for every affected member"
				return result
			}
			if member.ObservedTiming == nil || !member.ObservedTiming.StatusKnown || !member.ObservedTiming.SecondsLateKnown || !member.ObservedTiming.LateKnown || member.ObservedTiming.HasCustomStatus {
				result.Status, result.Reason = "held", "a member's current Canvas timing is incomplete or has a custom status"
				return result
			}
			workDoc := groupMemberAsStudentWork(work, member)
			prepared := s.prepareStudentTiming(workDoc, choice, work.CourseID)
			if prepared.Status == "held" || prepared.Timing == nil {
				result.Status, result.Reason = "held", "member "+ref+": "+prepared.Reason
				return result
			}
			if common == nil {
				common = prepared.Timing
			} else if common.Status != prepared.Timing.Status || common.SecondsLate != prepared.Timing.SecondsLate || common.DueAt != prepared.Timing.DueAt || common.Basis != prepared.Timing.Basis || common.SchoolTime.Deadline != prepared.Timing.SchoolTime.Deadline {
				result.Status, result.Reason = "held", "members do not share one school-time decision and effective due date"
				return result
			}
			result.ObservedMembers[ref] = *member.ObservedTiming
			result.MemberSchoolTime[ref] = *prepared.Timing.SchoolTime
		}
		result.Timing = common
		if result.Timing == nil {
			result.Status, result.Reason = "held", "shared timing has no reviewed members"
		} else if allGroupTimingUnchanged(result.ObservedMembers, *result.Timing) {
			result.Status = "unchanged"
		} else {
			result.Status = "proposed"
		}
		return result
	}
	for _, choice := range group.Members {
		member, ok := members[choice.StudentRef]
		if !ok {
			result.Members = append(result.Members, PreparedLateTiming{StudentRef: choice.StudentRef, GroupID: work.GroupID, Status: "held", Reason: "student is not in this group packet"})
			continue
		}
		prepared := s.prepareStudentTiming(groupMemberAsStudentWork(work, member), choice, work.CourseID)
		prepared.GroupID = work.GroupID
		result.Members = append(result.Members, prepared)
	}
	if len(result.Members) > 0 {
		result.Status = "proposed"
		for _, row := range result.Members {
			if row.Status == "held" {
				result.Status = "partial"
				break
			}
		}
	}
	return result
}

func (s *Service) acceptedTimingOrigin(courseID, assignmentID int64, studentRef string, current, source domain.AttemptRef, observed domain.ObservedTiming) bool {
	accepted := false
	_ = filepath.WalkDir(filepath.Join(s.Root, "courses"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry == nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		var results domain.ResultsDocument
		if _, err := workspace.ReadDocument(path, &results); err == nil && results.Schema == 1 && results.Kind == "results" && results.Assignment.Instance == s.Config.Instance && results.Assignment.CourseID == courseID && results.Assignment.AssignmentID == assignmentID {
			for _, row := range results.Students {
				if row.StudentRef != studentRef {
					continue
				}
				if row.Operation == "grade" && row.Grade != nil && row.Grade.LateTiming != nil && row.Grade.Outcomes.Late.Status == "succeeded" && sameAttempt(row.Grade.Attempt, current) && acceptedTimingMatches(*row.Grade.LateTiming, source, observed) {
					accepted = true
					return filepath.SkipAll
				}
				if row.Operation == "late-only" && row.LateOnly != nil && row.LateOnly.Outcome.Status == "succeeded" && sameAttempt(row.LateOnly.Attempt, current) && acceptedTimingMatches(row.LateOnly.LateTiming, source, observed) {
					accepted = true
					return filepath.SkipAll
				}
			}
			return nil
		}
		var groups domain.GroupResultsDocument
		if _, err := workspace.ReadDocument(path, &groups); err != nil || groups.Schema != 1 || groups.Kind != "group-results" || groups.Assignment.Instance != s.Config.Instance || groups.Assignment.CourseID != courseID || groups.Assignment.AssignmentID != assignmentID {
			return nil
		}
		for _, group := range groups.Groups {
			if !containsString(group.MemberRefs, studentRef) {
				continue
			}
			if group.Operation == "late-only" && group.LateOnly != nil && group.LateOnly.Outcome.Status == "succeeded" && groupResultCurrentAttempt(s.Root, groups.Packet, group.GroupID, studentRef, current) {
				if provenance, ok := group.LateOnly.MemberSchoolTime[studentRef]; ok && sameAttempt(provenance.Attempt, source) {
					timing := group.LateOnly.LateTiming
					timing.SchoolTime = &provenance
					if acceptedTimingMatches(timing, source, observed) {
						accepted = true
						return filepath.SkipAll
					}
				}
			}
			if group.Operation == "grade" && group.Grade != nil && group.Grade.LateTiming != nil && group.Grade.Outcomes.Late.Status == "succeeded" && groupResultCurrentAttempt(s.Root, groups.Packet, group.GroupID, studentRef, current) {
				if provenance, ok := group.Grade.MemberSchoolTime[studentRef]; ok && sameAttempt(provenance.Attempt, source) {
					timing := *group.Grade.LateTiming
					timing.SchoolTime = &provenance
					if acceptedTimingMatches(timing, source, observed) {
						accepted = true
						return filepath.SkipAll
					}
				}
			}
			for _, row := range group.Students {
				if row.StudentRef == studentRef && row.Operation == "grade" && row.Grade != nil && row.Grade.LateTiming != nil && row.Grade.Outcomes.Late.Status == "succeeded" && sameAttempt(row.Grade.Attempt, current) && acceptedTimingMatches(*row.Grade.LateTiming, source, observed) {
					accepted = true
					return filepath.SkipAll
				}
				if row.StudentRef == studentRef && row.Operation == "late-only" && row.LateOnly != nil && row.LateOnly.Outcome.Status == "succeeded" && sameAttempt(row.LateOnly.Attempt, current) && acceptedTimingMatches(row.LateOnly.LateTiming, source, observed) {
					accepted = true
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	return accepted
}

func (s *Service) acceptedTimingRecordExists(courseID, assignmentID int64, studentRef string, current domain.AttemptRef) bool {
	found := false
	_ = filepath.WalkDir(filepath.Join(s.Root, "courses"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry == nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		var results domain.ResultsDocument
		if _, err := workspace.ReadDocument(path, &results); err == nil && results.Schema == 1 && results.Kind == "results" && results.Assignment.Instance == s.Config.Instance && results.Assignment.CourseID == courseID && results.Assignment.AssignmentID == assignmentID {
			for _, row := range results.Students {
				if row.StudentRef == studentRef && ((row.Operation == "grade" && row.Grade != nil && row.Grade.Outcomes.Late.Status == "succeeded" && row.Grade.LateTiming != nil && sameAttempt(row.Grade.Attempt, current)) || (row.Operation == "late-only" && row.LateOnly != nil && row.LateOnly.Outcome.Status == "succeeded" && sameAttempt(row.LateOnly.Attempt, current))) {
					found = true
					return filepath.SkipAll
				}
			}
			return nil
		}
		var groups domain.GroupResultsDocument
		if _, err := workspace.ReadDocument(path, &groups); err != nil || groups.Schema != 1 || groups.Kind != "group-results" || groups.Assignment.Instance != s.Config.Instance || groups.Assignment.CourseID != courseID || groups.Assignment.AssignmentID != assignmentID {
			return nil
		}
		for _, group := range groups.Groups {
			if !containsString(group.MemberRefs, studentRef) {
				continue
			}
			if group.Operation == "late-only" && group.LateOnly != nil && group.LateOnly.Outcome.Status == "succeeded" && groupResultCurrentAttempt(s.Root, groups.Packet, group.GroupID, studentRef, current) {
				found = true
				return filepath.SkipAll
			}
			if group.Operation == "grade" && group.Grade != nil && group.Grade.LateTiming != nil && group.Grade.MemberSchoolTime[studentRef].Attempt.Attempt > 0 && group.Grade.Outcomes.Late.Status == "succeeded" && groupResultCurrentAttempt(s.Root, groups.Packet, group.GroupID, studentRef, current) {
				found = true
				return filepath.SkipAll
			}
			for _, row := range group.Students {
				if row.StudentRef == studentRef && ((row.Operation == "grade" && row.Grade != nil && row.Grade.LateTiming != nil && row.Grade.Outcomes.Late.Status == "succeeded" && sameAttempt(row.Grade.Attempt, current)) || (row.Operation == "late-only" && row.LateOnly != nil && row.LateOnly.Outcome.Status == "succeeded" && sameAttempt(row.LateOnly.Attempt, current))) {
					found = true
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	return found
}

func acceptedTimingMatches(timing domain.LateTiming, source domain.AttemptRef, observed domain.ObservedTiming) bool {
	if timing.SchoolTime == nil || !sameAttempt(timing.SchoolTime.Attempt, source) || observed.HasCustomStatus {
		return false
	}
	if timing.Status == "none" {
		return !observed.Late && (observed.LatePolicyStatus == nil || *observed.LatePolicyStatus == "none")
	}
	return timing.Status == "late" && observed.Late && observed.SecondsLateKnown && observed.SecondsLate == timing.SecondsLate && (observed.LatePolicyStatus == nil || *observed.LatePolicyStatus == "late")
}

func groupResultCurrentAttempt(root, packetPath string, groupID int64, studentRef string, current domain.AttemptRef) bool {
	if !workspacePathUnderCourses(packetPath) {
		return false
	}
	path, err := workspace.ConstrainPath(root, filepath.FromSlash(packetPath))
	if err != nil {
		return false
	}
	var packet domain.GroupPacketDocument
	if _, err := workspace.ReadDocument(path, &packet); err != nil {
		return false
	}
	entry := findGroupPacketEntry(packet, groupID)
	if entry == nil || !workspacePathUnderCourses(entry.Path) {
		return false
	}
	workPath, err := workspace.ConstrainPath(root, filepath.FromSlash(entry.Path))
	if err != nil {
		return false
	}
	var work domain.GroupWorkDocument
	if _, err := workspace.ReadDocument(workPath, &work); err != nil {
		return false
	}
	for _, member := range work.Members {
		if member.StudentRef == studentRef {
			return sameAttempt(domain.AttemptRef{Attempt: member.Attempt, SubmittedAt: member.SubmittedAt}, current)
		}
	}
	return false
}

func groupMemberAsStudentWork(group domain.GroupWorkDocument, member domain.GroupWorkMember) domain.StudentWorkDocument {
	current := domain.AttemptWork{Ref: domain.AttemptRef{Attempt: member.Attempt, SubmittedAt: member.SubmittedAt}, Late: member.Late, ObservedTiming: member.ObservedTiming, EffectiveDueAt: member.EffectiveDueAt}
	prior := make([]domain.AttemptWork, 0, len(member.Prior))
	for _, attempt := range member.Prior {
		prior = append(prior, domain.AttemptWork{Ref: attempt.Attempt, Late: attempt.Late})
	}
	return domain.StudentWorkDocument{StudentRef: member.StudentRef, AssignmentID: group.AssignmentID, AssignmentVisible: member.AssignmentVisible, Current: current, Prior: prior, Excused: member.Excused}
}

func allGroupTimingUnchanged(observed map[string]domain.ObservedTiming, timing domain.LateTiming) bool {
	if len(observed) == 0 {
		return false
	}
	for _, current := range observed {
		if !timingMatchesObserved(timing, current) {
			return false
		}
	}
	return true
}

func sortedMemberRefs(values map[string]domain.ObservedTiming) []string {
	refs := make([]string, 0, len(values))
	for ref := range values {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return refs
}
