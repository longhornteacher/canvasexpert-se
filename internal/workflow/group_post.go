package workflow

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
)

// GroupCardRef is the review/skip key of one group's indivisible shared card
// (grade, rubric, shared comment and late timing).
func GroupCardRef(groupID int64) string { return fmt.Sprintf("group:%d", groupID) }

// GroupPrivateRef is the review/skip key of one member's private comment.
func GroupPrivateRef(groupID int64, studentRef string) string {
	return fmt.Sprintf("group:%d:private:%s", groupID, studentRef)
}

type GroupRowReport struct {
	GroupID                    int64
	SharedDone                 bool
	Operation, Status, Message string
	Outcomes                   domain.ActionOutcomes
	Private                    map[string]domain.ActionOutcome
	Students                   []StudentPostReport
}
type GroupPostReport struct {
	Path     string
	Groups   []GroupRowReport
	Requests int64
	Elapsed  time.Duration
}

// groupSource is the pulled, constrained evidence behind one results row.
type groupSource struct {
	work     domain.GroupWorkDocument
	artifact domain.GroupArtifact
	students map[string]workspace.Student // by member ref
	members  map[string]domain.GroupWorkMember
	hold     string // reason this group's shared actions cannot be posted
}

// groupLive is the renewed pre-write read of one group.
type groupLive struct {
	subs map[string]Submission
	rep  workspace.Student
}

func groupUsesOnlyLateTiming(row domain.GroupResult) bool {
	if row.Operation == "late-only" {
		return true
	}
	if row.Operation != "individual" || len(row.Students) == 0 || row.SharedFeedback != nil {
		return false
	}
	for _, student := range row.Students {
		if student.Operation != "late-only" {
			return false
		}
	}
	return true
}

// GroupStudentRef is the review/skip key of one student's indivisible
// grade-and-comment card in an individual-mode group row.
func GroupStudentRef(groupID int64, studentRef string) string {
	return fmt.Sprintf("group:%d:student:%s", groupID, studentRef)
}

func groupSharedComplete(r domain.GroupResult) bool {
	if r.Operation == "individual" {
		return r.SharedFeedback == nil || definite(r.SharedFeedback.Outcome)
	}
	if r.Operation == "grade" {
		g := r.Grade
		return g != nil && definite(g.Outcomes.Comment) && definite(g.Outcomes.Rubric) && definite(g.Outcomes.Grade) && definite(g.Outcomes.Late)
	}
	if r.Operation == "late-only" {
		return r.LateOnly != nil && definite(r.LateOnly.Outcome)
	}
	return r.CommentOnly != nil && definite(r.CommentOnly.Outcome)
}

func groupComplete(r domain.GroupResult) bool {
	if !groupSharedComplete(r) {
		return false
	}
	for _, f := range r.IndividualFeedback {
		if !definite(f.Outcome) {
			return false
		}
	}
	for _, st := range r.Students {
		if !resultComplete(st) {
			return false
		}
	}
	return true
}

func holdGroupGrade(g *domain.GroupGradeOperation, message string) {
	for _, o := range []*domain.ActionOutcome{&g.Outcomes.Rubric, &g.Outcomes.Grade, &g.Outcomes.Comment, &g.Outcomes.Late} {
		if !definite(*o) {
			holdOutcome(o, message)
		}
	}
}

func holdGroupShared(r *domain.GroupResult, message string) {
	if r.Grade != nil {
		holdGroupGrade(r.Grade, message)
	} else if r.CommentOnly != nil {
		holdOutcome(&r.CommentOnly.Outcome, message)
	} else if r.LateOnly != nil {
		holdOutcome(&r.LateOnly.Outcome, message)
	}
}

// GroupSharedText renders the whole-group comment exactly as review shows and post sends.
func GroupSharedText(doc domain.GroupResultsDocument, row domain.GroupResult, v domain.ValidatedGroup, omitLate bool) (string, error) {
	basis := doc.Basis()
	if row.Operation == "individual" {
		sf := row.SharedFeedback
		if basis == nil || sf == nil || v.SharedFeedback == nil {
			return "", fmt.Errorf("shared feedback is not available for this row")
		}
		return render.RenderFeedback(render.FeedbackInput{Rubric: *basis, Interpretation: doc.Interpretation, Components: sf.Components, Assessment: v.SharedFeedback.Posted, AssignmentPoints: doc.Assignment.Points, Feedback: sf.Feedback, Header: sf.Header})
	}
	if basis == nil || v.Assessment == nil {
		return "", fmt.Errorf("scoring basis is absent")
	}
	if g := row.Grade; g != nil {
		if g.FeedbackAudience != "group" {
			return "", nil
		}
		return render.RenderFeedback(render.FeedbackInput{Rubric: *basis, Interpretation: doc.Interpretation, Components: g.Components, Assessment: g.Assessment, AssignmentPoints: doc.Assignment.Points, Feedback: g.Feedback, Late: isLate(g.LateTiming) && !omitLate, Header: g.Header})
	}
	c := row.CommentOnly
	return render.RenderFeedback(render.FeedbackInput{Rubric: *basis, Interpretation: doc.Interpretation, Components: c.Components, Assessment: v.Assessment.Posted, AssignmentPoints: doc.Assignment.Points, Feedback: c.Feedback, Header: c.Header})
}

func groupReport(r domain.GroupResult) GroupRowReport {
	p := GroupRowReport{GroupID: r.GroupID, Operation: r.Operation, Status: "partial", Private: map[string]domain.ActionOutcome{}, SharedDone: groupSharedComplete(r)}
	outcomes := []domain.ActionOutcome{}
	if r.Grade != nil {
		p.Outcomes = r.Grade.Outcomes
		outcomes = append(outcomes, p.Outcomes.Rubric, p.Outcomes.Grade, p.Outcomes.Comment, p.Outcomes.Late)
	} else if r.CommentOnly != nil {
		p.Outcomes.Comment = r.CommentOnly.Outcome
		outcomes = append(outcomes, r.CommentOnly.Outcome)
	} else if r.LateOnly != nil {
		p.Outcomes.Late = r.LateOnly.Outcome
		outcomes = append(outcomes, r.LateOnly.Outcome)
	}
	if r.SharedFeedback != nil {
		p.Outcomes.Comment = r.SharedFeedback.Outcome
		outcomes = append(outcomes, r.SharedFeedback.Outcome)
	}
	for _, st := range r.Students {
		sr := postReport(st)
		p.Students = append(p.Students, sr)
		if sr.Status != "completed" {
			if sr.Message != "" {
				if p.Message != "" {
					p.Message += "; "
				}
				p.Message += sr.StudentRef + ": " + sr.Message
			}
			if sr.Status == "uncertain" || (sr.Status == "held" && p.Status != "uncertain") {
				p.Status = sr.Status
			}
		}
	}
	for _, f := range r.IndividualFeedback {
		p.Private[f.StudentRef] = f.Outcome
		outcomes = append(outcomes, f.Outcome)
	}
	for _, o := range outcomes {
		if !definite(o) && o.Message != "" {
			if p.Message != "" {
				p.Message += "; "
			}
			p.Message += o.Message
		}
		if o.Status == "uncertain" || (o.Status == "held" && p.Status != "uncertain") {
			p.Status = o.Status
		}
	}
	if groupComplete(r) {
		p.Status = "completed"
	}
	return p
}

func (s *Service) PostGroupResults(ctx context.Context, path string) (report GroupPostReport, err error) {
	start := time.Now()
	outer := s // requests are counted across whichever key each course used
	requests := outer.requestCount()
	report.Path = path
	defer func() { report.Elapsed = time.Since(start); report.Requests = outer.requestCount() - requests }()
	if err = FillResultsAssignment(s.Root, path); err != nil {
		return report, err
	}
	var doc domain.GroupResultsDocument
	body, err := workspace.ReadDocument(path, &doc)
	if err != nil {
		return report, err
	}
	validated, err := domain.ValidateGroupResults(&doc)
	if err != nil {
		return report, err
	}
	// A local persistence error after remote success still reports known outcomes.
	defer func() {
		report.Groups = nil
		for _, row := range doc.Groups {
			report.Groups = append(report.Groups, groupReport(row))
		}
	}()
	if doc.Assignment.Instance != s.Client.Origin() {
		return report, fmt.Errorf("group results target a different Canvas instance")
	}
	configured := false
	var course domain.Course
	for _, c := range s.Config.Courses {
		if c.ID == doc.Assignment.CourseID {
			configured, course = true, c
		}
	}
	if !configured {
		return report, fmt.Errorf("group results course is not configured")
	}
	if s, err = s.ForCourse(ctx, course); err != nil {
		return report, err
	}
	allComplete := true
	for _, r := range doc.Groups {
		if !groupComplete(r) {
			allComplete = false
		}
	}
	if allComplete {
		return report, nil
	}
	// Resolve the packet, work files and identities before the first Canvas write.
	sources, err := s.groupPostSources(doc)
	if err != nil {
		return report, err
	}
	liveAssignment, err := GetAssignment(ctx, s.Client, doc.Assignment.CourseID, doc.Assignment.AssignmentID)
	if err != nil {
		return report, err
	}
	if liveAssignment.CourseID == 0 {
		liveAssignment.CourseID = doc.Assignment.CourseID
	}
	timingOnly := len(doc.Groups) > 0
	for _, row := range doc.Groups {
		if !groupUsesOnlyLateTiming(row) {
			timingOnly = false
			break
		}
	}
	var liveBasis domain.AssignmentSnapshot
	if timingOnly {
		liveBasis, err = TimingSnapshot(liveAssignment, s.Client.Origin())
	} else {
		liveBasis, err = Snapshot(liveAssignment, s.Client.Origin())
	}
	if err != nil {
		return report, err
	}
	basisHold := ""
	if !SameGradingBasis(doc.Assignment, liveBasis) {
		basisHold = "assignment, rubric or group grading basis changed; pull-work for reassessment"
	}
	coupledRubric := liveBasis.Rubric != nil && liveBasis.Rubric.UseForGradingKnown && liveBasis.Rubric.UseForGrading
	save := func() error { return workspace.WriteDocument(path, &doc, body) }
	for i := range doc.Groups {
		row := &doc.Groups[i]
		v := validated[i]
		src := sources[row.GroupID]
		if row.Operation == "individual" {
			if err = s.postGroupIndividual(ctx, &doc, row, v, src, save); err != nil {
				return report, err
			}
			continue
		}
		if row.Operation == "late-only" {
			hold := src.hold
			if hold == "" {
				hold = lateTimingAssignmentHold(liveAssignment, true)
			}
			if hold == "" && !SameGroupTimingMode(doc.Assignment, liveBasis) {
				hold = "assignment group grading mode changed; pull-work before adjusting timing"
			}
			if hold != "" {
				holdOutcome(&row.LateOnly.Outcome, hold)
				if err = save(); err != nil {
					return report, err
				}
			} else if !groupSharedComplete(*row) && !s.Skip[GroupCardRef(row.GroupID)] {
				if err = s.postGroupLateOnly(ctx, &doc, row, src, save); err != nil {
					return report, err
				}
			}
			continue
		}
		if !groupSharedComplete(*row) && !s.Skip[GroupCardRef(row.GroupID)] {
			hold := v.Hold
			if hold == "" {
				hold = basisHold
			}
			if hold == "" {
				hold = src.hold
			}
			if hold != "" {
				holdGroupShared(row, hold)
				if err = save(); err != nil {
					return report, err
				}
			} else if err = s.postGroupShared(ctx, &doc, row, v, src, coupledRubric, save); err != nil {
				return report, err
			}
		}
		for j := range row.IndividualFeedback {
			f := &row.IndividualFeedback[j]
			if definite(f.Outcome) || s.Skip[GroupPrivateRef(row.GroupID, f.StudentRef)] {
				continue
			}
			if err = s.postGroupPrivate(ctx, doc, row, f, src, save); err != nil {
				return report, err
			}
		}
	}
	return report, nil
}

// groupPostSources reads the exact packet and group-work files a results file
// names. Structural mismatch fails the file; a group whose evidence is simply
// not ready is held (src.hold) so independent groups continue.
func (s *Service) groupPostSources(doc domain.GroupResultsDocument) (map[int64]groupSource, error) {
	packetPath, err := workspace.ConstrainPath(s.Root, doc.Packet)
	if err != nil {
		return nil, err
	}
	var packet domain.GroupPacketDocument
	if _, err = workspace.ReadDocument(packetPath, &packet); err != nil {
		return nil, err
	}
	if err = packet.Validate(); err != nil {
		return nil, err
	}
	if packet.Assignment.Instance != doc.Assignment.Instance || packet.Assignment.CourseID != doc.Assignment.CourseID || packet.Assignment.AssignmentID != doc.Assignment.AssignmentID {
		return nil, fmt.Errorf("group results target differs from source packet")
	}
	paths := map[int64]string{}
	for _, g := range packet.Groups {
		paths[g.GroupID] = g.Path
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return nil, err
	}
	out := map[int64]groupSource{}
	for _, row := range doc.Groups {
		if groupComplete(row) {
			continue
		}
		rel, ok := paths[row.GroupID]
		if !ok {
			return nil, fmt.Errorf("group %d is absent from the source packet", row.GroupID)
		}
		workPath, err := workspace.ConstrainPath(s.Root, rel)
		if err != nil {
			return nil, err
		}
		var work domain.GroupWorkDocument
		if _, err = workspace.ReadDocument(workPath, &work); err != nil {
			return nil, err
		}
		if err = work.Validate(); err != nil {
			return nil, err
		}
		a := doc.Assignment
		if work.GroupID != row.GroupID || work.Instance != a.Instance || work.CourseID != a.CourseID || work.AssignmentID != a.AssignmentID || work.GroupCategoryID != a.GroupCategoryID {
			return nil, fmt.Errorf("group %d work file belongs to a different origin, course, assignment, set or group", row.GroupID)
		}
		if !groupUsesOnlyLateTiming(row) && !SameGradingBasis(work.Assignment, doc.Assignment) {
			return nil, fmt.Errorf("group %d work grading basis differs from results", row.GroupID)
		}
		src := groupSource{work: work, students: map[string]workspace.Student{}, members: map[string]domain.GroupWorkMember{}}
		for _, art := range work.Artifacts {
			if art.Key == row.ArtifactKey {
				src.artifact = art
			}
		}
		if src.artifact.Key == "" && row.Operation != "late-only" {
			return nil, fmt.Errorf("group %d artifact %q is absent from the pulled work", row.GroupID, row.ArtifactKey)
		}
		for _, m := range work.Members {
			src.members[m.StudentRef] = m
		}
		for _, ref := range row.MemberRefs {
			st, err := store.Resolve(s.Client.Origin(), ref)
			if err != nil {
				return nil, err
			}
			src.students[ref] = st
		}
		if row.Operation == "late-only" {
			src.hold = groupLateStaticHold(row, work, src.members)
		} else if row.Operation != "individual" {
			src.hold = groupStaticHold(row, work, src.members)
		}
		out[row.GroupID] = src
	}
	return out, nil
}

// groupStaticHold is every condition knowable from the pulled work alone.
func groupStaticHold(row domain.GroupResult, work domain.GroupWorkDocument, members map[string]domain.GroupWorkMember) string {
	if work.SharedAssessmentStatus != "ready" {
		reason := "pulled evidence is not ready for shared assessment"
		if len(work.SharedAssessmentReasons) > 0 {
			reason += ": " + work.SharedAssessmentReasons[0]
		}
		return reason
	}
	if !work.PropagationAudienceComplete {
		return "the assignment-specific group audience was not fully read; shared grading cannot be assumed"
	}
	requested := append([]string(nil), row.MemberRefs...)
	audience := append([]string(nil), work.PropagationAudience...)
	sort.Strings(requested)
	sort.Strings(audience)
	if fmt.Sprint(requested) != fmt.Sprint(audience) {
		return "requested members differ from the group's reviewed audience; a subset or changed list cannot become a group grade"
	}
	attempt := 0
	for _, ref := range row.MemberRefs {
		m, ok := members[ref]
		if !ok || !m.CurrentMember || m.SubmissionGroupID != row.GroupID || m.Attempt <= 0 || m.CurrentArtifactKey != row.ArtifactKey {
			return "member " + ref + " does not have the selected artifact as a current submission in this group"
		}
		if m.Excused || (m.AssignmentVisible != nil && !*m.AssignmentVisible) {
			return "member " + ref + " is excused or cannot see the assignment; shared grade would reach them"
		}
		if attempt != 0 && m.Attempt != attempt {
			return "members have different current attempts"
		}
		attempt = m.Attempt
	}
	return ""
}

func groupLateStaticHold(row domain.GroupResult, work domain.GroupWorkDocument, members map[string]domain.GroupWorkMember) string {
	if !work.PropagationAudienceComplete {
		return "the assignment-specific group audience was not fully read; timing cannot be assumed to affect only these members"
	}
	requested := append([]string(nil), row.MemberRefs...)
	audience := append([]string(nil), work.PropagationAudience...)
	sort.Strings(requested)
	sort.Strings(audience)
	if fmt.Sprint(requested) != fmt.Sprint(audience) {
		return "requested members differ from the reviewed assignment audience; a subset cannot become a group timing write"
	}
	attempt := 0
	due := row.LateOnly.LateTiming.DueAt
	for _, ref := range row.MemberRefs {
		m, ok := members[ref]
		if !ok || !m.CurrentMember || m.SubmissionGroupID != row.GroupID || m.Attempt <= 0 || m.SubmissionID <= 0 {
			return "member " + ref + " does not have a current submission attempt in this group"
		}
		if m.Excused || (m.AssignmentVisible != nil && !*m.AssignmentVisible) {
			return "member " + ref + " is excused or cannot see the assignment; timing would reach them"
		}
		if m.EffectiveDueAt == "" || !sameDate(&m.EffectiveDueAt, &due) {
			return "members do not share the effective due date used for this timing calculation"
		}
		if attempt != 0 && m.Attempt != attempt {
			return "members have different current attempts"
		}
		attempt = m.Attempt
		baseline, ok := row.LateOnly.ObservedMembers[ref]
		if !ok || !sameObservedTiming(m.ObservedTiming, &baseline) {
			return "member " + ref + " timing baseline differs from the pulled group work"
		}
	}
	return ""
}

func SameGroupTimingMode(want, live domain.AssignmentSnapshot) bool {
	return want.Instance == live.Instance && want.CourseID == live.CourseID && want.AssignmentID == live.AssignmentID &&
		want.GroupCategoryID > 0 && want.GroupCategoryID == live.GroupCategoryID &&
		!want.NewQuiz && !live.NewQuiz && !want.AnonymousGrading && !live.AnonymousGrading && !want.ModeratedGrading && !live.ModeratedGrading &&
		!hasTimingCheckpoints(want) && !hasTimingCheckpoints(live) &&
		isNumericGradingType(want.GradingType) && isNumericGradingType(live.GradingType) &&
		want.GradeGroupStudentsIndividually != nil && !*want.GradeGroupStudentsIndividually &&
		live.GradeGroupStudentsIndividually != nil && !*live.GradeGroupStudentsIndividually
}

func hasTimingCheckpoints(a domain.AssignmentSnapshot) bool {
	return (a.HasSubAssignments != nil && *a.HasSubAssignments) || (a.Checkpoints != nil && len(*a.Checkpoints) > 0)
}

func isNumericGradingType(value string) bool {
	return value == "points" || value == "percent"
}

func SameGroupIndividualTimingMode(want, live domain.AssignmentSnapshot) bool {
	return want.Instance == live.Instance && want.CourseID == live.CourseID && want.AssignmentID == live.AssignmentID &&
		want.GroupCategoryID > 0 && want.GroupCategoryID == live.GroupCategoryID &&
		!want.NewQuiz && !live.NewQuiz && !want.AnonymousGrading && !live.AnonymousGrading && !want.ModeratedGrading && !live.ModeratedGrading &&
		isNumericGradingType(want.GradingType) && isNumericGradingType(live.GradingType) &&
		want.GradeGroupStudentsIndividually != nil && *want.GradeGroupStudentsIndividually &&
		live.GradeGroupStudentsIndividually != nil && *live.GradeGroupStudentsIndividually
}

func sameObservedTiming(a, b *domain.ObservedTiming) bool {
	if !knownObservedTiming(a) || !knownObservedTiming(b) || a.Late != b.Late || a.SecondsLate != b.SecondsLate {
		return false
	}
	if a.LatePolicyStatus == nil {
		if b.LatePolicyStatus != nil {
			return false
		}
	} else if b.LatePolicyStatus == nil || *a.LatePolicyStatus != *b.LatePolicyStatus {
		return false
	}
	return !a.CustomStatusKnown || (b.CustomStatusKnown && a.HasCustomStatus == b.HasCustomStatus)
}

// groupPreflight renews every live precondition immediately before one
// propagated action: group identity, current enrolled membership, the
// assignment-specific audience, and each member's attempt and evidence.
func (s *Service) groupPreflight(ctx context.Context, doc domain.GroupResultsDocument, row domain.GroupResult, src groupSource, timingOnly ...bool) (groupLive, string, error) {
	a := doc.Assignment
	live := groupLive{subs: map[string]Submission{}}
	if liveA, err := GetAssignment(ctx, s.Client, a.CourseID, a.AssignmentID); err != nil {
		return live, "assignment could not be re-read: " + err.Error(), nil
	} else if len(timingOnly) > 0 && timingOnly[0] && lateTimingAssignmentHold(liveA, true) != "" {
		return live, lateTimingAssignmentHold(liveA, true), nil
	} else if basis, err := groupTimingSnapshot(liveA, s.Client.Origin(), len(timingOnly) > 0 && timingOnly[0]); err != nil {
		return live, "", err
	} else if (len(timingOnly) > 0 && timingOnly[0] && !SameGroupTimingMode(a, basis)) || ((len(timingOnly) == 0 || !timingOnly[0]) && !SameGradingBasis(a, basis)) {
		return live, "assignment, rubric or group grading mode changed; pull-work for reassessment", nil
	}
	var g groupPullIndex
	if err := s.Client.Get(ctx, fmt.Sprintf("/api/v1/groups/%d", row.GroupID), &g); err != nil {
		return live, "group could not be read: " + err.Error(), nil
	}
	if g.ID != row.GroupID || g.CourseID != a.CourseID || g.ContextType != "Course" || g.GroupCategoryID != a.GroupCategoryID || nonCollaborative(g.NonCollaborative) {
		return live, "group is no longer a collaborative group of this assignment's set and course", nil
	}
	enrollments, err := StudentEnrollments(ctx, s.Client, a.CourseID)
	if err != nil {
		return live, "enrollments could not be read: " + err.Error(), nil
	}
	active := map[int64]bool{}
	for _, e := range enrollments {
		id := e.UserID
		if id == 0 {
			id = e.User.ID
		}
		if id > 0 && e.State == "active" {
			active[id] = true
		}
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return live, "", err
	}
	roster, err := s.groupRosterScoped(ctx, a.CourseID, "", a.GroupCategoryID, g, active, store)
	if err != nil {
		return live, "current group membership could not be read: " + err.Error(), nil
	}
	requested := map[string]bool{}
	for _, ref := range row.MemberRefs {
		requested[ref] = true
		if !active[src.students[ref].UserID] {
			return live, "member " + ref + " is no longer actively enrolled", nil
		}
	}
	if !roster.Roster.RosterComplete {
		return live, "current group membership could not be mapped completely", nil
	}
	current := map[string]bool{}
	for _, m := range roster.Roster.Groups[0].Members {
		current[m.StudentRef] = true
	}
	if !sameKeys(current, requested) {
		return live, "group membership changed since the reviewed pull", nil
	}
	rep := row.Representative
	if rep == "" {
		rep = chooseRepresentative(row)
	}
	live.rep = src.students[rep]
	var audience []groupAudienceMember
	if err = s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments/%d/users/%d/group_members?per_page=100", a.CourseID, a.AssignmentID, live.rep.UserID), &audience); err != nil {
		return live, "assignment-specific audience could not be read: " + err.Error(), nil
	}
	heard := map[string]bool{}
	for _, item := range audience {
		id, perr := parseGroupAudienceID(item.ID)
		st, ok := roster.ByUserID[id]
		if perr != nil || id <= 0 || !ok {
			return live, "assignment audience includes someone who cannot be mapped to a reviewed member", nil
		}
		heard[st.Ref] = true
	}
	if !sameKeys(heard, requested) {
		return live, "Canvas's assignment audience differs from the reviewed members; the grade would reach a different set of students", nil
	}
	for _, ref := range row.MemberRefs {
		var hold string
		if len(timingOnly) > 0 && timingOnly[0] {
			hold = s.checkGroupTimingMember(ctx, a, row, src, ref, &live)
		} else {
			hold = s.checkGroupMember(ctx, a, row, src, ref, &live)
		}
		if hold != "" {
			return live, hold, nil
		}
	}
	return live, "", nil
}

func groupTimingSnapshot(a Assignment, origin string, timingOnly bool) (domain.AssignmentSnapshot, error) {
	if timingOnly {
		return TimingSnapshot(a, origin)
	}
	return Snapshot(a, origin)
}

// checkGroupMember compares one member's live submission with the pulled work.
func (s *Service) checkGroupMember(ctx context.Context, a domain.AssignmentSnapshot, row domain.GroupResult, src groupSource, ref string, live *groupLive) string {
	student := src.students[ref]
	m := src.members[ref]
	sub, err := GetGroupSubmission(ctx, s.Client, a.CourseID, a.AssignmentID, student.UserID)
	if err != nil {
		return "submission for " + ref + " could not be read: " + err.Error()
	}
	user := sub.UserID
	if user <= 0 {
		user = sub.User.ID
	}
	if user != student.UserID || (sub.AssignmentID != 0 && sub.AssignmentID != a.AssignmentID) {
		return "live submission identity for " + ref + " disagrees with the requested target"
	}
	if submissionGroupID(sub) != row.GroupID {
		return "submission for " + ref + " no longer belongs to this group"
	}
	if sub.ID != m.SubmissionID || !sameAttempt(domain.AttemptRef{Attempt: m.Attempt}, attemptRef(sub)) {
		return "new submission attempt for " + ref + " since pull; pull-work for this group"
	}
	expected := src.artifact
	for _, art := range src.work.Artifacts {
		if art.Key == m.CurrentArtifactKey {
			expected = art
		}
	}
	if submissionFingerprint(sub) != expected.Fingerprint {
		return "submitted work for " + ref + " changed since pull"
	}
	if sub.Excused {
		return "member " + ref + " is excused; shared grade would reach them"
	}
	if reason := unsupportedSubmission(sub); reason != "" {
		return reason
	}
	live.subs[ref] = sub
	return ""
}

func (s *Service) checkGroupTimingMember(ctx context.Context, a domain.AssignmentSnapshot, row domain.GroupResult, src groupSource, ref string, live *groupLive) string {
	student := src.students[ref]
	m := src.members[ref]
	sub, err := GetGroupSubmission(ctx, s.Client, a.CourseID, a.AssignmentID, student.UserID)
	if err != nil {
		return "submission for " + ref + " could not be read: " + err.Error()
	}
	user := sub.UserID
	if user <= 0 {
		user = sub.User.ID
	}
	if user != student.UserID || (sub.AssignmentID != 0 && sub.AssignmentID != a.AssignmentID) {
		return "live submission identity for " + ref + " disagrees with the requested target"
	}
	if submissionGroupID(sub) != row.GroupID || sub.ID != m.SubmissionID || !sameAttempt(domain.AttemptRef{Attempt: m.Attempt}, attemptRef(sub)) {
		return "submission attempt or group changed for " + ref + "; pull-work before adjusting timing"
	}
	if sub.Excused || (sub.AssignmentVisible != nil && !*sub.AssignmentVisible) {
		return "member " + ref + " is excused or cannot see the assignment; timing would reach them"
	}
	if sub.CachedDueDate == nil || !sameDate(sub.CachedDueDate, &row.LateOnly.LateTiming.DueAt) {
		return "member " + ref + " live due date differs from the common timing deadline"
	}
	timing := row.LateOnly.LateTiming
	if provenance, ok := row.LateOnly.MemberSchoolTime[ref]; ok {
		timing.SchoolTime = &provenance
	}
	if reason := s.schoolTimeHold(sub, &timing); reason != "" {
		return "member " + ref + ": " + reason
	}
	baseline := row.LateOnly.ObservedMembers[ref]
	if !timingMatchesBaseline(sub, &baseline) && !(ambiguous(row.LateOnly.Outcome) && timingMatches(sub, row.LateOnly.LateTiming)) {
		return "Canvas timing changed for " + ref + " since the group timing baseline was pulled"
	}
	if hasCustomStatus(sub) {
		return "Canvas has a custom grading status for " + ref + "; changing late timing would clear it"
	}
	live.subs[ref] = sub
	return ""
}

func (s *Service) checkGroupIndividualTimingMember(ctx context.Context, a domain.AssignmentSnapshot, row domain.GroupResult, src groupSource, ref string, baseline domain.ObservedTiming, timing domain.LateTiming, live *groupLive) string {
	student := src.students[ref]
	m := src.members[ref]
	sub, err := GetGroupSubmission(ctx, s.Client, a.CourseID, a.AssignmentID, student.UserID)
	if err != nil {
		return "submission for " + ref + " could not be read: " + err.Error()
	}
	user := sub.UserID
	if user <= 0 {
		user = sub.User.ID
	}
	if user != student.UserID || (sub.AssignmentID != 0 && sub.AssignmentID != a.AssignmentID) || submissionGroupID(sub) != row.GroupID || sub.ID != m.SubmissionID || !sameAttempt(domain.AttemptRef{Attempt: m.Attempt}, attemptRef(sub)) {
		return "submission identity, group or attempt changed for " + ref + "; pull-work before adjusting timing"
	}
	if sub.Excused || (sub.AssignmentVisible != nil && !*sub.AssignmentVisible) {
		return "member " + ref + " is excused or cannot see the assignment; timing was left unchanged"
	}
	if sub.CachedDueDate == nil || !sameDate(sub.CachedDueDate, &timing.DueAt) {
		return "live due date changed for " + ref + "; recompute timing"
	}
	if reason := s.schoolTimeHold(sub, &timing); reason != "" {
		return reason
	}
	if !timingMatchesBaseline(sub, &baseline) && !(ambiguous(individualLateOutcome(row, ref)) && timingMatches(sub, timing)) {
		return "Canvas timing changed for " + ref + " since it was pulled"
	}
	if hasCustomStatus(sub) {
		return "Canvas has a custom grading status for " + ref + "; changing late timing would clear it"
	}
	live.subs[ref] = sub
	return ""
}

func individualLateOutcome(row domain.GroupResult, ref string) domain.ActionOutcome {
	for _, student := range row.Students {
		if student.StudentRef == ref && student.LateOnly != nil {
			return student.LateOnly.Outcome
		}
	}
	return domain.ActionOutcome{}
}

func (s *Service) postGroupLateOnly(ctx context.Context, doc *domain.GroupResultsDocument, row *domain.GroupResult, src groupSource, save func() error) error {
	target := &row.LateOnly.LateTiming
	if !membersShareDue(src.members, row.MemberRefs, target.DueAt) {
		holdOutcome(&row.LateOnly.Outcome, "members do not share the effective due date used for this timing calculation")
		return save()
	}
	live, hold, err := s.groupPreflight(ctx, *doc, *row, src, true)
	if err != nil {
		return err
	}
	if hold == "" && !commonDueDate(live, target.DueAt) {
		hold = "live members do not share the effective due date used for this timing calculation"
	}
	if hold != "" {
		holdOutcome(&row.LateOnly.Outcome, hold)
		return save()
	}
	allMatch := true
	for _, ref := range row.MemberRefs {
		allMatch = allMatch && timingMatches(live.subs[ref], *target)
	}
	if allMatch {
		row.LateOnly.Outcome = domain.ActionOutcome{Status: "preserved", Message: "requested lateness timing is present for every reviewed member"}
		return save()
	}
	if ambiguous(row.LateOnly.Outcome) {
		row.LateOnly.Outcome.Status = "uncertain"
		row.LateOnly.Outcome.Message = "shared timing completion remains uncertain; requested timing is not present for every reviewed member"
		return save()
	}
	representative := row.Representative
	if representative == "" {
		representative = chooseRepresentative(*row)
		row.Representative = representative
		if err := save(); err != nil {
			return err
		}
	}
	baseline := row.LateOnly.ObservedMembers[representative]
	target.Observed = &baseline
	if provenance, ok := row.LateOnly.MemberSchoolTime[representative]; ok {
		target.SchoolTime = &provenance
	}
	return s.writeLateTiming(ctx, doc.Assignment, src.students[representative].UserID, target, &row.LateOnly.Outcome, save)
}

func sameKeys(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// chooseRepresentative is deterministic: the lowest member ref.
func chooseRepresentative(row domain.GroupResult) string {
	refs := append([]string(nil), row.MemberRefs...)
	sort.Strings(refs)
	return refs[0]
}

func groupRoute(a domain.AssignmentSnapshot, userID int64) string {
	return fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions/%d", a.CourseID, a.AssignmentID, userID)
}

func (s *Service) postGroupShared(ctx context.Context, doc *domain.GroupResultsDocument, row *domain.GroupResult, v domain.ValidatedGroup, src groupSource, coupledRubric bool, save func() error) error {
	a := doc.Assignment
	if row.Representative == "" {
		row.Representative = chooseRepresentative(*row)
		if err := save(); err != nil {
			return err
		}
	}
	live, hold, err := s.groupPreflight(ctx, *doc, *row, src)
	if err != nil {
		return err
	}
	if hold != "" {
		holdGroupShared(row, hold)
		return save()
	}
	attempt := src.members[row.Representative].Attempt
	if row.CommentOnly != nil {
		return s.postGroupComment(ctx, doc, row, v, &row.CommentOnly.Outcome, live, attempt, false, save)
	}
	g := row.Grade
	repID := live.rep.UserID
	noCanvasRubric := a.Rubric == nil
	if noCanvasRubric && !definite(g.Outcomes.Rubric) {
		g.Outcomes.Rubric = domain.ActionOutcome{Status: "not-requested", Message: "assignment has no Canvas rubric"}
	}
	repSub := live.subs[row.Representative]
	// Unknown writes are reconciled, never replayed based on an empty lookup.
	if ambiguous(g.Outcomes.Rubric) {
		if rubricMatches(repSub, *v.Assessment) {
			g.Outcomes.Rubric = domain.ActionOutcome{Status: "preserved", Message: "intended rubric values are present"}
		} else {
			g.Outcomes.Rubric.Status = "uncertain"
			g.Outcomes.Rubric.Message = "rubric write completion remains uncertain; inspect current assessment"
		}
	}
	if ambiguous(g.Outcomes.Grade) {
		if grade := decimalPointer(repSub.EnteredScore); grade != nil && grade.Cmp(g.Assessment) == 0 {
			g.Outcomes.Grade = domain.ActionOutcome{Status: "preserved", Message: "computed assessment is present on intended attempt"}
		} else {
			g.Outcomes.Grade.Status = "uncertain"
			g.Outcomes.Grade.Message = "grade completion remains uncertain; a different live score is not proof of failure"
		}
	}
	if err = save(); err != nil {
		return err
	}
	route := groupRoute(a, repID)
	// A coupled rubric's raw sum would replace the computed assessment when scales
	// differ: post the honest rubric first, then the computed grade as its own action.
	if coupledRubric && !definite(g.Outcomes.Rubric) && !ambiguous(g.Outcomes.Rubric) && !ambiguous(g.Outcomes.Grade) {
		g.Outcomes.Rubric = domain.ActionOutcome{Status: "pending"}
		g.Outcomes.Grade = domain.ActionOutcome{}
		if err = save(); err != nil {
			return err
		}
		if e := s.Client.Write(ctx, "PUT", route, map[string]any{"rubric_assessment": rubricFields(*v.Assessment)}, nil); e != nil {
			g.Outcomes.Rubric = failedOutcome(e)
		} else {
			g.Outcomes.Rubric = domain.ActionOutcome{Status: "succeeded"}
		}
		if err = save(); err != nil {
			return err
		}
	}
	if !noCanvasRubric && !coupledRubric && !ambiguous(g.Outcomes.Rubric) && !ambiguous(g.Outcomes.Grade) && (!definite(g.Outcomes.Rubric) || !definite(g.Outcomes.Grade)) {
		g.Outcomes.Rubric = domain.ActionOutcome{Status: "pending"}
		g.Outcomes.Grade = domain.ActionOutcome{Status: "pending"}
		if err = save(); err != nil {
			return err
		}
		if e := s.Client.Write(ctx, "PUT", route, map[string]any{"rubric_assessment": rubricFields(*v.Assessment), "submission": map[string]any{"posted_grade": g.Assessment.String()}}, nil); e != nil {
			g.Outcomes.Rubric, g.Outcomes.Grade = failedOutcome(e), failedOutcome(e)
		} else {
			g.Outcomes.Rubric = domain.ActionOutcome{Status: "succeeded"}
			g.Outcomes.Grade = domain.ActionOutcome{Status: "succeeded"}
		}
		if err = save(); err != nil {
			return err
		}
	}
	if (noCanvasRubric || (coupledRubric && definite(g.Outcomes.Rubric))) && !definite(g.Outcomes.Grade) && !ambiguous(g.Outcomes.Grade) {
		// A distinct action: renew the audience and attempt checks first.
		_, hold, perr := s.groupPreflight(ctx, *doc, *row, src)
		if perr != nil {
			return perr
		}
		if hold != "" {
			holdOutcome(&g.Outcomes.Grade, hold)
		} else {
			g.Outcomes.Grade = domain.ActionOutcome{Status: "pending"}
			if err = save(); err != nil {
				return err
			}
			if e := s.Client.Write(ctx, "PUT", route, map[string]any{"submission": map[string]any{"posted_grade": g.Assessment.String()}}, nil); e != nil {
				g.Outcomes.Grade = failedOutcome(e)
			} else {
				g.Outcomes.Grade = domain.ActionOutcome{Status: "succeeded"}
			}
		}
		if err = save(); err != nil {
			return err
		}
	}
	if !definite(g.Outcomes.Comment) {
		if g.FeedbackAudience == "none" {
			g.Outcomes.Comment = domain.ActionOutcome{Status: "not-requested", Message: "no shared feedback requested"}
		} else {
			fresh, hold, perr := s.groupPreflight(ctx, *doc, *row, src)
			if perr != nil {
				return perr
			}
			if hold != "" {
				holdOutcome(&g.Outcomes.Comment, hold)
			} else if err = s.postGroupComment(ctx, doc, row, v, &g.Outcomes.Comment, fresh, attempt, g.LateTiming != nil && !membersShareDue(src.members, row.MemberRefs, g.LateTiming.DueAt), save); err != nil {
				return err
			}
		}
		if err = save(); err != nil {
			return err
		}
	}
	if g.LateTiming == nil {
		if !definite(g.Outcomes.Late) {
			g.Outcomes.Late = domain.ActionOutcome{Status: "not-requested"}
		}
	} else if !definite(g.Outcomes.Late) {
		fresh, hold, perr := s.groupPreflight(ctx, *doc, *row, src)
		if perr != nil {
			return perr
		}
		switch {
		case hold != "":
			holdOutcome(&g.Outcomes.Late, hold)
		case !membersShareDue(src.members, row.MemberRefs, g.LateTiming.DueAt) || !commonDueDate(fresh, g.LateTiming.DueAt):
			// Shared lateness propagates, so it needs one timing that is true for everyone.
			g.Outcomes.Late = domain.ActionOutcome{Status: "not-requested", Message: "members have different effective due dates; late timing was omitted and the grade and feedback continued"}
		default:
			if timingHold := s.checkGroupGradeLateTiming(*row, src, fresh, g.LateTiming); timingHold != "" {
				holdOutcome(&g.Outcomes.Late, timingHold)
			} else {
				if baseline, ok := src.members[row.Representative].ObservedTiming, row.Representative != ""; ok && g.LateTiming.Observed != nil {
					g.LateTiming.Observed = baseline
				}
				if provenance, ok := g.MemberSchoolTime[row.Representative]; ok {
					g.LateTiming.SchoolTime = &provenance
				}
				if err = s.postLateTiming(ctx, a, repID, domain.AttemptRef{Attempt: attempt}, g.LateTiming, &g.Outcomes.Late, save); err != nil {
					return err
				}
			}
		}
	}
	return save()
}

func (s *Service) checkGroupGradeLateTiming(row domain.GroupResult, src groupSource, live groupLive, timing *domain.LateTiming) string {
	if timing == nil || (timing.Observed == nil && timing.SchoolTime == nil) {
		return ""
	}
	for _, ref := range row.MemberRefs {
		member := src.members[ref]
		sub, ok := live.subs[ref]
		if !ok {
			return "live timing for member " + ref + " could not be verified; grade and feedback may continue"
		}
		if member.ObservedTiming == nil || !knownObservedTiming(member.ObservedTiming) || !timingMatchesBaseline(sub, member.ObservedTiming) {
			return "Canvas timing changed or was not observed for member " + ref + "; the timing action was held while grade and feedback continued"
		}
		if hasCustomStatus(sub) {
			return "member " + ref + " has a custom grading status; timing was held while grade and feedback continued"
		}
		memberTiming := *timing
		if timing.SchoolTime != nil {
			provenance, ok := row.Grade.MemberSchoolTime[ref]
			if !ok {
				return "school-time provenance is missing for member " + ref + "; timing was held while grade and feedback continued"
			}
			memberTiming.SchoolTime = &provenance
		}
		if reason := s.schoolTimeHold(sub, &memberTiming); reason != "" {
			return "member " + ref + ": " + reason + "; grade and feedback continued"
		}
	}
	return ""
}

// membersShareDue reports whether every reviewed member's pulled effective due
// date matches the one late timing was worked out against.
func membersShareDue(members map[string]domain.GroupWorkMember, refs []string, due string) bool {
	for _, ref := range refs {
		m, ok := members[ref]
		if !ok || m.EffectiveDueAt == "" || !sameDate(&m.EffectiveDueAt, &due) {
			return false
		}
	}
	return true
}

func commonDueDate(live groupLive, due string) bool {
	for _, sub := range live.subs {
		if sub.CachedDueDate == nil || !sameDate(sub.CachedDueDate, &due) {
			return false
		}
	}
	return true
}

// postGroupComment sends one whole-group comment through the representative
// route with comment[group_comment]=true.
func (s *Service) postGroupComment(ctx context.Context, doc *domain.GroupResultsDocument, row *domain.GroupResult, v domain.ValidatedGroup, out *domain.ActionOutcome, live groupLive, attempt int, omitLate bool, save func() error) error {
	text, err := GroupSharedText(*doc, *row, v, omitLate)
	if err != nil {
		return err
	}
	if out.PostedText != "" && ambiguous(*out) {
		text = out.PostedText
	}
	has := func(sub Submission) bool {
		for _, c := range commentsForAttempt(sub, attempt) {
			if c.Comment == text {
				return true
			}
		}
		return false
	}
	if ambiguous(*out) {
		// Only a copy of the exact text in every member's submission is evidence of a
		// shared comment; one equal individual comment is not.
		all := true
		for _, ref := range row.MemberRefs {
			all = all && has(live.subs[ref])
		}
		if all {
			*out = domain.ActionOutcome{Status: "preserved", PostedText: text, Message: "exact comment text is present for every member"}
		} else {
			out.Status = "uncertain"
			out.Message = "shared comment completion remains uncertain; inspect Canvas before repeating it"
		}
		return save()
	}
	allHave := true
	for _, ref := range row.MemberRefs {
		allHave = allHave && has(live.subs[ref])
	}
	if allHave {
		*out = domain.ActionOutcome{Status: "preserved", PostedText: text, Message: "exact intended text is already present for every member; ownership not assumed"}
		return save()
	}
	*out = domain.ActionOutcome{Status: "pending", PostedText: text}
	if err = save(); err != nil {
		return err
	}
	comment := map[string]any{"text_comment": text, "attempt": attempt, "group_comment": true}
	if e := s.Client.Write(ctx, "PUT", groupRoute(doc.Assignment, live.rep.UserID), map[string]any{"comment": comment}, nil); e != nil {
		*out = failedOutcome(e)
		out.PostedText = text
	} else {
		*out = domain.ActionOutcome{Status: "succeeded", PostedText: text}
	}
	return save()
}

// postGroupPrivate sends one private comment to one member (group_comment=false).
func (s *Service) postGroupPrivate(ctx context.Context, doc domain.GroupResultsDocument, row *domain.GroupResult, f *domain.IndividualFeedback, src groupSource, save func() error) error {
	student, ok := src.students[f.StudentRef]
	member, known := src.members[f.StudentRef]
	if !ok || !known || member.Attempt <= 0 {
		holdOutcome(&f.Outcome, "member has no pulled submission attempt for this group")
		return save()
	}
	probe := groupLive{subs: map[string]Submission{}}
	if hold := s.checkGroupMember(ctx, doc.Assignment, *row, src, f.StudentRef, &probe); hold != "" {
		holdOutcome(&f.Outcome, hold)
		return save()
	}
	sub := probe.subs[f.StudentRef]
	text := workspace.ReplaceStudentName(f.Text, student)
	if f.Outcome.PostedText != "" && ambiguous(f.Outcome) {
		text = f.Outcome.PostedText
	}
	present := false
	for _, c := range commentsForAttempt(sub, member.Attempt) {
		present = present || c.Comment == text
	}
	if present {
		f.Outcome = domain.ActionOutcome{Status: "preserved", PostedText: text, Message: "exact intended text is already present; ownership not assumed"}
		return save()
	}
	if ambiguous(f.Outcome) {
		f.Outcome.Status = "uncertain"
		f.Outcome.Message = "private comment completion remains uncertain; not repeated"
		return save()
	}
	f.Outcome = domain.ActionOutcome{Status: "pending", PostedText: text}
	if err := save(); err != nil {
		return err
	}
	comment := map[string]any{"text_comment": text, "attempt": member.Attempt, "group_comment": false}
	if e := s.Client.Write(ctx, "PUT", groupRoute(doc.Assignment, student.UserID), map[string]any{"comment": comment}, nil); e != nil {
		f.Outcome = failedOutcome(e)
		f.Outcome.PostedText = text
	} else {
		f.Outcome = domain.ActionOutcome{Status: "succeeded", PostedText: text}
	}
	return save()
}

// buildGroupResultsReview renders what PostGroupResults would send: one
// indivisible card per group's shared action and one card per private comment.
func buildGroupResultsReview(root, path string) (ReviewModel, error) {
	var doc domain.GroupResultsDocument
	if _, e := workspace.ReadDocument(path, &doc); e != nil {
		return ReviewModel{}, e
	}
	validated, e := domain.ValidateGroupResults(&doc)
	if e != nil {
		return ReviewModel{}, e
	}
	packetPath, e := workspace.ConstrainPath(root, doc.Packet)
	if e != nil {
		return ReviewModel{}, e
	}
	var packet domain.GroupPacketDocument
	if _, e = workspace.ReadDocument(packetPath, &packet); e != nil {
		return ReviewModel{}, e
	}
	paths := map[int64]string{}
	names := map[int64]string{}
	for _, g := range packet.Groups {
		paths[g.GroupID], names[g.GroupID] = g.Path, g.Name
	}
	store, _ := workspace.OpenStudents(root)
	m := ReviewModel{Kind: "Shared group assessment", Title: doc.Assignment.Title, Notice: "A shared grade and rubric reach every member of the group together, so each group card is approved or revised as a whole. Private comments are separate cards."}
	for i, row := range doc.Groups {
		v := validated[i]
		if row.Operation == "individual" {
			m.Cards = append(m.Cards, individualReviewCards(root, doc, row, v, paths[row.GroupID], names[row.GroupID], store)...)
			continue
		}
		rel, ok := paths[row.GroupID]
		var work domain.GroupWorkDocument
		hold := v.Hold
		if !ok {
			hold = fmt.Sprintf("group %d is absent from the source packet", row.GroupID)
		} else if p, pe := workspace.ConstrainPath(root, rel); pe != nil {
			hold = pe.Error()
		} else if _, re := workspace.ReadDocument(p, &work); re != nil {
			hold = re.Error()
		}
		members := map[string]domain.GroupWorkMember{}
		aliases := map[string]string{}
		for _, mem := range work.Members {
			members[mem.StudentRef] = mem
			aliases[mem.StudentRef] = mem.Alias
		}
		for _, mem := range work.Membership {
			aliases[mem.StudentRef] = mem.Alias
		}
		if hold == "" && row.Operation != "late-only" {
			var artifactFound bool
			for _, a := range work.Artifacts {
				artifactFound = artifactFound || a.Key == row.ArtifactKey
			}
			if !artifactFound {
				hold = fmt.Sprintf("artifact %q is absent from the pulled work", row.ArtifactKey)
			} else {
				hold = groupStaticHold(row, work, members)
			}
		} else if hold == "" {
			hold = groupLateStaticHold(row, work, members)
		}
		memberAliases := []string{}
		for _, ref := range row.MemberRefs {
			label := aliases[ref]
			if label == "" {
				label = ref
			}
			memberAliases = append(memberAliases, label)
		}
		c := ReviewCard{Ref: GroupCardRef(row.GroupID), Alias: names[row.GroupID], Operation: row.Operation, Hold: hold, Done: groupSharedComplete(row)}
		c.Facts = []ReviewFact{{"Group", fmt.Sprintf("%s (group %d)", names[row.GroupID], row.GroupID)}, {"Affects", strings.Join(memberAliases, ", ")}}
		if row.Operation == "late-only" {
			c.Facts = append(c.Facts, ReviewFact{"Grading", "late timing only; no grade, rubric or comment will change"})
			c.Late = lateSummary(row.LateOnly.LateTiming)
			c.LateBasis = row.LateOnly.LateTiming.Basis
			c.Facts = append(c.Facts, timingReviewFacts(row.LateOnly.LateTiming)...)
			for _, ref := range row.MemberRefs {
				baseline := row.LateOnly.ObservedMembers[ref]
				status := "automatic or no explicit status"
				if baseline.LatePolicyStatus != nil {
					status = *baseline.LatePolicyStatus
				}
				label := aliases[ref]
				if label == "" {
					label = ref
				}
				c.Facts = append(c.Facts, ReviewFact{"Current timing for " + label, fmt.Sprintf("status %s; late=%t; seconds=%d", status, baseline.Late, baseline.SecondsLate)})
				if provenance, ok := row.LateOnly.MemberSchoolTime[ref]; ok {
					c.Facts = append(c.Facts,
						ReviewFact{"First genuine submission counted for " + label, fmt.Sprintf("attempt %d at %s", provenance.Attempt.Attempt, provenance.SubmittedAt)},
						ReviewFact{"School-time deadline for " + label, provenance.Deadline},
					)
					if len(provenance.SkippedDates) > 0 {
						c.Facts = append(c.Facts, ReviewFact{"Calendar days skipped for " + label, strings.Join(provenance.SkippedDates, ", ")})
					}
				}
			}
		} else {
			c.Facts = append(c.Facts, ReviewFact{"Artifact", row.ArtifactKey}, ReviewFact{"Grading", "one shared grade for the whole group"})
		}
		if row.Operation != "late-only" {
			for _, a := range work.Artifacts {
				if a.Key == row.ArtifactKey {
					c.Submission = strings.TrimSpace(a.ReadableText)
					for _, att := range a.Attachments {
						c.Submission += "\n[attachment: " + att.Name + "]"
					}
				}
			}
		}
		if g := row.Grade; g != nil {
			c.TeacherNote = g.TeacherNote
			if g.LateTiming != nil {
				c.Late, c.LateBasis = lateSummary(*g.LateTiming), g.LateTiming.Basis
			}
			if v.Assessment != nil {
				c.Total = v.Assessment.Posted.String() + " / " + doc.Assignment.Points.String()
			}
			c.RubricComments = rubricCommentFacts(doc.Assignment.Rubric, v.Assessment)
			if g.FeedbackAudience == "none" {
				c.Facts = append(c.Facts, ReviewFact{"Shared comment", "none"})
			}
		}
		if c.Hold == "" && row.Operation != "late-only" {
			omitLate := row.Grade != nil && row.Grade.LateTiming != nil && !membersShareDue(members, row.MemberRefs, row.Grade.LateTiming.DueAt)
			text, te := GroupSharedText(doc, row, v, omitLate)
			if te != nil {
				c.Hold = te.Error()
			}
			c.Comment = text
		}
		m.Cards = append(m.Cards, c)
		for _, f := range row.IndividualFeedback {
			text := f.Text
			if store != nil {
				if st, se := store.Resolve(doc.Assignment.Instance, f.StudentRef); se == nil {
					text = workspace.ReplaceStudentName(text, st)
				}
			}
			label := aliases[f.StudentRef]
			if label == "" {
				label = f.StudentRef
			}
			pc := ReviewCard{Ref: GroupPrivateRef(row.GroupID, f.StudentRef), Alias: label, Operation: "private comment", Comment: text, Done: definite(f.Outcome)}
			pc.Facts = []ReviewFact{{"Group", fmt.Sprintf("%s (group %d)", names[row.GroupID], row.GroupID)}, {"Visible to", label + " only"}}
			if mem, ok := members[f.StudentRef]; !ok || mem.Attempt <= 0 {
				pc.Hold = "member has no pulled submission attempt for this group"
			} else {
				pc.Attempt = mem.Attempt
			}
			m.Cards = append(m.Cards, pc)
		}
	}
	return m, nil
}

func holdStudentResult(r *domain.StudentResult, message string) {
	if r.Grade != nil {
		holdGrade(r.Grade, message)
	} else if r.CommentOnly != nil {
		holdOutcome(&r.CommentOnly.Outcome, message)
	} else if r.LateOnly != nil {
		holdOutcome(&r.LateOnly.Outcome, message)
	}
}

// postGroupIndividual posts per-student grades/comments on a shared product
// (live grade_group_students_individually: true). Each student's write targets
// only that student's route with group_comment=false; the live mode and that
// student's attempt/evidence are re-checked before each student.
func (s *Service) postGroupIndividual(ctx context.Context, doc *domain.GroupResultsDocument, row *domain.GroupResult, v domain.ValidatedGroup, src groupSource, save func() error) error {
	a := doc.Assignment
	tmp := &domain.ResultsDocument{Schema: 1, Kind: "results", Packet: doc.Packet, Assignment: a, Interpretation: doc.Interpretation, ScoringBasis: doc.ScoringBasis, Variants: doc.Variants, Students: row.Students}
	s.privateGroupComments = true
	defer func() { s.privateGroupComments = false }()
	for i := range row.Students {
		r := &row.Students[i]
		vs := v.Students[i]
		if resultComplete(*r) || s.Skip[GroupStudentRef(row.GroupID, r.StudentRef)] {
			continue
		}
		hold := vs.Hold
		coupled := false
		if hold == "" {
			live, err := GetAssignment(ctx, s.Client, a.CourseID, a.AssignmentID)
			if err != nil {
				hold = "assignment could not be re-read: " + err.Error()
			} else {
				if live.CourseID == 0 {
					live.CourseID = a.CourseID
				}
				if r.Operation == "late-only" {
					hold = lateTimingAssignmentHold(live, true)
				}
				basis, err := groupTimingSnapshot(live, s.Client.Origin(), r.Operation == "late-only")
				if err != nil {
					return err
				}
				if r.Operation == "late-only" {
					if !SameGroupIndividualTimingMode(a, basis) {
						hold = "assignment group grading mode changed; pull-work before adjusting timing"
					}
				} else if !SameGradingBasis(a, basis) {
					hold = "assignment, rubric or group grading mode changed; pull-work for reassessment"
				}
				coupled = basis.Rubric != nil && basis.Rubric.UseForGradingKnown && basis.Rubric.UseForGrading
			}
		}
		student, known := src.students[r.StudentRef]
		m, member := src.members[r.StudentRef]
		attempt := 0
		if r.Grade != nil {
			attempt = r.Grade.Attempt.Attempt
		} else if r.CommentOnly != nil {
			attempt = r.CommentOnly.Attempt.Attempt
		} else if r.LateOnly != nil {
			attempt = r.LateOnly.Attempt.Attempt
		}
		if hold == "" && (!known || !member || !m.CurrentMember || m.SubmissionGroupID != row.GroupID || m.Attempt <= 0 || m.Excused) {
			hold = "member is not a current, non-excused submitter in this group"
		}
		if hold == "" && attempt != m.Attempt {
			hold = "target attempt differs from the pulled work; pull-work for this group"
		}
		var probe groupLive
		if hold == "" {
			probe = groupLive{subs: map[string]Submission{}}
			if r.LateOnly != nil {
				m := src.members[r.StudentRef]
				if m.EffectiveDueAt == "" || !sameDate(&m.EffectiveDueAt, &r.LateOnly.LateTiming.DueAt) || !sameObservedTiming(m.ObservedTiming, r.LateOnly.LateTiming.Observed) {
					hold = "timing deadline or observed baseline differs from the pulled work"
				} else {
					hold = s.checkGroupIndividualTimingMember(ctx, a, *row, src, r.StudentRef, *r.LateOnly.LateTiming.Observed, r.LateOnly.LateTiming, &probe)
				}
			} else {
				hold = s.checkGroupMember(ctx, a, *row, src, r.StudentRef, &probe)
			}
		}
		if hold != "" {
			holdStudentResult(r, hold)
			if err := save(); err != nil {
				return err
			}
			continue
		}
		source := postSource{student: student}
		var err error
		if r.LateOnly != nil {
			err = s.postLateTiming(ctx, a, student.UserID, r.LateOnly.Attempt, &r.LateOnly.LateTiming, &r.LateOnly.Outcome, save)
		} else if r.CommentOnly != nil {
			err = s.postCommentOnly(ctx, tmp, r, vs, source, probe.subs[r.StudentRef], save)
		} else {
			err = s.postGrade(ctx, tmp, r, vs, source, probe.subs[r.StudentRef], coupled, save, func() string {
				live, e := GetAssignment(ctx, s.Client, a.CourseID, a.AssignmentID)
				if e != nil {
					return "assignment could not be re-read: " + e.Error()
				}
				if live.CourseID == 0 {
					live.CourseID = a.CourseID
				}
				basis, e := Snapshot(live, s.Client.Origin())
				if e != nil {
					return e.Error()
				}
				if !SameGradingBasis(a, basis) {
					return "assignment, rubric or group grading mode changed; pull-work for reassessment"
				}
				return s.checkGroupMember(ctx, a, *row, src, r.StudentRef, &groupLive{subs: map[string]Submission{}})
			})
		}
		if err != nil {
			return err
		}
	}
	sf := row.SharedFeedback
	if sf == nil || definite(sf.Outcome) || s.Skip[GroupCardRef(row.GroupID)] {
		return nil
	}
	if row.Representative == "" {
		row.Representative = chooseRepresentative(*row)
		if err := save(); err != nil {
			return err
		}
	}
	hold := v.SharedHold
	if hold == "" {
		hold = individualSharedHold(*row, src)
	}
	var live groupLive
	if hold == "" {
		var err error
		live, hold, err = s.groupPreflight(ctx, *doc, *row, src)
		if err != nil {
			return err
		}
	}
	if hold != "" {
		holdOutcome(&sf.Outcome, hold)
		return save()
	}
	s.privateGroupComments = false
	return s.postGroupComment(ctx, doc, row, v, &sf.Outcome, live, src.members[row.Representative].Attempt, false, save)
}

// individualSharedHold covers the audience conditions of a whole-group comment
// that is posted beside individual grades.
func individualSharedHold(row domain.GroupResult, src groupSource) string {
	if !src.work.PropagationAudienceComplete {
		return "the assignment-specific group audience was not fully read; a group comment cannot be assumed to reach the reviewed members"
	}
	requested := append([]string(nil), row.MemberRefs...)
	audience := append([]string(nil), src.work.PropagationAudience...)
	sort.Strings(requested)
	sort.Strings(audience)
	if fmt.Sprint(requested) != fmt.Sprint(audience) {
		return "requested members differ from the group's reviewed audience; a subset cannot become a group comment"
	}
	attempt := 0
	for _, ref := range row.MemberRefs {
		m, ok := src.members[ref]
		if !ok || m.Attempt <= 0 || m.Excused {
			return "member " + ref + " has no usable current attempt for a group comment"
		}
		if attempt != 0 && m.Attempt != attempt {
			return "members have different current attempts; a group comment needs one attempt"
		}
		attempt = m.Attempt
	}
	return ""
}

// individualReviewCards renders an individual-mode row: one card per student
// (grade and comment move together) and, when requested, one separate card for
// the whole-group comment.
func individualReviewCards(root string, doc domain.GroupResultsDocument, row domain.GroupResult, v domain.ValidatedGroup, rel, groupName string, store *workspace.StudentStore) []ReviewCard {
	var work domain.GroupWorkDocument
	loadHold := ""
	if rel == "" {
		loadHold = fmt.Sprintf("group %d is absent from the source packet", row.GroupID)
	} else if p, err := workspace.ConstrainPath(root, rel); err != nil {
		loadHold = err.Error()
	} else if _, err = workspace.ReadDocument(p, &work); err != nil {
		loadHold = err.Error()
	}
	members := map[string]domain.GroupWorkMember{}
	aliases := map[string]string{}
	for _, mem := range work.Members {
		members[mem.StudentRef] = mem
		aliases[mem.StudentRef] = mem.Alias
	}
	artifacts := map[string]domain.GroupArtifact{}
	for _, a := range work.Artifacts {
		artifacts[a.Key] = a
	}
	basis := doc.Basis()
	var cards []ReviewCard
	for i, r := range row.Students {
		vs := v.Students[i]
		label := aliases[r.StudentRef]
		if label == "" {
			label = r.StudentRef
		}
		c := ReviewCard{Ref: GroupStudentRef(row.GroupID, r.StudentRef), Alias: label, Operation: r.Operation, Hold: vs.Hold, Done: resultComplete(r)}
		c.Facts = []ReviewFact{{"Group", fmt.Sprintf("%s (group %d)", groupName, row.GroupID)}, {"Grading", "individual grade on a shared product; only " + label + " is affected"}}
		var feedback string
		attempt := 0
		switch {
		case r.Grade != nil:
			g := r.Grade
			attempt, feedback, c.TeacherNote = g.Attempt.Attempt, g.Feedback, g.TeacherNote
			if g.LateTiming != nil {
				c.Late, c.LateBasis = lateSummary(*g.LateTiming), g.LateTiming.Basis
			}
			if vs.Assessment != nil {
				c.Total = vs.Assessment.Posted.String() + " / " + doc.Assignment.Points.String()
			}
			c.RubricComments = rubricCommentFacts(doc.Assignment.Rubric, vs.Assessment)
		case r.CommentOnly != nil:
			attempt, feedback = r.CommentOnly.Attempt.Attempt, r.CommentOnly.Feedback
		case r.LateOnly != nil:
			attempt = r.LateOnly.Attempt.Attempt
			c.Late = lateSummary(r.LateOnly.LateTiming)
			c.LateBasis = r.LateOnly.LateTiming.Basis
			c.Facts = append(c.Facts, ReviewFact{"Grading", "late timing only; no grade, rubric or comment will change"})
			c.Facts = append(c.Facts, timingReviewFacts(r.LateOnly.LateTiming)...)
		}
		c.Attempt = attempt
		if c.Hold == "" {
			mem, ok := members[r.StudentRef]
			switch {
			case loadHold != "":
				c.Hold = loadHold
			case !ok || !mem.CurrentMember || mem.Attempt <= 0 || mem.Excused:
				c.Hold = "member is not a current, non-excused submitter in this group"
			case mem.Attempt != attempt:
				c.Hold = "target attempt differs from the pulled work"
			case r.LateOnly != nil && (mem.EffectiveDueAt == "" || !sameDate(&mem.EffectiveDueAt, &r.LateOnly.LateTiming.DueAt) || !sameObservedTiming(mem.ObservedTiming, r.LateOnly.LateTiming.Observed)):
				c.Hold = "timing deadline or observed baseline differs from the pulled work"
			}
		}
		if mem, ok := members[r.StudentRef]; ok && r.LateOnly == nil {
			art := artifacts[mem.CurrentArtifactKey]
			c.Submission = strings.TrimSpace(art.ReadableText)
			for _, att := range art.Attachments {
				c.Submission += "\n[attachment: " + att.Name + "]"
			}
		}
		if store != nil {
			if st, err := store.Resolve(doc.Assignment.Instance, r.StudentRef); err == nil {
				feedback = workspace.ReplaceStudentName(feedback, st)
			}
		}
		if c.Hold == "" && basis != nil {
			if r.Grade != nil {
				text, err := render.RenderFeedback(render.FeedbackInput{Rubric: *basis, Interpretation: vs.Interpretation, Components: r.Grade.Components, Assessment: r.Grade.Assessment, AssignmentPoints: doc.Assignment.Points, Feedback: feedback, Late: isLate(r.Grade.LateTiming), Header: r.Grade.Header})
				if err != nil {
					c.Hold = err.Error()
				}
				c.Comment = text
			} else if r.CommentOnly != nil {
				computed, err := domain.ComputeAssessment(*basis, vs.Interpretation, r.CommentOnly.Components, doc.Assignment.Points)
				if err == nil {
					c.Comment, err = render.RenderFeedback(render.FeedbackInput{Rubric: *basis, Interpretation: vs.Interpretation, Components: r.CommentOnly.Components, Assessment: computed.Posted, AssignmentPoints: doc.Assignment.Points, Feedback: feedback, Header: r.CommentOnly.Header})
				}
				if err != nil {
					c.Hold = err.Error()
				}
			}
		}
		cards = append(cards, c)
	}
	if sf := row.SharedFeedback; sf != nil {
		c := ReviewCard{Ref: GroupCardRef(row.GroupID), Alias: groupName, Operation: "shared comment", Hold: v.SharedHold, Done: definite(sf.Outcome)}
		names := []string{}
		for _, ref := range row.MemberRefs {
			if aliases[ref] != "" {
				names = append(names, aliases[ref])
			} else {
				names = append(names, ref)
			}
		}
		c.Facts = []ReviewFact{{"Group", fmt.Sprintf("%s (group %d)", groupName, row.GroupID)}, {"Visible to", "every member: " + strings.Join(names, ", ")}}
		if c.Hold == "" {
			c.Hold = loadHold
		}
		if c.Hold == "" {
			c.Hold = individualSharedHold(row, groupSource{work: work, members: members})
		}
		if c.Hold == "" {
			text, err := GroupSharedText(doc, row, v, false)
			if err != nil {
				c.Hold = err.Error()
			}
			c.Comment = text
		}
		cards = append(cards, c)
	}
	return cards
}
