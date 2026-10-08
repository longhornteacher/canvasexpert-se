package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"canvassidekick/internal/domain"
	"canvassidekick/internal/render"
	"canvassidekick/internal/workspace"
)

type StudentPostReport struct {
	StudentRef, Operation, Status, Message string
	Outcomes                               domain.ActionOutcomes
	Revision                               domain.ActionOutcome
}
type PostReport struct {
	Path     string
	Students []StudentPostReport
	Requests int64
	Elapsed  time.Duration
}
type postSource struct {
	student workspace.Student
	work    domain.StudentWorkDocument
}

func (s *Service) Post(ctx context.Context, path string) (report PostReport, err error) {
	start := time.Now()
	requests := s.Client.Requests()
	report.Path = path
	defer func() { report.Elapsed = time.Since(start); report.Requests = s.Client.Requests() - requests }()
	var doc domain.ResultsDocument
	body, err := workspace.ReadDocument(path, &doc)
	if err != nil {
		return report, err
	}
	validated, err := domain.ValidateResults(&doc)
	if err != nil {
		return report, err
	}
	// Even a local persistence error after remote success returns the known
	// in-memory action outcomes; it must not imply nothing reached Canvas.
	defer func() {
		report.Students = nil
		for _, row := range doc.Students {
			report.Students = append(report.Students, postReport(row))
		}
	}()
	if doc.Assignment.Instance != s.Client.Origin() {
		return report, fmt.Errorf("results target a different Canvas instance")
	}
	configured := false
	for _, c := range s.Config.Courses {
		if c.ID == doc.Assignment.CourseID {
			configured = true
		}
	}
	if !configured {
		return report, fmt.Errorf("results course is not configured")
	}
	allComplete := true
	for _, r := range doc.Students {
		if !resultComplete(r) {
			allComplete = false
		}
	}
	if allComplete {
		for _, r := range doc.Students {
			report.Students = append(report.Students, postReport(r))
		}
		return report, nil
	}
	// Resolve every referenced identity and packet assertion before the first
	// Canvas write. A malformed later row cannot cause partial earlier writes.
	sources, err := s.postSources(doc)
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
	basisMatches := SameGradingBasis(doc.Assignment, liveBasis)
	save := func() error { return workspace.WriteDocument(path, &doc, body) }
	for i := range doc.Students {
		r := &doc.Students[i]
		v := validated[i]
		if resultComplete(*r) {
			report.Students = append(report.Students, postReport(*r))
			continue
		}
		if r.Operation == "grade" && !basisMatches {
			holdGrade(r.Grade, "assignment/rubric grading basis changed; pull-work for reassessment")
			if err = save(); err != nil {
				return report, err
			}
			report.Students = append(report.Students, postReport(*r))
			continue
		}
		source := sources[r.StudentRef]
		live, e := GetSubmission(ctx, s.Client, doc.Assignment.CourseID, doc.Assignment.AssignmentID, source.student.UserID)
		if e != nil {
			if r.Grade != nil {
				holdGrade(r.Grade, e.Error())
			} else {
				holdOutcome(&r.ReviseFeedback.Outcome, e.Error())
			}
			if err = save(); err != nil {
				return report, err
			}
			report.Students = append(report.Students, postReport(*r))
			continue
		}
		if live.UserID != source.student.UserID || live.AssignmentID != doc.Assignment.AssignmentID {
			return report, fmt.Errorf("live submission identity disagrees with requested target")
		}
		if r.Operation == "revise-feedback" {
			err = s.postRevision(ctx, &doc, r, v, source, live, save)
		} else {
			err = s.postGrade(ctx, &doc, r, v, source, live, save)
		}
		if err != nil {
			return report, err
		}
		report.Students = append(report.Students, postReport(*r))
	}
	return report, nil
}

func resultComplete(r domain.StudentResult) bool {
	if r.Operation == "grade" {
		return gradeComplete(r.Grade)
	}
	return r.ReviseFeedback != nil && definite(r.ReviseFeedback.Outcome)
}
func postReport(r domain.StudentResult) StudentPostReport {
	p := StudentPostReport{StudentRef: r.StudentRef, Operation: r.Operation, Status: "partial"}
	if r.Grade != nil {
		p.Outcomes = r.Grade.Outcomes
		for _, o := range []domain.ActionOutcome{p.Outcomes.Rubric, p.Outcomes.Grade, p.Outcomes.Comment, p.Outcomes.Late} {
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
	} else if r.ReviseFeedback != nil {
		p.Revision = r.ReviseFeedback.Outcome
		p.Status = p.Revision.Status
		p.Message = p.Revision.Message
	}
	if resultComplete(r) {
		p.Status = "completed"
	}
	return p
}
func holdGrade(g *domain.GradeOperation, message string) {
	for _, o := range []*domain.ActionOutcome{&g.Outcomes.Rubric, &g.Outcomes.Grade, &g.Outcomes.Comment, &g.Outcomes.Late} {
		if !definite(*o) {
			if !ambiguous(*o) {
				o.Status = "held"
			}
			o.Message = message
		}
	}
}
func holdOutcome(o *domain.ActionOutcome, message string) {
	if !ambiguous(*o) {
		o.Status = "held"
	}
	o.Message = message
}
func sameAttempt(a, b domain.AttemptRef) bool {
	ta, e1 := time.Parse(time.RFC3339Nano, a.SubmittedAt)
	tb, e2 := time.Parse(time.RFC3339Nano, b.SubmittedAt)
	return e1 == nil && e2 == nil && a.Attempt == b.Attempt && ta.Equal(tb)
}
func (s *Service) postSources(doc domain.ResultsDocument) (map[string]postSource, error) {
	packetPath, e := workspace.ConstrainPath(s.Root, doc.Packet)
	if e != nil {
		return nil, e
	}
	var packet domain.PacketDocument
	if _, e = workspace.ReadDocument(packetPath, &packet); e != nil {
		return nil, e
	}
	if !SameGradingBasis(packet.Assignment, doc.Assignment) {
		return nil, fmt.Errorf("results grading basis differs from source packet")
	}
	store, e := workspace.OpenStudents(s.Root)
	if e != nil {
		return nil, e
	}
	workByRef := map[string]domain.StudentWorkDocument{}
	for _, rel := range packet.Students {
		path, e := workspace.ConstrainPath(s.Root, rel)
		if e != nil {
			return nil, e
		}
		var work domain.StudentWorkDocument
		if _, e = workspace.ReadDocument(path, &work); e != nil {
			return nil, e
		}
		if work.AssignmentID != doc.Assignment.AssignmentID {
			return nil, fmt.Errorf("student-work assignment differs from packet")
		}
		if _, exists := workByRef[work.StudentRef]; exists {
			return nil, fmt.Errorf("packet duplicates student_ref %s", work.StudentRef)
		}
		workByRef[work.StudentRef] = work
	}
	sources := map[string]postSource{}
	for _, r := range doc.Students {
		if resultComplete(r) {
			continue
		}
		st, e := store.Resolve(s.Client.Origin(), r.StudentRef)
		if e != nil {
			return nil, e
		}
		work, ok := workByRef[r.StudentRef]
		if !ok {
			return nil, fmt.Errorf("student %s is absent from source packet", r.StudentRef)
		}
		if r.Grade != nil {
			g := r.Grade
			if !sameAttempt(g.Attempt, work.Current.Ref) {
				return nil, fmt.Errorf("%s target attempt differs from pulled work", r.StudentRef)
			}
			if g.StrongestPrior != nil && !hasAttempt(work.Prior, g.StrongestPrior.Attempt) {
				return nil, fmt.Errorf("%s strongest prior is absent from actual pulled history", r.StudentRef)
			}
			if late := g.LateForgiveness; late != nil {
				found := false
				for _, prior := range work.Prior {
					if sameAttempt(prior.Ref, late.FirstAttempt) && prior.EffectiveDueAt == late.EffectiveDueAt {
						found = true
					}
				}
				if !found {
					return nil, fmt.Errorf("%s late evidence is absent or differs from pulled effective due date", r.StudentRef)
				}
			}
		} else {
			rv := r.ReviseFeedback
			if !sameAttempt(rv.Attempt, work.Current.Ref) && !hasAttempt(work.Prior, rv.Attempt) {
				return nil, fmt.Errorf("%s revision target attempt is absent from packet", r.StudentRef)
			}
			found := false
			for _, c := range work.Comments {
				if c.ID == rv.CommentID {
					found = true
					if c.Attempt != nil && *c.Attempt != rv.Attempt.Attempt {
						return nil, fmt.Errorf("revision comment belongs to another attempt")
					}
				}
			}
			if !found {
				return nil, fmt.Errorf("revision comment ID is absent from pulled comments")
			}
		}
		sources[r.StudentRef] = postSource{student: st, work: work}
	}
	return sources, nil
}
func hasAttempt(prior []domain.AttemptWork, a domain.AttemptRef) bool {
	for _, p := range prior {
		if sameAttempt(p.Ref, a) {
			return true
		}
	}
	return false
}
func submissionPath(d domain.ResultsDocument, userID int64) string {
	return fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions/%d", d.Assignment.CourseID, d.Assignment.AssignmentID, userID)
}
func rubricFields(a domain.Assessment) map[string]any {
	m := map[string]any{}
	for _, c := range a.Criteria {
		v := map[string]any{"points": domain.RoundRat(c.Points, 8)}
		if c.RatingID != "" {
			v["rating_id"] = c.RatingID
		}
		if c.Comment != "" {
			v["comments"] = c.Comment
		}
		m[c.CriterionID] = v
	}
	return m
}
func rubricMatches(sub Submission, a domain.Assessment) bool {
	if len(sub.RubricAssessment) != len(a.Criteria) {
		return false
	}
	for _, c := range a.Criteria {
		r, ok := sub.RubricAssessment[c.CriterionID]
		p := decimalPointer(r.Points)
		if !ok || p == nil || p.Cmp(domain.RoundRat(c.Points, 8)) != 0 || r.RatingID != c.RatingID || r.Comments != c.Comment {
			return false
		}
	}
	return true
}
func ambiguous(o domain.ActionOutcome) bool { return o.Status == "pending" || o.Status == "uncertain" }

func (s *Service) postGrade(ctx context.Context, doc *domain.ResultsDocument, row *domain.StudentResult, v domain.ValidatedStudent, source postSource, live Submission, save func() error) error {
	g := row.Grade
	if reason := unsupportedSubmission(live); reason != "" {
		holdGrade(g, reason)
		return save()
	}
	if !sameAttempt(g.Attempt, attemptRef(live)) {
		holdGrade(g, "new submission attempt since pull; pull-work for this student")
		return save()
	}
	if !reflect.DeepEqual(source.work.Current.BodyHTML, live.Body) || !sameAttachmentIDs(source.work.Current.Attachments, live.Attachments) {
		holdGrade(g, "submission content changed since pull; pull-work for this student")
		return save()
	}
	known, entered, reason := s.enteredGrade(live)
	if !known {
		holdGrade(g, reason)
		return save()
	}
	rubric := doc.Assignment.Rubric
	if rubric == nil || !rubric.UseForGradingKnown {
		holdGrade(g, "rubric automatic-grade setting cannot be determined")
		return save()
	}
	if e := s.require(s.Evidence.FractionalRubricPoints, "honest rubric criterion points"); e != nil {
		holdGrade(g, e.Error())
		return save()
	}
	if e := s.require(s.Evidence.CommentAttempts, "feedback attempt targeting and returned comment identity"); e != nil {
		holdGrade(g, e.Error())
		return save()
	}
	if rubric.UseForGrading {
		if e := s.require(s.Evidence.CoupledRubricPreservation, "combined coupled-rubric/entered-grade preservation"); e != nil {
			holdGrade(g, e.Error()+"; existing association requires a verified path or explicit decoupling choice")
			return save()
		}
	} else {
		if e := s.require(s.Evidence.IndependentRubricAssessment, "independent rubric assessment"); e != nil {
			holdGrade(g, e.Error())
			return save()
		}
	}
	decision := domain.DecideGrade(g.Assessment, entered)
	// Unknown writes are reconciled, never replayed based on an empty lookup.
	if ambiguous(g.Outcomes.Rubric) {
		if rubricMatches(live, *v.Assessment) {
			g.Outcomes.Rubric = domain.ActionOutcome{Status: "preserved", Message: "intended rubric values are present"}
		} else {
			g.Outcomes.Rubric.Status = "uncertain"
			g.Outcomes.Rubric.Message = "rubric write completion remains uncertain; inspect current assessment"
		}
	}
	gradeSatisfied := !decision.Write
	if ambiguous(g.Outcomes.Grade) {
		gradeSatisfied = g.DesiredEnteredGrade != nil && entered != nil && entered.Cmp(*g.DesiredEnteredGrade) >= 0
	}
	if !definite(g.Outcomes.Grade) && gradeSatisfied {
		g.Outcomes.Grade = domain.ActionOutcome{Status: "preserved", Message: "live entered grade is at least the assessment"}
	}
	if ambiguous(g.Outcomes.Grade) && !gradeSatisfied {
		g.Outcomes.Grade.Status = "uncertain"
		g.Outcomes.Grade.Message = "grade write completion remains uncertain; lower live value is not proof of failure"
	}
	if err := save(); err != nil {
		return err
	}
	if rubric.UseForGrading && ambiguous(g.Outcomes.Grade) {
		holdOutcome(&g.Outcomes.Comment, "coupled entered-grade preservation remains uncertain; feedback held")
		holdOutcome(&g.Outcomes.Late, "coupled entered-grade preservation remains uncertain; late adjustment held")
		return save()
	}
	if rubric.UseForGrading {
		if !definite(g.Outcomes.Rubric) && !ambiguous(g.Outcomes.Rubric) {
			if definite(g.Outcomes.Grade) && decision.Write {
				g.Outcomes.Rubric = domain.ActionOutcome{Status: "held", Message: "entered grade changed after completed grade action; coupled rubric would restore an old grade"}
			} else {
				// This coupled write necessarily re-sends the preserved current
				// grade. Its returned grade must be checked even when the initial
				// read already established retention; never trust field ordering.
				needGrade := true
				g.Outcomes.Rubric = domain.ActionOutcome{Status: "pending"}
				desired := decision.Desired
				g.DesiredEnteredGrade = &desired
				if needGrade {
					g.Outcomes.Grade = domain.ActionOutcome{Status: "pending"}
				}
				if err := save(); err != nil {
					return err
				}
				var response Submission
				err := s.Client.Write(ctx, "PUT", submissionPath(*doc, source.student.UserID), map[string]any{"rubric_assessment": rubricFields(*v.Assessment), "submission": map[string]any{"posted_grade": decision.Desired.String()}}, &response)
				if err != nil {
					g.Outcomes.Rubric = failedOutcome(err)
					if needGrade {
						g.Outcomes.Grade = failedOutcome(err)
					}
				} else {
					g.Outcomes.Rubric = confirmedRubric(response, *v.Assessment, g.Attempt)
					if needGrade {
						g.Outcomes.Grade = s.confirmedGrade(response, decision.Desired, g.Attempt)
					}
				}
				if err = save(); err != nil {
					return err
				}
				if !definite(g.Outcomes.Grade) {
					holdOutcome(&g.Outcomes.Comment, "combined rubric update did not confirm grade preservation; feedback held")
					holdOutcome(&g.Outcomes.Late, "combined rubric update did not confirm grade preservation; late action held")
					return save()
				}
			}
		}
	} else if !definite(g.Outcomes.Rubric) && !ambiguous(g.Outcomes.Rubric) {
		g.Outcomes.Rubric = domain.ActionOutcome{Status: "pending"}
		if err := save(); err != nil {
			return err
		}
		var response Submission
		err := s.Client.Write(ctx, "PUT", submissionPath(*doc, source.student.UserID), map[string]any{"rubric_assessment": rubricFields(*v.Assessment)}, &response)
		if err != nil {
			g.Outcomes.Rubric = failedOutcome(err)
		} else {
			g.Outcomes.Rubric = confirmedRubric(response, *v.Assessment, g.Attempt)
			live = response
		}
		if err = save(); err != nil {
			return err
		}
	}
	if !definite(g.Outcomes.Grade) && !ambiguous(g.Outcomes.Grade) && decision.Write {
		// Recheck entered value/attempt after rubric write, immediately before the
		// separate independent grade action. Concurrent teacher edits can still
		// race this read/write interval; Canvas offers no proven conditional write.
		fresh, e := GetSubmission(ctx, s.Client, doc.Assignment.CourseID, doc.Assignment.AssignmentID, source.student.UserID)
		if e != nil {
			g.Outcomes.Grade = domain.ActionOutcome{Status: "held", Message: e.Error()}
		} else if !sameAttempt(g.Attempt, attemptRef(fresh)) {
			holdGrade(g, "student resubmitted between actions")
			return save()
		} else if ok, current, why := s.enteredGrade(fresh); !ok {
			g.Outcomes.Grade = domain.ActionOutcome{Status: "held", Message: why}
		} else {
			decision = domain.DecideGrade(g.Assessment, current)
			entered = current
			if !decision.Write {
				g.Outcomes.Grade = domain.ActionOutcome{Status: "preserved"}
			} else {
				g.Outcomes.Grade = domain.ActionOutcome{Status: "pending"}
				desired := decision.Desired
				g.DesiredEnteredGrade = &desired
				if err := save(); err != nil {
					return err
				}
				var response Submission
				e = s.Client.Write(ctx, "PUT", submissionPath(*doc, source.student.UserID), map[string]any{"submission": map[string]any{"posted_grade": decision.Desired.String()}}, &response)
				if e != nil {
					g.Outcomes.Grade = failedOutcome(e)
				} else {
					g.Outcomes.Grade = s.confirmedGrade(response, decision.Desired, g.Attempt)
					live = response
				}
			}
		}
		if e = save(); e != nil {
			return e
		}
	}
	if !definite(g.Outcomes.Comment) {
		retained := (*domain.Decimal)(nil)
		if entered != nil && entered.Cmp(g.Assessment) > 0 {
			retained = entered
		}
		text, e := render.RenderFeedback(render.FeedbackInput{Rubric: *rubric, Interpretation: v.Interpretation, Components: g.Components, Assessment: g.Assessment, AssignmentPoints: doc.Assignment.Points, RetainedGrade: retained, Feedback: workspace.ReplaceStudentName(g.Feedback, source.student)})
		if e != nil {
			return e
		}
		// Once an add has begun, retain its exact text across retries even if a
		// teacher subsequently changes the entered grade or the file's feedback.
		if g.Outcomes.Comment.PostedText != "" && (ambiguous(g.Outcomes.Comment) || g.Outcomes.Comment.ID > 0) {
			text = g.Outcomes.Comment.PostedText
		}
		fresh, e := GetSubmission(ctx, s.Client, doc.Assignment.CourseID, doc.Assignment.AssignmentID, source.student.UserID)
		if e != nil {
			holdOutcome(&g.Outcomes.Comment, e.Error())
			g.Outcomes.Comment.PostedText = text
		} else if !sameAttempt(g.Attempt, attemptRef(fresh)) {
			holdGrade(g, "student resubmitted before feedback")
			return save()
		} else {
			if !ambiguous(g.Outcomes.Comment) && g.Outcomes.Comment.ID == 0 {
				known, current, why := s.enteredGrade(fresh)
				if !known {
					holdOutcome(&g.Outcomes.Comment, why)
					return save()
				}
				retained = nil
				if current != nil && current.Cmp(g.Assessment) > 0 {
					retained = current
				}
				text, e = render.RenderFeedback(render.FeedbackInput{Rubric: *rubric, Interpretation: v.Interpretation, Components: g.Components, Assessment: g.Assessment, AssignmentPoints: doc.Assignment.Points, RetainedGrade: retained, Feedback: workspace.ReplaceStudentName(g.Feedback, source.student)})
				if e != nil {
					return e
				}
			}
			e = s.addFeedback(ctx, *doc, source.student.UserID, g.Attempt, text, &g.Outcomes.Comment, fresh, save)
			if e != nil {
				return e
			}
			live = fresh
		}
		if e = save(); e != nil {
			return e
		}
	}
	if g.LateForgiveness == nil {
		if !definite(g.Outcomes.Late) {
			g.Outcomes.Late = domain.ActionOutcome{Status: "not-requested"}
		}
	} else if !definite(g.Outcomes.Late) {
		if err := s.postLate(ctx, *doc, source.student.UserID, g, save); err != nil {
			return err
		}
	}
	return save()
}
func sameAttachmentIDs(a []domain.Attachment, b []Attachment) bool {
	if len(a) != len(b) {
		return false
	}
	ids := map[int64]bool{}
	for _, v := range a {
		ids[v.ID] = true
	}
	for _, v := range b {
		if !ids[v.ID] {
			return false
		}
	}
	return true
}
func confirmedRubric(s Submission, a domain.Assessment, attempt domain.AttemptRef) domain.ActionOutcome {
	if sameAttempt(attempt, attemptRef(s)) && rubricMatches(s, a) {
		return domain.ActionOutcome{Status: "succeeded"}
	}
	return domain.ActionOutcome{Status: "uncertain", Message: "returned submission did not confirm intended attempt and rubric values"}
}
func (s *Service) confirmedGrade(sub Submission, desired domain.Decimal, attempt domain.AttemptRef) domain.ActionOutcome {
	known, grade, _ := s.enteredGrade(sub)
	if sameAttempt(attempt, attemptRef(sub)) && known && grade != nil && grade.Cmp(desired) == 0 {
		return domain.ActionOutcome{Status: "succeeded"}
	}
	return domain.ActionOutcome{Status: "uncertain", Message: "returned submission did not confirm desired pre-penalty grade on intended attempt"}
}

// submissionComments includes comments returned only with prior attempts. A
// history context supplies an association only when all observed contexts agree;
// repeated unscoped comments must not be assigned arbitrarily to one attempt.
func submissionComments(s Submission) []SubmissionComment {
	byID := map[int64]SubmissionComment{}
	associations := map[int64]map[int]bool{}
	add := func(c SubmissionComment, context int) {
		if c.ID <= 0 {
			return
		}
		if _, exists := byID[c.ID]; !exists {
			byID[c.ID] = c
		}
		attempt := context
		if c.Attempt != nil {
			attempt = *c.Attempt
		}
		if attempt > 0 {
			if associations[c.ID] == nil {
				associations[c.ID] = map[int]bool{}
			}
			associations[c.ID][attempt] = true
		}
	}
	for _, c := range s.Comments {
		add(c, 0)
	}
	for _, h := range s.History {
		for _, c := range h.Comments {
			add(c, h.Attempt)
		}
	}
	var out []SubmissionComment
	for _, c := range byID {
		c.Attempt = nil
		if len(associations[c.ID]) == 1 {
			for attempt := range associations[c.ID] {
				a := attempt
				c.Attempt = &a
			}
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func commentsForAttempt(s Submission, attempt int) []SubmissionComment {
	var out []SubmissionComment
	for _, c := range submissionComments(s) {
		if c.Attempt != nil && *c.Attempt == attempt {
			out = append(out, c)
		}
	}
	return out
}
func (s *Service) addFeedback(ctx context.Context, doc domain.ResultsDocument, userID int64, attempt domain.AttemptRef, text string, out *domain.ActionOutcome, live Submission, save func() error) error {
	if out.ID > 0 {
		for _, c := range submissionComments(live) {
			if c.ID == out.ID {
				if c.Attempt == nil || *c.Attempt != attempt.Attempt {
					out.Status = "held"
					out.Message = "known comment attempt association is unavailable or changed"
				} else if c.Comment == out.PostedText {
					out.Status = "preserved"
					out.Message = "known comment remains present"
				} else {
					out.Status = "held"
					out.Message = "known comment was teacher-edited; old feedback will not be restored"
				}
				return nil
			}
		}
		out.Status = "held"
		out.Message = "known comment was deleted; old feedback will not be recreated"
		return nil
	}
	matches := []SubmissionComment{}
	for _, c := range commentsForAttempt(live, attempt.Attempt) {
		if c.Comment == text {
			matches = append(matches, c)
		}
	}
	if len(matches) == 1 {
		*out = domain.ActionOutcome{Status: "preserved", PostedText: text, Message: "exact intended text is already present; ownership not assumed"}
		return nil
	}
	if len(matches) > 1 {
		holdOutcome(out, "multiple exact feedback matches; select the precise comment")
		out.PostedText = text
		return nil
	}
	if ambiguous(*out) {
		out.Status = "uncertain"
		out.PostedText = text
		out.Message = "comment completion uncertain; no unambiguous attempt-associated exact match"
		return nil
	}
	before := map[int64]bool{}
	for _, c := range submissionComments(live) {
		before[c.ID] = true
	}
	*out = domain.ActionOutcome{Status: "pending", PostedText: text}
	if err := save(); err != nil {
		return err
	}
	var response Submission
	err := s.Client.Write(ctx, "PUT", submissionPath(doc, userID), map[string]any{"comment": map[string]any{"text_comment": text, "attempt": attempt.Attempt}}, &response)
	if err != nil {
		*out = failedOutcome(err)
		out.PostedText = text
		return nil
	}
	added := []SubmissionComment{}
	for _, c := range response.Comments {
		if !before[c.ID] && c.ID > 0 && c.Comment == text && (c.Attempt == nil || *c.Attempt == attempt.Attempt) {
			added = append(added, c)
		}
	}
	if len(added) == 1 && sameAttempt(attempt, attemptRef(response)) {
		*out = domain.ActionOutcome{Status: "succeeded", ID: added[0].ID, PostedText: text}
		return nil
	}
	*out = domain.ActionOutcome{Status: "uncertain", PostedText: text, Message: "comment response did not identify one new exact comment on intended attempt"}
	return nil
}

func (s *Service) postRevision(ctx context.Context, doc *domain.ResultsDocument, row *domain.StudentResult, v domain.ValidatedStudent, source postSource, live Submission, save func() error) error {
	r := row.ReviseFeedback
	if e := s.require(s.Evidence.CommentEditing, "selected comment editing"); e != nil {
		holdOutcome(&r.Outcome, e.Error())
		return save()
	}
	var original *domain.ExistingComment
	for i := range source.work.Comments {
		if source.work.Comments[i].ID == r.CommentID {
			original = &source.work.Comments[i]
		}
	}
	var current *SubmissionComment
	comments := submissionComments(live)
	for i := range comments {
		if comments[i].ID == r.CommentID {
			current = &comments[i]
		}
	}
	if current == nil {
		holdOutcome(&r.Outcome, "selected comment was deleted; no recreation")
		r.Outcome.ID = r.CommentID
		return save()
	}
	if current.Attempt == nil || *current.Attempt != r.Attempt.Attempt {
		holdOutcome(&r.Outcome, "selected comment attempt association is unavailable or differs from target")
		return save()
	}
	text := workspace.ReplaceStudentName(r.Feedback, source.student)
	if r.Assessment != nil {
		var e error
		text, e = render.RenderFeedback(render.FeedbackInput{Rubric: *doc.Assignment.Rubric, Interpretation: v.Interpretation, Components: r.Assessment.Components, Assessment: r.Assessment.Assessment, AssignmentPoints: doc.Assignment.Points, RetainedGrade: r.Assessment.RetainedGrade, Feedback: text})
		if e != nil {
			return e
		}
	}
	if ambiguous(r.Outcome) {
		if current.Comment == r.Outcome.PostedText {
			r.Outcome.Status = "preserved"
			r.Outcome.Message = "requested replacement already present"
			return save()
		}
		r.Outcome.Status = "uncertain"
		r.Outcome.Message = "previous edit completion uncertain; do not restore over live text"
		return save()
	}
	if original == nil || current.Comment != original.Text {
		r.Outcome = domain.ActionOutcome{Status: "held", ID: r.CommentID, Message: "selected comment changed since pull; pull the current comment before revising"}
		return save()
	}
	if current.Comment == text {
		r.Outcome = domain.ActionOutcome{Status: "preserved", ID: r.CommentID, PostedText: text}
		return save()
	}
	r.Outcome = domain.ActionOutcome{Status: "pending", ID: r.CommentID, PostedText: text}
	if e := save(); e != nil {
		return e
	}
	var response SubmissionComment
	e := s.Client.Write(ctx, "PUT", submissionPath(*doc, source.student.UserID)+fmt.Sprintf("/comments/%d", r.CommentID), map[string]any{"comment": text}, &response)
	if e != nil {
		r.Outcome = failedOutcome(e)
		r.Outcome.ID = r.CommentID
		r.Outcome.PostedText = text
	} else if response.ID == r.CommentID && response.Comment == text {
		r.Outcome = domain.ActionOutcome{Status: "succeeded", ID: r.CommentID, PostedText: text}
	} else {
		r.Outcome = domain.ActionOutcome{Status: "uncertain", ID: r.CommentID, PostedText: text, Message: "comment edit response did not confirm selected ID/text"}
	}
	return save()
}

func (s *Service) postLate(ctx context.Context, doc domain.ResultsDocument, userID int64, g *domain.GradeOperation, save func() error) error {
	if e := s.require(s.Evidence.LateOverride && s.Evidence.EffectiveDueDate, "late forgiveness with applicable due-date evidence"); e != nil {
		holdOutcome(&g.Outcomes.Late, e.Error())
		return save()
	}
	live, e := GetSubmission(ctx, s.Client, doc.Assignment.CourseID, doc.Assignment.AssignmentID, userID)
	if e != nil {
		holdOutcome(&g.Outcomes.Late, e.Error())
		return save()
	}
	if !sameAttempt(g.Attempt, attemptRef(live)) {
		holdOutcome(&g.Outcomes.Late, "new attempt before late adjustment")
		return save()
	}
	l := g.LateForgiveness
	found := false
	for _, h := range live.History {
		if sameAttempt(l.FirstAttempt, attemptRef(h)) && h.CachedDueDate != nil && *h.CachedDueDate == l.EffectiveDueAt {
			found = true
		}
	}
	if !found {
		holdOutcome(&g.Outcomes.Late, "live first-attempt/applicable due-date evidence unavailable or changed")
		return save()
	}
	if live.LatePolicyStatus != nil && *live.LatePolicyStatus == "none" {
		g.Outcomes.Late = domain.ActionOutcome{Status: "preserved"}
		return save()
	}
	if ambiguous(g.Outcomes.Late) {
		g.Outcomes.Late.Status = "uncertain"
		g.Outcomes.Late.Message = "late override completion uncertain; current state does not prove failure"
		return save()
	}
	g.Outcomes.Late = domain.ActionOutcome{Status: "pending"}
	if e = save(); e != nil {
		return e
	}
	var response Submission
	e = s.Client.Write(ctx, "PUT", submissionPath(doc, userID), map[string]any{"submission": map[string]any{"late_policy_status": "none"}}, &response)
	if e != nil {
		g.Outcomes.Late = failedOutcome(e)
	} else if sameAttempt(g.Attempt, attemptRef(response)) && response.LatePolicyStatus != nil && *response.LatePolicyStatus == "none" {
		g.Outcomes.Late = domain.ActionOutcome{Status: "succeeded"}
	} else {
		g.Outcomes.Late = domain.ActionOutcome{Status: "uncertain", Message: "response did not confirm late override on intended attempt"}
	}
	return save()
}

// ResultPaths assists callers without requiring stdout parsing.
func ResultPaths(root string, packet domain.PacketDocument) ([]string, error) {
	var paths []string
	for _, p := range packet.Students {
		path, e := workspace.ConstrainPath(root, filepath.FromSlash(strings.TrimSpace(p)))
		if e != nil {
			return nil, e
		}
		paths = append(paths, path)
	}
	return paths, nil
}
