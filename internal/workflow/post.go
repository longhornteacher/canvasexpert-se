package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
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
	coupledRubric := liveBasis.Rubric != nil && liveBasis.Rubric.UseForGradingKnown && liveBasis.Rubric.UseForGrading
	save := func() error { return workspace.WriteDocument(path, &doc, body) }
	for i := range doc.Students {
		r := &doc.Students[i]
		v := validated[i]
		if resultComplete(*r) || s.Skip[r.StudentRef] {
			report.Students = append(report.Students, postReport(*r))
			continue
		}
		if v.Hold != "" {
			if r.Grade != nil {
				holdGrade(r.Grade, v.Hold)
			} else if r.CommentOnly != nil {
				holdOutcome(&r.CommentOnly.Outcome, v.Hold)
			} else if r.ReviseFeedback != nil {
				holdOutcome(&r.ReviseFeedback.Outcome, v.Hold)
			}
			if err = save(); err != nil {
				return report, err
			}
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
		if r.Operation == "comment-only" && !basisMatches {
			holdOutcome(&r.CommentOnly.Outcome, "assignment/rubric basis changed; pull-work for reassessment")
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
			} else if r.CommentOnly != nil {
				holdOutcome(&r.CommentOnly.Outcome, e.Error())
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
		} else if r.Operation == "comment-only" {
			if !liveAssignment.NewQuiz {
				return report, fmt.Errorf("live assignment is no longer a New Quiz")
			}
			err = s.postCommentOnly(ctx, &doc, r, v, source, live, save)
		} else {
			err = s.postGrade(ctx, &doc, r, v, source, live, coupledRubric, save)
		}
		if err != nil {
			return report, err
		}
		report.Students = append(report.Students, postReport(*r))
	}
	return report, nil
}

func (s *Service) postCommentOnly(ctx context.Context, doc *domain.ResultsDocument, row *domain.StudentResult, v domain.ValidatedStudent, source postSource, live Submission, save func() error) error {
	c := row.CommentOnly
	if live.UserID != source.student.UserID || live.AssignmentID != doc.Assignment.AssignmentID {
		return fmt.Errorf("live submission identity disagrees with requested target")
	}
	if !sameAttempt(c.Attempt, attemptRef(live)) {
		holdOutcome(&c.Outcome, "new submission attempt since pull; pull-work for this student")
		return save()
	}
	computed, err := domain.ComputeAssessment(*doc.Basis(), v.Interpretation, c.Components, doc.Assignment.Points)
	if err != nil {
		return err
	}
	text, err := render.RenderFeedback(render.FeedbackInput{Rubric: *doc.Basis(), Interpretation: v.Interpretation, Components: c.Components, Assessment: computed.Posted, AssignmentPoints: doc.Assignment.Points, Feedback: workspace.ReplaceStudentName(c.Feedback, source.student), OmitAssessment: true, Header: c.Header})
	if err != nil {
		return err
	}
	var matches int
	for _, existing := range commentsForAttempt(live, c.Attempt.Attempt) {
		if existing.Comment == text {
			matches++
		}
	}
	if matches >= 1 {
		c.Outcome = domain.ActionOutcome{Status: "preserved", PostedText: text, Message: "exact intended text is already present; ownership not assumed"}
		return save()
	}
	c.Outcome = domain.ActionOutcome{Status: "pending", PostedText: text}
	if err = save(); err != nil {
		return err
	}
	err = s.Client.Write(ctx, "PUT", submissionPath(*doc, source.student.UserID), map[string]any{"comment": map[string]any{"text_comment": text, "attempt": c.Attempt.Attempt}}, nil)
	if err != nil {
		c.Outcome = failedOutcome(err)
		c.Outcome.PostedText = text
		return save()
	}
	// Canvas accepted the comment. This mode has no grade or rubric write and
	// deliberately does not read the submission back.
	c.Outcome = domain.ActionOutcome{Status: "succeeded", PostedText: text}
	return save()
}

func resultComplete(r domain.StudentResult) bool {
	if r.Operation == "grade" {
		return gradeComplete(r.Grade)
	}
	if r.Operation == "comment-only" {
		return r.CommentOnly != nil && definite(r.CommentOnly.Outcome)
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
	} else if r.CommentOnly != nil {
		p.Revision = r.CommentOnly.Outcome
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
// sameAttempt identifies an attempt by its number alone. Canvas gives every
// resubmission a new attempt number, and the same moment is reported with or
// without milliseconds, so comparing timestamps only creates false "new
// attempt" holds.
func sameAttempt(a, b domain.AttemptRef) bool {
	return a.Attempt == b.Attempt
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
		if resultComplete(r) || s.Skip[r.StudentRef] {
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

		} else if r.CommentOnly != nil {
			if !sameAttempt(r.CommentOnly.Attempt, work.Current.Ref) {
				return nil, fmt.Errorf("%s target attempt differs from pulled work", r.StudentRef)
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

func (s *Service) postGrade(ctx context.Context, doc *domain.ResultsDocument, row *domain.StudentResult, v domain.ValidatedStudent, source postSource, live Submission, coupledRubric bool, save func() error) error {
	g := row.Grade
	if reason := unsupportedSubmission(live); reason != "" {
		holdGrade(g, reason)
		return save()
	}
	if !sameAttempt(g.Attempt, attemptRef(live)) {
		holdGrade(g, "new submission attempt since pull; pull-work for this student")
		return save()
	}
	// Without a Canvas rubric the agent's scoring_basis supplies criteria for the
	// arithmetic and feedback lines; nothing rubric-shaped is written to Canvas.
	rubric := doc.Basis()
	noCanvasRubric := doc.Assignment.Rubric == nil
	if rubric == nil {
		holdGrade(g, "scoring basis is absent; provide common-sense criteria for this work")
		return save()
	}
	if noCanvasRubric {
		if !definite(g.Outcomes.Rubric) {
			g.Outcomes.Rubric = domain.ActionOutcome{Status: "not-requested", Message: "assignment has no Canvas rubric"}
		}
	}
	// Unknown writes are reconciled, never replayed based on an empty lookup.
	if ambiguous(g.Outcomes.Rubric) {
		if rubricMatches(live, *v.Assessment) {
			g.Outcomes.Rubric = domain.ActionOutcome{Status: "preserved", Message: "intended rubric values are present"}
		} else {
			g.Outcomes.Rubric.Status = "uncertain"
			g.Outcomes.Rubric.Message = "rubric write completion remains uncertain; inspect current assessment"
		}
	}
	if ambiguous(g.Outcomes.Grade) {
		if grade := decimalPointer(live.EnteredScore); grade != nil && grade.Cmp(g.Assessment) == 0 {
			g.Outcomes.Grade = domain.ActionOutcome{Status: "preserved", Message: "computed assessment is present on intended attempt"}
		} else {
			g.Outcomes.Grade.Status = "uncertain"
			g.Outcomes.Grade.Message = "grade completion remains uncertain; a different live score is not proof of failure"
		}
	}
	if err := save(); err != nil {
		return err
	}
	// Canvas applies a coupled rubric's raw sum after a simultaneous grade,
	// which can replace the computed assessment when their scales differ.
	// Confirm the honest rubric first, then post the computed grade separately.
	if coupledRubric && !definite(g.Outcomes.Rubric) && !ambiguous(g.Outcomes.Rubric) && !ambiguous(g.Outcomes.Grade) {
		g.Outcomes.Rubric = domain.ActionOutcome{Status: "pending"}
		g.Outcomes.Grade = domain.ActionOutcome{}
		if err := save(); err != nil {
			return err
		}
		var response Submission
		err := s.Client.Write(ctx, "PUT", submissionPath(*doc, source.student.UserID), map[string]any{"rubric_assessment": rubricFields(*v.Assessment)}, &response)
		if err != nil {
			g.Outcomes.Rubric = failedOutcome(err)
		} else {
			g.Outcomes.Rubric = domain.ActionOutcome{Status: "succeeded"}
			live = response
		}
		if err = save(); err != nil {
			return err
		}
	}
	if !noCanvasRubric && !coupledRubric && !ambiguous(g.Outcomes.Rubric) && !ambiguous(g.Outcomes.Grade) && (!definite(g.Outcomes.Rubric) || !definite(g.Outcomes.Grade)) {
		g.Outcomes.Rubric = domain.ActionOutcome{Status: "pending"}
		g.Outcomes.Grade = domain.ActionOutcome{Status: "pending"}
		if err := save(); err != nil {
			return err
		}
		var response Submission
		err := s.Client.Write(ctx, "PUT", submissionPath(*doc, source.student.UserID), map[string]any{"rubric_assessment": rubricFields(*v.Assessment), "submission": map[string]any{"posted_grade": g.Assessment.String()}}, &response)
		if err != nil {
			g.Outcomes.Rubric, g.Outcomes.Grade = failedOutcome(err), failedOutcome(err)
		} else {
			g.Outcomes.Rubric = domain.ActionOutcome{Status: "succeeded"}
			g.Outcomes.Grade = domain.ActionOutcome{Status: "succeeded"}
			live = response
		}
		if err = save(); err != nil {
			return err
		}
	}
	if (noCanvasRubric || (coupledRubric && definite(g.Outcomes.Rubric))) && !definite(g.Outcomes.Grade) && !ambiguous(g.Outcomes.Grade) {
		fresh, e := GetSubmission(ctx, s.Client, doc.Assignment.CourseID, doc.Assignment.AssignmentID, source.student.UserID)
		if e != nil {
			g.Outcomes.Grade = domain.ActionOutcome{Status: "held", Message: e.Error()}
		} else if !sameAttempt(g.Attempt, attemptRef(fresh)) {
			holdGrade(g, "student resubmitted between actions")
			return save()
		} else {
			g.Outcomes.Grade = domain.ActionOutcome{Status: "pending"}
			if err := save(); err != nil {
				return err
			}
			var response Submission
			e = s.Client.Write(ctx, "PUT", submissionPath(*doc, source.student.UserID), map[string]any{"submission": map[string]any{"posted_grade": g.Assessment.String()}}, &response)
			if e != nil {
				g.Outcomes.Grade = failedOutcome(e)
			} else {
				g.Outcomes.Grade = domain.ActionOutcome{Status: "succeeded"}
				live = response
			}
		}
		if e = save(); e != nil {
			return e
		}
	}
	if !definite(g.Outcomes.Comment) {
		text, e := render.RenderFeedback(render.FeedbackInput{Rubric: *rubric, Interpretation: v.Interpretation, Components: g.Components, Assessment: g.Assessment, AssignmentPoints: doc.Assignment.Points, Feedback: workspace.ReplaceStudentName(g.Feedback, source.student), Late: isLate(g.LateTiming), Header: g.Header})
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
	if g.LateTiming == nil {
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
				out.Status = "preserved"
				out.Message = "known comment remains present"
				if c.Comment != out.PostedText {
					out.Message = "known comment was edited in Canvas; the teacher's version is kept"
				}
				return nil
			}
		}
		out.Status = "preserved"
		out.Message = "known comment was deleted in Canvas; it is not posted again"
		return nil
	}
	matches := []SubmissionComment{}
	for _, c := range commentsForAttempt(live, attempt.Attempt) {
		if c.Comment == text {
			matches = append(matches, c)
		}
	}
	if len(matches) >= 1 {
		*out = domain.ActionOutcome{Status: "preserved", PostedText: text, Message: "exact intended text is already present; ownership not assumed"}
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
	// Canvas accepted the write. Its reply does not reliably echo the new
	// comment, and the teacher sees the result in Canvas, so do not second-guess it.
	id := int64(0)
	for _, c := range response.Comments {
		if !before[c.ID] && c.ID > id && c.Comment == text {
			id = c.ID
		}
	}
	*out = domain.ActionOutcome{Status: "succeeded", ID: id, PostedText: text}
	return nil
}

func (s *Service) postRevision(ctx context.Context, doc *domain.ResultsDocument, row *domain.StudentResult, v domain.ValidatedStudent, source postSource, live Submission, save func() error) error {
	r := row.ReviseFeedback
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
		text, e = render.RenderFeedback(render.FeedbackInput{Rubric: *doc.Basis(), Interpretation: v.Interpretation, Components: r.Assessment.Components, Assessment: r.Assessment.Assessment, AssignmentPoints: doc.Assignment.Points, Feedback: text, OmitAssessment: doc.Assignment.NewQuiz, Header: r.Header})
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

// isLate reports whether the agent's late timing marks the attempt late.
func isLate(t *domain.LateTiming) bool { return t != nil && t.Status == "late" }
