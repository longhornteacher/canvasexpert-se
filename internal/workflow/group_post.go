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
	GroupID                  int64
	SharedDone               bool
	Operation, Status, Message string
	Outcomes                 domain.ActionOutcomes
	Private                  map[string]domain.ActionOutcome
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

func groupSharedComplete(r domain.GroupResult) bool {
	if r.Operation == "grade" {
		g := r.Grade
		return g != nil && definite(g.Outcomes.Comment) && definite(g.Outcomes.Rubric) && definite(g.Outcomes.Grade) && definite(g.Outcomes.Late)
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
	}
}

// GroupSharedText renders the whole-group comment exactly as review shows and post sends.
func GroupSharedText(doc domain.GroupResultsDocument, row domain.GroupResult, v domain.ValidatedGroup) (string, error) {
	basis := doc.Basis()
	if basis == nil || v.Assessment == nil {
		return "", fmt.Errorf("scoring basis is absent")
	}
	if g := row.Grade; g != nil {
		if g.FeedbackAudience != "group" {
			return "", nil
		}
		return render.RenderFeedback(render.FeedbackInput{Rubric: *basis, Interpretation: doc.Interpretation, Components: g.Components, Assessment: g.Assessment, AssignmentPoints: doc.Assignment.Points, Feedback: g.Feedback, Late: isLate(g.LateTiming), Header: g.Header})
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
	requests := s.Client.Requests()
	report.Path = path
	defer func() { report.Elapsed = time.Since(start); report.Requests = s.Client.Requests() - requests }()
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
	for _, c := range s.Config.Courses {
		if c.ID == doc.Assignment.CourseID {
			configured = true
		}
	}
	if !configured {
		return report, fmt.Errorf("group results course is not configured")
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
	liveBasis, err := Snapshot(liveAssignment, s.Client.Origin())
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
	if !SameGradingBasis(packet.Assignment, doc.Assignment) {
		return nil, fmt.Errorf("group results grading basis differs from source packet")
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
		if !SameGradingBasis(work.Assignment, doc.Assignment) {
			return nil, fmt.Errorf("group %d work grading basis differs from results", row.GroupID)
		}
		src := groupSource{work: work, students: map[string]workspace.Student{}, members: map[string]domain.GroupWorkMember{}}
		for _, art := range work.Artifacts {
			if art.Key == row.ArtifactKey {
				src.artifact = art
			}
		}
		if src.artifact.Key == "" {
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
		src.hold = groupStaticHold(row, work, src.members)
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

// groupPreflight renews every live precondition immediately before one
// propagated action: group identity, current enrolled membership, the
// assignment-specific audience, and each member's attempt and evidence.
func (s *Service) groupPreflight(ctx context.Context, doc domain.GroupResultsDocument, row domain.GroupResult, src groupSource) (groupLive, string, error) {
	a := doc.Assignment
	live := groupLive{subs: map[string]Submission{}}
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
		hold := s.checkGroupMember(ctx, a, row, src, ref, &live)
		if hold != "" {
			return live, hold, nil
		}
	}
	return live, "", nil
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
	if submissionFingerprint(sub) != src.artifact.Fingerprint {
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
		return s.postGroupComment(ctx, doc, row, v, &row.CommentOnly.Outcome, live, attempt, save)
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
			} else if err = s.postGroupComment(ctx, doc, row, v, &g.Outcomes.Comment, fresh, attempt, save); err != nil {
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
		case !commonDueDate(fresh, g.LateTiming.DueAt):
			// Shared lateness propagates, so it needs one timing that is true for everyone.
			g.Outcomes.Late = domain.ActionOutcome{Status: "not-requested", Message: "members have different effective due dates; late timing was omitted and the grade and feedback continued"}
		default:
			if err = s.postLateTiming(ctx, a, repID, domain.AttemptRef{Attempt: attempt}, g.LateTiming, &g.Outcomes.Late, save); err != nil {
				return err
			}
		}
	}
	return save()
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
func (s *Service) postGroupComment(ctx context.Context, doc *domain.GroupResultsDocument, row *domain.GroupResult, v domain.ValidatedGroup, out *domain.ActionOutcome, live groupLive, attempt int, save func() error) error {
	text, err := GroupSharedText(*doc, *row, v)
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
	if has(live.subs[row.Representative]) {
		*out = domain.ActionOutcome{Status: "preserved", PostedText: text, Message: "exact intended text is already present; ownership not assumed"}
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
		if hold == "" {
			var artifactFound bool
			for _, a := range work.Artifacts {
				artifactFound = artifactFound || a.Key == row.ArtifactKey
			}
			if !artifactFound {
				hold = fmt.Sprintf("artifact %q is absent from the pulled work", row.ArtifactKey)
			} else {
				hold = groupStaticHold(row, work, members)
			}
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
		c.Facts = []ReviewFact{{"Group", fmt.Sprintf("%s (group %d)", names[row.GroupID], row.GroupID)}, {"Affects", strings.Join(memberAliases, ", ")}, {"Artifact", row.ArtifactKey}, {"Grading", "one shared grade for the whole group"}}
		for _, a := range work.Artifacts {
			if a.Key == row.ArtifactKey {
				c.Submission = strings.TrimSpace(a.ReadableText)
				for _, att := range a.Attachments {
					c.Submission += "\n[attachment: " + att.Name + "]"
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
			if g.FeedbackAudience == "none" {
				c.Facts = append(c.Facts, ReviewFact{"Shared comment", "none"})
			}
		}
		if c.Hold == "" {
			text, te := GroupSharedText(doc, row, v)
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
