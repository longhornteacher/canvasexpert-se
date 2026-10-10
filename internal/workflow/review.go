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
	Cards       []ReviewCard
}

// ReviewCard is one student. Digest changes whenever anything shown changes.
type ReviewCard struct {
	Ref, Alias, Operation string
	Attempt               int
	Total                 string // computed assessment, e.g. "18.5 / 20"
	Comment               string // exact plain text that will be posted (ordinary results)
	Lines                 []ReviewLine
	TeacherNote           string
	Late                  string
	Hold                  string // reason this student will not post
	Done                  bool   // already posted
	Digest                string
}

// ReviewLine is one Classic question.
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
	b, e := os.ReadFile(path)
	if e != nil {
		return ReviewModel{}, e
	}
	kind, e := workspace.DocumentKind(b)
	if e != nil {
		return ReviewModel{}, e
	}
	var m ReviewModel
	switch kind {
	case "classic-quiz-results":
		m, e = buildClassicReview(root, path)
	case "results":
		m, e = buildResultsReview(root, path)
	default:
		return ReviewModel{}, fmt.Errorf("review supports results and classic-quiz-results files, not %q", kind)
	}
	if e != nil {
		return m, e
	}
	for i := range m.Cards {
		m.Cards[i].Digest = digestOf(m.Cards[i])
	}
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
	if packetPath, e := workspace.ConstrainPath(root, doc.Packet); e == nil {
		var packet domain.PacketDocument
		if _, e = workspace.ReadDocument(packetPath, &packet); e == nil {
			for _, rel := range packet.Students {
				if p, e := workspace.ConstrainPath(root, rel); e == nil {
					var work domain.StudentWorkDocument
					if _, e = workspace.ReadDocument(p, &work); e == nil {
						aliases[work.StudentRef] = work.Alias
					}
				}
			}
		}
	}
	store, _ := workspace.OpenStudents(root)
	m := ReviewModel{Kind: "Assignment scores", Title: doc.Assignment.Title}
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
			}
			feedback = g.Feedback
			if v.Assessment != nil {
				c.Total = v.Assessment.Posted.String() + " / " + doc.Assignment.Points.String()
			}
		case r.ReviseFeedback != nil:
			c.Attempt = r.ReviseFeedback.Attempt.Attempt
			feedback = r.ReviseFeedback.Feedback
		}
		if store != nil {
			if st, e := store.Resolve(doc.Assignment.Instance, r.StudentRef); e == nil {
				feedback = workspace.ReplaceStudentName(feedback, st)
			}
		}
		if c.Hold == "" && r.Grade != nil && doc.Basis() != nil {
			text, e := render.RenderFeedback(render.FeedbackInput{Rubric: *doc.Basis(), Interpretation: v.Interpretation, Components: r.Grade.Components, Assessment: r.Grade.Assessment, AssignmentPoints: doc.Assignment.Points, Feedback: feedback})
			if e != nil {
				c.Hold = e.Error()
			} else {
				c.Comment = text
			}
		} else if r.ReviseFeedback != nil {
			c.Comment = feedback
		}
		m.Cards = append(m.Cards, c)
	}
	return m, nil
}

func buildClassicReview(root, path string) (ReviewModel, error) {
	var doc domain.ClassicQuizResults
	if _, e := workspace.ReadDocument(path, &doc); e != nil {
		return ReviewModel{}, e
	}
	sources, e := loadClassicSources(root, &doc)
	if e != nil {
		return ReviewModel{}, e
	}
	m := ReviewModel{Kind: "Classic quiz writing", Title: doc.Assignment.Title}
	for _, row := range doc.Students {
		work := sources.Work[row.StudentRef]
		c := ReviewCard{Ref: row.StudentRef, Alias: work.Alias, Operation: "classic-question-comments", Attempt: row.Attempt.Attempt, TeacherNote: row.TeacherNote, Done: classicStudentComplete(row)}
		if row.LateTiming != nil {
			c.Late = lateSummary(*row.LateTiming)
		}
		held := 0
		for _, q := range row.Questions {
			src, _ := classicWorkQuestion(work.Current, q.QuestionID)
			l := ReviewLine{Name: src.Name, TeacherNote: q.TeacherNote, Done: classicQuestionDefinite(q.Outcome)}
			if l.Name == "" {
				l.Name = fmt.Sprintf("Question %d", q.QuestionID)
			}
			switch {
			case q.Evidence == "unknown":
				l.Hold = "evidence unknown: " + compactText(q.TeacherNote)
			case !src.ResponsePresent:
				l.Hold = "pulled response row unavailable"
			case q.Score == nil:
				l.Hold = "no score"
			default:
				l.Score = q.Score.String() + " / " + src.PointsPossible.String()
				l.Comment = domain.ClassicQuestionComment(*q.Score, src.PointsPossible, q.Comment)
			}
			if l.Hold != "" {
				held++
			}
			c.Lines = append(c.Lines, l)
		}
		if held > 0 && held == len(row.Questions) {
			c.Hold = "every selected question is held"
		}
		m.Cards = append(m.Cards, c)
	}
	return m, nil
}

func compactText(s string) string { return strings.Join(strings.Fields(s), " ") }

func lateSummary(t domain.LateTiming) string {
	if t.Status != "late" {
		return "not late"
	}
	h := t.SecondsLate / 3600
	return fmt.Sprintf("late %dd %dh", h/24, h%24)
}
