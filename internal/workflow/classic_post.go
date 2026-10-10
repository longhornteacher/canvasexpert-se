package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type ClassicStudentPostReport struct {
	StudentRef             string
	Attempt                domain.AttemptRef
	Status, Message        string
	Questions              []ClassicQuestionPostReport
	Late                   domain.ActionOutcome
	CanvasScore, KeptScore *domain.Decimal
	CanvasState            string
}
type ClassicQuestionPostReport struct {
	QuestionID     int64
	Score          *domain.Decimal
	PointsPossible domain.Decimal
	Outcome        domain.ActionOutcome
}
type classicSources struct {
	Packet domain.ClassicQuizPacket
	Work   map[string]domain.ClassicQuizWork
}

func loadClassicSources(root string, doc *domain.ClassicQuizResults) (classicSources, error) {
	sources := classicSources{Work: map[string]domain.ClassicQuizWork{}}
	if e := doc.Validate(); e != nil {
		return sources, e
	}
	path, e := workspace.ConstrainPath(root, doc.Packet)
	if e != nil {
		return sources, e
	}
	if _, e = workspace.ReadDocument(path, &sources.Packet); e != nil {
		return sources, e
	}
	packet := sources.Packet
	if packet.QuizID != doc.QuizID || !SameGradingBasis(packet.Assignment, doc.Assignment) {
		return sources, fmt.Errorf("Classic result identity/grading basis differs from packet")
	}
	for _, rel := range packet.Students {
		path, e := workspace.ConstrainPath(root, rel)
		if e != nil {
			return sources, e
		}
		var work domain.ClassicQuizWork
		if _, e = workspace.ReadDocument(path, &work); e != nil {
			return sources, e
		}
		if work.AssignmentID != doc.Assignment.AssignmentID || work.QuizID != doc.QuizID {
			return sources, fmt.Errorf("Classic work identity differs from packet")
		}
		if _, exists := sources.Work[work.StudentRef]; exists {
			return sources, fmt.Errorf("duplicate Classic packet student")
		}
		sources.Work[work.StudentRef] = work
	}
	for i := range doc.Students {
		row := &doc.Students[i]
		work, ok := sources.Work[row.StudentRef]
		if !ok {
			return sources, fmt.Errorf("student %s absent from Classic packet", row.StudentRef)
		}
		if !sameAttempt(row.Attempt, work.Current.Ref) {
			return sources, fmt.Errorf("Classic result attempt differs from pulled current attempt")
		}
		if row.StrongestPrior != nil {
			found := false
			for _, p := range work.Prior {
				if sameAttempt(p.Ref, row.StrongestPrior.Attempt) {
					found = true
				}
			}
			if !found {
				return sources, fmt.Errorf("strongest prior absent from supplied Classic history")
			}
		}
		for j := range row.Questions {
			q := &row.Questions[j]
			source, ok := classicWorkQuestion(work.Current, q.QuestionID)
			if !ok || !domain.ClassicManual(source.QuestionType) {
				return sources, fmt.Errorf("question %d is not a supplied current manual question", q.QuestionID)
			}
			if e = domain.NormalizeClassicScore(q.Score, source.PointsPossible); e != nil {
				return sources, e
			}
		}
	}
	return sources, nil
}

// ValidateClassicResults performs complete offline source validation; no private store is needed.
func ValidateClassicResults(root string, doc *domain.ClassicQuizResults) error {
	_, e := loadClassicSources(root, doc)
	return e
}
func classicWorkQuestion(work domain.ClassicAttemptWork, id int64) (domain.ClassicQuestionWork, bool) {
	for _, q := range work.Questions {
		if q.QuestionID == id {
			return q, true
		}
	}
	return domain.ClassicQuestionWork{}, false
}
func classicStudentComplete(row domain.ClassicStudentResult) bool {
	for _, q := range row.Questions {
		if !classicQuestionDefinite(q.Outcome) {
			return false
		}
	}
	return (row.LateTiming == nil && (row.LateOutcome.Status == "" || row.LateOutcome.Status == "not-requested")) || (row.LateTiming != nil && classicQuestionDefinite(row.LateOutcome))
}
func classicHold(row *domain.ClassicStudentResult, message string) {
	for i := range row.Questions {
		if !classicQuestionDefinite(row.Questions[i].Outcome) {
			holdOutcome(&row.Questions[i].Outcome, message)
		}
	}
	if row.LateTiming != nil && !definite(row.LateOutcome) {
		holdOutcome(&row.LateOutcome, message)
	}
}
func classicReport(doc domain.ClassicQuizResults, sources classicSources, observed map[string]ClassicSubmission) []ClassicStudentPostReport {
	var reports []ClassicStudentPostReport
	for _, row := range doc.Students {
		p := ClassicStudentPostReport{StudentRef: row.StudentRef, Attempt: row.Attempt, Status: "partial", Late: row.LateOutcome}
		if row.LateTiming == nil {
			p.Late = domain.ActionOutcome{Status: "not-requested"}
		}
		if classicStudentComplete(row) {
			p.Status = "completed"
		}
		for _, q := range row.Questions {
			source, _ := classicWorkQuestion(sources.Work[row.StudentRef].Current, q.QuestionID)
			p.Questions = append(p.Questions, ClassicQuestionPostReport{QuestionID: q.QuestionID, Score: q.Score, PointsPossible: source.PointsPossible, Outcome: q.Outcome})
			if !classicQuestionDefinite(q.Outcome) && q.Outcome.Message != "" {
				p.Message = q.Outcome.Message
			}
		}
		if observed, ok := observed[row.StudentRef]; ok {
			p.CanvasScore = decimalPointer(observed.Score)
			p.KeptScore = decimalPointer(observed.KeptScore)
			p.CanvasState = observed.State
		}
		reports = append(reports, p)
	}
	return reports
}
func (s *Service) postClassicResults(ctx context.Context, path string) (report PostReport, err error) {
	start := time.Now()
	requests := s.Client.Requests()
	report.Path = path
	report.Classic = true
	var doc domain.ClassicQuizResults
	body, err := workspace.ReadDocument(path, &doc)
	if err != nil {
		return report, err
	}
	sources, err := loadClassicSources(s.Root, &doc)
	if err != nil {
		return report, err
	}
	observed := map[string]ClassicSubmission{}
	defer func() {
		report.Elapsed = time.Since(start)
		report.Requests = s.Client.Requests() - requests
		report.QuizStudents = classicReport(doc, sources, observed)
	}()
	if doc.Assignment.Instance != s.Client.Origin() {
		return report, fmt.Errorf("Classic results target a different Canvas instance")
	}
	configured := false
	for _, c := range s.Config.Courses {
		if c.ID == doc.Assignment.CourseID && c.Nickname == sources.Packet.Course {
			configured = true
		}
	}
	if !configured {
		return report, fmt.Errorf("Classic results course is not configured")
	}
	store, e := workspace.OpenStudents(s.Root)
	if e != nil {
		return report, e
	}
	identities := map[string]workspace.Student{}
	complete := true
	for _, row := range doc.Students {
		if s.Skip[row.StudentRef] {
			continue
		}
		st, e := store.Resolve(s.Client.Origin(), row.StudentRef)
		if e != nil {
			return report, e
		}
		identities[row.StudentRef] = st
		complete = complete && classicStudentComplete(row)
	}
	if complete {
		return report, nil
	}
	assignment, e := GetAssignment(ctx, s.Client, doc.Assignment.CourseID, doc.Assignment.AssignmentID)
	if e != nil {
		return report, e
	}
	snapshot, e := Snapshot(assignment, s.Client.Origin())
	if e != nil {
		return report, e
	}
	quiz, e := getClassicQuiz(ctx, s.Client, doc.Assignment.CourseID, doc.QuizID)
	if e != nil {
		return report, e
	}
	basisMatches := SameGradingBasis(snapshot, doc.Assignment) && assignment.QuizID != nil && *assignment.QuizID == doc.QuizID && int64(quiz.ID) == doc.QuizID && int64(quiz.AssignmentID) == doc.Assignment.AssignmentID && quiz.QuizType == "assignment"
	save := func() error { return workspace.WriteDocument(path, &doc, body) }
	for i := range doc.Students {
		row := &doc.Students[i]
		if row.LateTiming == nil {
			row.LateOutcome = domain.ActionOutcome{Status: "not-requested"}
		}
		if classicStudentComplete(*row) || s.Skip[row.StudentRef] {
			continue
		}
		if !basisMatches {
			classicHold(row, "assignment/quiz grading basis changed; pull-work for reassessment")
			if err = save(); err != nil {
				return report, err
			}
			continue
		}
		work := sources.Work[row.StudentRef]
		user := identities[row.StudentRef].UserID
		sub, e := GetSubmission(ctx, s.Client, doc.Assignment.CourseID, doc.Assignment.AssignmentID, user)
		var qs ClassicSubmission
		if e == nil {
			qs, e = getClassicSubmission(ctx, s.Client, doc.Assignment.CourseID, doc.QuizID, work.QuizSubmissionID)
		}
		why := ""
		if e != nil {
			why = e.Error()
		} else {
			observed[row.StudentRef] = qs
			why = classicJoin(qs, sub, doc.QuizID, doc.Assignment.AssignmentID, user)
			if why == "" && (sub.ID != work.SubmissionID || !sameAttempt(row.Attempt, attemptRef(sub))) {
				why = "new or different current submission; pull-work for reassessment"
			}
		}
		if why != "" {
			classicHold(row, why)
			if err = save(); err != nil {
				return report, err
			}
			continue
		}
		defs, e := classicQuestions(ctx, s.Client, doc.Assignment.CourseID, doc.QuizID, work.QuizSubmissionID, row.Attempt.Attempt)
		if e != nil {
			classicHold(row, e.Error())
			if err = save(); err != nil {
				return report, err
			}
			continue
		}
		response, responseWhy := classicResponse(sub, row.Attempt)
		answers, duplicates := classicAnswers(response.SubmissionData)
		targets := map[string]any{}
		var included []int
		for j := range row.Questions {
			q := &row.Questions[j]
			if classicQuestionDefinite(q.Outcome) {
				continue
			}
			if q.Evidence == "unknown" {
				holdOutcome(&q.Outcome, q.TeacherNote)
				continue
			}
			source, _ := classicWorkQuestion(work.Current, q.QuestionID)
			var live *ClassicQuestion
			multiple := false
			for k := range defs {
				if int64(defs[k].ID) == q.QuestionID {
					if live != nil {
						multiple = true
					}
					live = &defs[k]
				}
			}
			if live == nil || multiple || live.Type != source.QuestionType || !live.Points.IsSet() || live.Points.Cmp(source.PointsPossible) != 0 {
				holdOutcome(&q.Outcome, "selected question type/maximum changed or unavailable; reassess")
				continue
			}
			a, ok := answers[q.QuestionID]
			if responseWhy != "" || !ok || duplicates[q.QuestionID] {
				why := responseWhy
				if why == "" {
					why = "current question response row unavailable or ambiguous"
				}
				holdOutcome(&q.Outcome, why)
				continue
			}
			text := domain.ClassicQuestionComment(*q.Score, live.Points, q.Comment)
			matches := classicAnswerMatches(a, *q.Score, text)
			if matches {
				status := "preserved"
				if ambiguous(q.Outcome) {
					status = "succeeded"
				}
				q.Outcome = domain.ActionOutcome{Status: status, ID: q.QuestionID, PostedText: text}
				continue
			}
			if ambiguous(q.Outcome) {
				q.Outcome.Status = "uncertain"
				q.Outcome.Message = "question completion remains uncertain; inspect Canvas and explicitly reassess with fresh pull/results if needed"
				continue
			}
			q.Outcome = domain.ActionOutcome{Status: "pending", ID: q.QuestionID, PostedText: text}
			included = append(included, j)
			targets[fmt.Sprint(q.QuestionID)] = map[string]any{"score": *q.Score, "comment": text}
		}
		// Persist pending outcomes before any question PUT.
		if err = save(); err != nil {
			return report, err
		}
		if len(included) > 0 {
			var written struct {
				Rows []ClassicSubmission `json:"quiz_submissions"`
			}
			e = s.Client.Write(ctx, "PUT", fmt.Sprintf("%s/submissions/%d", classicPath(doc.Assignment.CourseID, doc.QuizID), work.QuizSubmissionID), map[string]any{"quiz_submissions": []any{map[string]any{"attempt": row.Attempt.Attempt, "questions": targets}}}, &written)
			outcome := domain.ActionOutcome{Status: "uncertain", Message: "read-back did not confirm question assessment on intended attempt"}
			if e != nil {
				outcome = failedOutcome(e)
			}
			if e == nil || outcome.Status == "uncertain" {
				fresh, readErr := GetSubmission(ctx, s.Client, doc.Assignment.CourseID, doc.Assignment.AssignmentID, user)
				freshQS, quizErr := getClassicSubmission(ctx, s.Client, doc.Assignment.CourseID, doc.QuizID, work.QuizSubmissionID)
				if quizErr == nil {
					observed[row.StudentRef] = freshQS
				}
				// A new current attempt is not confirmation of this exact intended write.
				confirmed := readErr == nil && quizErr == nil && classicJoin(freshQS, fresh, doc.QuizID, doc.Assignment.AssignmentID, user) == "" && fresh.ID == work.SubmissionID && sameAttempt(row.Attempt, attemptRef(fresh))
				freshResponse, reason := classicResponse(fresh, row.Attempt)
				readAnswers, duplicate := classicAnswers(freshResponse.SubmissionData)
				for _, j := range included {
					q := &row.Questions[j]
					a, ok := readAnswers[q.QuestionID]
					if confirmed && reason == "" && ok && !duplicate[q.QuestionID] && classicAnswerMatches(a, *q.Score, q.Outcome.PostedText) {
						q.Outcome.Status = "succeeded"
						q.Outcome.Message = ""
					} else {
						q.Outcome.Status = "uncertain"
						q.Outcome.Message = "read-back did not confirm question assessment on intended attempt"
					}
				}
			} else {
				for _, j := range included {
					q := &row.Questions[j]
					q.Outcome.Status = outcome.Status
					q.Outcome.Message = outcome.Message
				}
			}
			if err = save(); err != nil {
				return report, err
			}
		}
		if row.LateTiming != nil && !definite(row.LateOutcome) {
			// Recheck the quiz's current finished state before the timing helper's fresh assignment read.
			current, e := getClassicSubmission(ctx, s.Client, doc.Assignment.CourseID, doc.QuizID, work.QuizSubmissionID)
			if e != nil {
				holdOutcome(&row.LateOutcome, e.Error())
				err = save()
			} else if current.Attempt != row.Attempt.Attempt || current.FinishedAt == nil || *current.FinishedAt == "" || (current.State != "pending_review" && current.State != "complete") {
				holdOutcome(&row.LateOutcome, "new or unfinished quiz attempt before timing adjustment")
				err = save()
			} else {
				err = s.postLateTiming(ctx, doc.Assignment, user, row.Attempt, row.LateTiming, &row.LateOutcome, save)
			}
			if err != nil {
				return report, err
			}
		}
	}
	return report, nil
}
func classicAnswerMatches(a ClassicAnswer, score domain.Decimal, text string) bool {
	observed := decimalPointer(a.Points)
	return observed != nil && observed.Cmp(score) == 0 && strings.ReplaceAll(a.MoreComments, "\r\n", "\n") == strings.ReplaceAll(text, "\r\n", "\n")
}

// ClassicPreview is shared by the local CLI preview after source validation.
func ClassicPreview(root, path string) ([]ClassicStudentPostReport, error) {
	var doc domain.ClassicQuizResults
	if _, e := workspace.ReadDocument(path, &doc); e != nil {
		return nil, e
	}
	sources, e := loadClassicSources(root, &doc)
	if e != nil {
		return nil, e
	}
	for i := range doc.Students {
		for j := range doc.Students[i].Questions {
			q := &doc.Students[i].Questions[j]
			source, _ := classicWorkQuestion(sources.Work[doc.Students[i].StudentRef].Current, q.QuestionID)
			if q.Evidence == "unknown" {
				holdOutcome(&q.Outcome, q.TeacherNote)
			} else if !source.ResponsePresent {
				holdOutcome(&q.Outcome, "pulled response row unavailable")
			}
		}
	}
	return classicReport(doc, sources, nil), nil
}
