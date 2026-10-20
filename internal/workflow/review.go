package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
)

// ReviewModel is the teacher-facing rendering of a results file: exactly what
// post would write, plus holds and teacher-only notes (marked as such).
type ReviewModel struct {
	Kind, Title string
	Notice      string // fidelity limits shown at the top of the page
	// AllOrNothing is for content that is created whole (assignment, quiz): Post
	// needs every card approved, and any revise sends the whole thing back.
	AllOrNothing bool
	Action       string // label of the all-or-nothing button; default "Create in Canvas"
	// PostLabel replaces "Post approved"; Actions add buttons that run the same
	// poster with their Value; Test adds a "Send a test to me" button.
	PostLabel string
	Actions   []ReviewAction
	Test      bool
	// Email switches the page wording from writing to Canvas to sending email.
	Email bool
	// Frames are shared message previews above the per-student recipient cards.
	Frames []ReviewFrame
	// ContentDigest binds nonvisual bytes such as email file attachments.
	ContentDigest string
	Cards         []ReviewCard
}

// ReviewAction is one extra page button that approves-and-runs with its Value.
type ReviewAction struct{ Value, Label string }

// ReviewFact is one labelled setting shown on a card.
type ReviewFact struct{ Label, Value string }

// ReviewFrame is an HTML field shown the way Canvas keeps it: sandboxed, no scripts, no page styling.
type ReviewFrame struct {
	Label, HTML string
	Files       []string // local attachment names shown privately, without paths
}

// ReviewCard is one student. Digest changes whenever anything shown changes.
type ReviewCard struct {
	Ref, Alias, Operation string
	Attempt               int
	Total                 string // computed assessment, e.g. "18.5 / 20"
	Comment               string // exact plain text that will be posted (ordinary results)
	Submission            string // the student's work for this attempt, so feedback is judged against it
	Lines                 []ReviewLine
	Facts                 []ReviewFact
	Frames                []ReviewFrame
	Key                   string // teacher-only answer key, shown behind a toggle
	TeacherNote           string
	Late                  string
	LateBasis             string // agent's private lateness reasoning, teacher only
	Hold                  string // reason this student will not post
	Done                  bool   // already posted
	Digest                string
}

// ReviewLine is one line within a review card.
type ReviewLine struct {
	Name, Score, Comment, TeacherNote, Hold string
	Done                                    bool
}

// ReviewItem is the teacher's decision on one card.
type ReviewItem struct {
	Status string `json:"status"` // approved | revise
	Note   string `json:"note,omitempty"`
	Digest string `json:"digest"`
}

// ReviewState lives beside the results file and never reaches Canvas.
type ReviewState struct {
	Items map[string]ReviewItem `json:"items"`
}

func reviewStatePath(results string) string { return results + ".review.json" }

// LoadReviewState returns an empty state when none has been saved.
func LoadReviewState(results string) (ReviewState, error) {
	st := ReviewState{Items: map[string]ReviewItem{}}
	b, e := os.ReadFile(reviewStatePath(results))
	if os.IsNotExist(e) {
		return st, nil
	}
	if e != nil {
		return st, e
	}
	if e = json.Unmarshal(b, &st); e != nil {
		return ReviewState{Items: map[string]ReviewItem{}}, fmt.Errorf("review state file is unreadable; delete %s to start over", reviewStatePath(results))
	}
	if st.Items == nil {
		st.Items = map[string]ReviewItem{}
	}
	return st, nil
}

func SaveReviewState(results string, st ReviewState) error {
	b, e := json.MarshalIndent(st, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(reviewStatePath(results), b, 0o600)
}

// ReviewDigest identifies the whole rendered page so Post can refuse a file that changed after render.
func (m ReviewModel) ReviewDigest() string {
	h := sha256.New()
	shared, _ := json.Marshal(struct {
		Title, Notice string
		Frames        []ReviewFrame
	}{m.Title, m.Notice, m.Frames})
	h.Write(shared)
	h.Write([]byte(m.ContentDigest))
	for _, c := range m.Cards {
		h.Write([]byte(c.Ref + c.Digest))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func digestOf(c ReviewCard) string {
	c.Digest = ""
	b, _ := json.Marshal(c)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

// BuildReview renders a results file without credentials or any Canvas call.
func BuildReview(root, path string) (ReviewModel, error) {
	kind, e := ReviewInputKind(path)
	if e != nil {
		return ReviewModel{}, e
	}
	var m ReviewModel
	switch kind {
	case "assignment-dates":
		m, e = buildAssignmentDatesReview(path)
	case "page":
		m, e = buildPageReview(path)
	case "assignment":
		m, e = buildAssignmentReview(path)
	case "quiz":
		m, e = buildQuizReview(path)
	case "results":
		m, e = buildResultsReview(root, path)
	case "group-results":
		m, e = buildGroupResultsReview(root, path)
	case "discussion":
		m, e = buildDiscussionReview(path)
	default:
		return ReviewModel{}, fmt.Errorf("review supports results, discussions, new assignments, Pages and QuizForge quiz files, not %q", kind)
	}
	if e != nil {
		return m, e
	}
	for i := range m.Cards {
		m.Cards[i].Digest = digestOf(m.Cards[i])
	}
	return m, nil
}

// ReviewInputKind uses the contents, including QuizForge envelopes, rather than
// filenames or the presence of an optional course argument.
func ReviewInputKind(path string) (string, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	// A QuizForge file has no document header; any file with one is a workspace document.
	kind, kindErr := workspace.DocumentKind(b)
	if kindErr != nil && !strings.Contains(string(b), "QUIZFORGE_JSON") && !strings.HasPrefix(strings.TrimSpace(string(b)), "{") {
		return "", kindErr
	}
	if kindErr != nil {
		kind = "quiz"
	}
	return kind, nil
}

// ValidateReviewInput performs local parsing and the optional wrong-course
// check before a review process or Canvas connection is opened.
func ValidateReviewInput(root, path, course string) (string, error) {
	kind, e := ReviewInputKind(path)
	if e != nil {
		return "", e
	}
	var nickname string
	var courseID int64
	switch kind {
	case "assignment-dates":
		var d domain.AssignmentDatesDocument
		_, e = workspace.ReadDocument(path, &d)
		if e == nil {
			e = domain.ValidateAssignmentDates(d)
		}
		nickname, courseID = d.Course, d.CourseID
	case "assignment":
		var d domain.AssignmentDocument
		_, e = workspace.ReadDocument(path, &d)
		nickname, courseID = d.Course, d.CourseID
		if e == nil && d.Operation != "create" && d.Operation != "update" {
			e = fmt.Errorf("review supports assignment create or update documents")
		}
	case "page":
		var d domain.PageDocument
		body, readErr := workspace.ReadDocument(path, &d)
		e = readErr
		if e == nil {
			e = d.ValidateBody(body)
		}
		nickname, courseID = d.Course, d.CourseID
	case "quiz":
		if course == "" {
			return "", fmt.Errorf("QuizForge review requires an explicit course nickname")
		}
		_, e = ReadQuizForge(path)
	case "results":
		var d domain.ResultsDocument
		_, e = workspace.ReadDocument(path, &d)
		courseID = d.Assignment.CourseID
	case "group-results":
		var d domain.GroupResultsDocument
		_, e = workspace.ReadDocument(path, &d)
		courseID = d.Assignment.CourseID
	case "discussion":
		var d domain.DiscussionDocument
		body, readErr := workspace.ReadDocument(path, &d)
		e = readErr
		if e == nil {
			e = d.ValidateBody(body)
		}
		nickname, courseID = d.Course, d.CourseID
	default:
		return "", fmt.Errorf("review supports assignments, Pages, discussions, results and QuizForge quiz files, not %q", kind)
	}
	if e != nil {
		return "", e
	}
	if course != "" {
		if nickname != "" && !strings.EqualFold(course, nickname) {
			return "", fmt.Errorf("review course %q differs from document course %q", course, nickname)
		}
		if courseID > 0 {
			var config domain.CourseConfig
			if _, e = workspace.ReadTeacherDocument(root, "context/courses.md", &config); e != nil {
				return "", e
			}
			configured, err := (&Service{Config: config}).Course(course)
			if err != nil {
				return "", err
			}
			if configured.ID != courseID {
				return "", fmt.Errorf("review course %q (%d) differs from document course_id %d", course, configured.ID, courseID)
			}
		}
	}
	return kind, nil
}

func buildDiscussionReview(path string) (ReviewModel, error) {
	var d domain.DiscussionDocument
	body, err := workspace.ReadDocument(path, &d)
	if err != nil {
		return ReviewModel{}, err
	}
	if err = d.ValidateBody(body); err != nil {
		return ReviewModel{}, err
	}
	html, err := render.TeachingHTML(body, d.BodyFormat)
	if err != nil {
		return ReviewModel{}, err
	}
	scope := "Course discussion"
	if d.GroupCategoryID > 0 {
		scope = fmt.Sprintf("Distributed discussion for group set %d", d.GroupCategoryID)
	}
	if d.GroupID > 0 {
		scope = fmt.Sprintf("Direct group topic for group %d", d.GroupID)
	}
	operation := d.Operation
	if d.IsAnnouncement != nil && *d.IsAnnouncement {
		operation = "announcement · " + operation
	}
	m := ReviewModel{Kind: "Ungraded discussion", Title: d.Title, AllOrNothing: true, Action: "Continue to Canvas"}
	if d.Assignment != nil {
		m.Kind = "Graded discussion"
	}
	published := discussionBool(d.Published, false)
	discussionType := d.DiscussionType
	requireInitial := discussionBool(d.RequireInitialPost, false)
	if d.Observed != nil {
		if d.Published == nil {
			published = d.Observed.Published
		}
		if discussionType == "" {
			discussionType = d.Observed.DiscussionType
		}
		if d.RequireInitialPost == nil {
			requireInitial = d.Observed.RequireInitialPost
		}
	}
	facts := []ReviewFact{fact("Operation", operation), fact("Audience", scope), fact("Published", fmt.Sprint(published))}
	if discussionType != "" {
		facts = append(facts, fact("Discussion type", discussionType))
	}
	facts = append(facts, fact("Require initial post", fmt.Sprint(requireInitial)))
	if d.DelayedPostAt != nil {
		facts = append(facts, fact("Delayed post", *d.DelayedPostAt))
	}
	if d.LockAt != nil {
		facts = append(facts, fact("Lock at", *d.LockAt))
	}
	if d.IsAnnouncement != nil && *d.IsAnnouncement {
		facts = append(facts, fact("Explicit communication intent", d.AnnouncementIntent))
	}
	if a := d.Assignment; a != nil {
		facts = append(facts, fact("Grading", "individual grade per student in the gradebook; each student is graded on their own posts"), fact("Points", a.Points.String()))
		if a.DueAt != nil {
			facts = append(facts, fact("Due", *a.DueAt))
		}
		if a.Rubric != nil {
			facts = append(facts, fact("Rubric", fmt.Sprintf("%s (%d criteria, not used for grading)", a.Rubric.Title, len(a.Rubric.Criteria))))
		}
		if d.GroupCategoryID > 0 {
			facts = append(facts, fact("Group set", "the group set is on the topic only; the assignment stays individual"))
		}
	}
	m.Cards = []ReviewCard{{Ref: "discussion-" + fmt.Sprint(d.GroupID) + "-" + fmt.Sprint(d.GroupCategoryID), Alias: d.Title, Operation: "create or update discussion", Facts: facts, Frames: []ReviewFrame{{Label: "Topic body", HTML: html}}}}
	return m, nil
}

func buildResultsReview(root, path string) (ReviewModel, error) {
	var doc domain.ResultsDocument
	if _, e := workspace.ReadDocument(path, &doc); e != nil {
		return ReviewModel{}, e
	}
	validated, e := domain.ValidateResults(&doc)
	if e != nil {
		return ReviewModel{}, e
	}
	// Aliases and first names come from the packet and the private student store; both are best effort.
	aliases := map[string]string{}
	works := map[string]domain.StudentWorkDocument{}
	if packetPath, e := workspace.ConstrainPath(root, doc.Packet); e == nil {
		var packet domain.PacketDocument
		if _, e = workspace.ReadDocument(packetPath, &packet); e == nil {
			for _, rel := range packet.Students {
				if p, e := workspace.ConstrainPath(root, rel); e == nil {
					var work domain.StudentWorkDocument
					if _, e = workspace.ReadDocument(p, &work); e == nil {
						aliases[work.StudentRef] = work.Alias
						works[work.StudentRef] = work
					}
				}
			}
		}
	}
	store, _ := workspace.OpenStudents(root)
	kind := "Assignment scores"
	if doc.Assignment.NewQuiz {
		kind = "New Quiz comments"
	}
	m := ReviewModel{Kind: kind, Title: doc.Assignment.Title}
	for i, r := range doc.Students {
		v := validated[i]
		c := ReviewCard{Ref: r.StudentRef, Alias: aliases[r.StudentRef], Operation: r.Operation, Hold: v.Hold, Done: resultComplete(r)}
		var feedback string
		switch {
		case r.Grade != nil:
			g := r.Grade
			c.Attempt = g.Attempt.Attempt
			c.TeacherNote = g.TeacherNote
			if g.LateTiming != nil {
				c.Late = lateSummary(*g.LateTiming)
				c.LateBasis = g.LateTiming.Basis
			}
			feedback = g.Feedback
			if v.Assessment != nil {
				c.Total = v.Assessment.Posted.String() + " / " + doc.Assignment.Points.String()
			}
		case r.ReviseFeedback != nil:
			c.Attempt = r.ReviseFeedback.Attempt.Attempt
			feedback = r.ReviseFeedback.Feedback
		case r.CommentOnly != nil:
			c.Attempt = r.CommentOnly.Attempt.Attempt
			feedback = r.CommentOnly.Feedback
		case r.LateOnly != nil:
			c.Attempt = r.LateOnly.Attempt.Attempt
			c.Late = lateSummary(r.LateOnly.LateTiming)
			c.LateBasis = r.LateOnly.LateTiming.Basis
			c.Facts = append(c.Facts, timingReviewFacts(r.LateOnly.LateTiming)...)
		}
		if r.Grade != nil && r.Grade.LateTiming != nil {
			c.Facts = append(c.Facts, timingReviewFacts(*r.Grade.LateTiming)...)
		}
		c.Submission = submissionText(works[r.StudentRef], c.Attempt)
		if w, ok := works[r.StudentRef]; ok && w.DiscussionEvidence != nil {
			var ev *domain.EvidenceSelection
			if r.Grade != nil {
				ev = r.Grade.Evidence
			} else if r.CommentOnly != nil {
				ev = r.CommentOnly.Evidence
			}
			if text := discussionEvidenceText(root, w, ev); text != "" {
				c.Submission = text
			}
		}
		if store != nil {
			if st, e := store.Resolve(doc.Assignment.Instance, r.StudentRef); e == nil {
				feedback = workspace.ReplaceStudentName(feedback, st)
			}
		}
		if c.Hold == "" && r.Grade != nil && doc.Basis() != nil {
			text, e := render.RenderFeedback(render.FeedbackInput{Rubric: *doc.Basis(), Interpretation: v.Interpretation, Components: r.Grade.Components, Assessment: r.Grade.Assessment, AssignmentPoints: doc.Assignment.Points, Feedback: feedback, Late: isLate(r.Grade.LateTiming), Header: r.Grade.Header})
			if e != nil {
				c.Hold = e.Error()
			} else {
				c.Comment = text
			}
		} else if r.ReviseFeedback != nil {
			c.Comment = feedback
		} else if r.CommentOnly != nil && doc.Basis() != nil {
			computed, e := domain.ComputeAssessment(*doc.Basis(), v.Interpretation, r.CommentOnly.Components, doc.Assignment.Points)
			if e != nil {
				c.Hold = e.Error()
			} else {
				text, e := render.RenderFeedback(render.FeedbackInput{Rubric: *doc.Basis(), Interpretation: v.Interpretation, Components: r.CommentOnly.Components, Assessment: computed.Posted, AssignmentPoints: doc.Assignment.Points, Feedback: feedback, OmitAssessment: doc.Assignment.NewQuiz, Header: r.CommentOnly.Header})
				if e != nil {
					c.Hold = e.Error()
				} else {
					c.Comment = text
				}
			}
		}
		m.Cards = append(m.Cards, c)
	}
	return m, nil
}

// submissionText returns what the student wrote for one attempt, from the
// pulled work, so the teacher sees the evidence beside the feedback.
func submissionText(w domain.StudentWorkDocument, attempt int) string {
	pick := func(a domain.AttemptWork) string {
		text := strings.TrimSpace(a.ReadableText)
		for _, response := range a.WrittenResponses {
			label := response.Prompt
			if label == "" {
				label = "item " + response.ItemID
			}
			answer := response.Text
			if answer == "" {
				answer = render.HTMLText(response.HTML)
			}
			if text != "" {
				text += "\n"
			}
			text += "[" + label + "]\n" + answer
		}
		if text == "" {
			text = "(no typed text in this attempt)"
		}
		for _, f := range a.Attachments {
			text += "\n[attachment: " + f.Name + "]"
		}
		for _, f := range a.WrittenFiles {
			text += "\n[New Quiz file: " + f.Name + ", " + f.Status
			if f.Reason != "" {
				text += ": " + f.Reason
			}
			text += "]"
		}
		return text
	}
	if w.Current.Ref.Attempt == attempt {
		return pick(w.Current)
	}
	for _, a := range w.Prior {
		if a.Ref.Attempt == attempt {
			return pick(a)
		}
	}
	return ""
}

func compactText(s string) string { return strings.Join(strings.Fields(s), " ") }

func lateSummary(t domain.LateTiming) string {
	if t.Status != "late" {
		return "not late"
	}
	h := t.SecondsLate / 3600
	return fmt.Sprintf("late %dd %dh", h/24, h%24)
}

func timingReviewFacts(t domain.LateTiming) []ReviewFact {
	facts := []ReviewFact{{Label: "Timing action", Value: "late policy status: " + t.Status}}
	if t.Status == "late" {
		facts = append(facts, ReviewFact{Label: "Seconds late", Value: fmt.Sprint(t.SecondsLate)})
	}
	if t.Observed != nil {
		observed := "automatic or no explicit status"
		if t.Observed.LatePolicyStatus != nil {
			observed = *t.Observed.LatePolicyStatus
		}
		facts = append(facts, ReviewFact{Label: "Current Canvas timing", Value: fmt.Sprintf("status %s; late=%t; seconds=%d", observed, t.Observed.Late, t.Observed.SecondsLate)})
	}
	if t.SchoolTime != nil {
		facts = append(facts,
			ReviewFact{Label: "First genuine submission counted", Value: fmt.Sprintf("attempt %d at %s", t.SchoolTime.Attempt.Attempt, t.SchoolTime.SubmittedAt)},
			ReviewFact{Label: "School-time deadline", Value: t.SchoolTime.Deadline},
		)
		if len(t.SchoolTime.SkippedDates) > 0 {
			facts = append(facts, ReviewFact{Label: "Calendar days skipped", Value: strings.Join(t.SchoolTime.SkippedDates, ", ")})
		}
	}
	return facts
}
